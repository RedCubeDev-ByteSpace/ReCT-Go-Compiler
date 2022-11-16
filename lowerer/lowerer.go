package lowerer

import (
	"ReCT-Go-Compiler/builtins"
	"ReCT-Go-Compiler/lexer"
	"ReCT-Go-Compiler/nodes/boundnodes"
	"ReCT-Go-Compiler/print"
	"ReCT-Go-Compiler/symbols"
	"fmt"
	"os"
)

var labelCounter int = 0

func GenerateLabel() boundnodes.BoundLabel {
	labelCounter++
	return boundnodes.BoundLabel(fmt.Sprintf("Label%d", labelCounter))
}

func Lower(functionSymbol symbols.FunctionSymbol, stmt boundnodes.BoundStatementNode) boundnodes.BoundBlockStatementNode {
	result := RewriteStatement(stmt)
	return Flatten(functionSymbol, result)
}

func Flatten(functionSymbol symbols.FunctionSymbol, stmt boundnodes.BoundStatementNode) boundnodes.BoundBlockStatementNode {
	statements := make([]boundnodes.BoundStatementNode, 0)
	stack := make([]boundnodes.BoundStatementNode, 0)

	pushTo := func(stck *[]boundnodes.BoundStatementNode, stmt boundnodes.BoundStatementNode) {
		*stck = append(*stck, stmt)
	}

	transferTo := func(stck *[]boundnodes.BoundStatementNode, stmt []boundnodes.BoundStatementNode) {
		*stck = append(*stck, stmt...)
	}

	popFrom := func(stck *[]boundnodes.BoundStatementNode) boundnodes.BoundStatementNode {
		element := (*stck)[len(*stck)-1]
		*stck = (*stck)[:len(*stck)-1]
		return element
	}

	pushTo(&stack, stmt)

	root := true

	for len(stack) > 0 {
		current := popFrom(&stack)

		if current.NodeType() == boundnodes.BoundBlockStatement {
			// create a new local stack for this block
			// this is so we can insert nodes before these if we need to
			localStack := make([]boundnodes.BoundStatementNode, 0)

			// keep track of any variable declarations made in this block
			variables := make([]symbols.VariableSymbol, 0)

			// push all elements onto the stack in reverse order (bc yk stacks are like that)
			currentBlock := current.(boundnodes.BoundBlockStatementNode)
			for i := len(currentBlock.Statements) - 1; i >= 0; i-- {
				stmt := currentBlock.Statements[i]

				pushTo(&localStack, stmt)

				// if this is a variable declaration, keep track of its variable!
				if stmt.NodeType() == boundnodes.BoundVariableDeclaration {
					declStatement := stmt.(boundnodes.BoundVariableDeclarationStatementNode)

					if !declStatement.Variable.IsGlobal() {
						variables = append(variables, declStatement.Variable)
					}
				}
			}

			// if we have any variables in here and this isnt the function body itself, add a GC call
			if len(variables) != 0 && !root {
				pushTo(&stack, boundnodes.CreateBoundGarbageCollectionStatementNode(variables, print.TextSpan{}))
			}

			// transfer elements from out local stack over to the main one
			transferTo(&stack, localStack)
			root = false
		} else {
			statements = append(statements, current)
		}
	}

	if functionSymbol.Type.Fingerprint() == builtins.Void.Fingerprint() {
		if len(statements) == 0 || CanFallThrough(statements[len(statements)-1]) {
			statements = append(statements, boundnodes.CreateBoundReturnStatementNode(nil, print.TextSpan{}))
		}
	}

	return boundnodes.CreateBoundBlockStatementNode(statements, stmt.Span())
}

func CanFallThrough(stmt boundnodes.BoundStatementNode) bool {
	return stmt.NodeType() != boundnodes.BoundReturnStatement &&
		stmt.NodeType() != boundnodes.BoundGotoStatement
}

func RewriteStatement(stmt boundnodes.BoundStatementNode) boundnodes.BoundStatementNode {
	switch stmt.NodeType() {
	case boundnodes.BoundBlockStatement:
		return RewriteBlockStatement(stmt.(boundnodes.BoundBlockStatementNode))
	case boundnodes.BoundVariableDeclaration:
		return RewriteVariableDeclaration(stmt.(boundnodes.BoundVariableDeclarationStatementNode))
	case boundnodes.BoundIfStatement:
		return RewriteIfStatement(stmt.(boundnodes.BoundIfStatementNode))
	case boundnodes.BoundWhileStatement:
		return RewriteWhileStatement(stmt.(boundnodes.BoundWhileStatementNode))
	case boundnodes.BoundForStatement:
		return RewriteForStatement(stmt.(boundnodes.BoundForStatementNode))
	case boundnodes.BoundFromToStatement:
		return RewriteFromToStatement(stmt.(boundnodes.BoundFromToStatementNode))
	case boundnodes.BoundLabelStatement:
		return RewriteLabelStatement(stmt.(boundnodes.BoundLabelStatementNode))
	case boundnodes.BoundGotoStatement:
		return RewriteGotoStatement(stmt.(boundnodes.BoundGotoStatementNode))
	case boundnodes.BoundConditionalGotoStatement:
		return RewriteConditionalGotoStatement(stmt.(boundnodes.BoundConditionalGotoStatementNode))
	case boundnodes.BoundReturnStatement:
		return RewriteReturnStatement(stmt.(boundnodes.BoundReturnStatementNode))
	case boundnodes.BoundExpressionStatement:
		return RewriteExpressionStatement(stmt.(boundnodes.BoundExpressionStatementNode))
	default:
		return stmt
	}
}

func RewriteBlockStatement(stmt boundnodes.BoundBlockStatementNode) boundnodes.BoundBlockStatementNode {
	rewrittenStatements := make([]boundnodes.BoundStatementNode, 0)

	for _, statement := range stmt.Statements {
		rewrittenStatements = append(rewrittenStatements, RewriteStatement(statement))
	}

	return boundnodes.CreateBoundBlockStatementNode(rewrittenStatements, stmt.BoundSpan)
}

func RewriteVariableDeclaration(stmt boundnodes.BoundVariableDeclarationStatementNode) boundnodes.BoundVariableDeclarationStatementNode {
	if stmt.Initializer != nil {
		initializer := RewriteExpression(stmt.Initializer)
		return boundnodes.CreateBoundVariableDeclarationStatementNode(stmt.Variable, initializer, stmt.BoundSpan)
	}

	return stmt
}

func RewriteIfStatement(stmt boundnodes.BoundIfStatementNode) boundnodes.BoundStatementNode {
	if stmt.ElseStatement == nil {
		// if <condition> { <then> }
		//
		// <- gets lowered into: ->
		//
		// condGoto <condition> then, end
		// then:
		// 	<then>
		// goto end
		// end:
		thenLabel := GenerateLabel()
		endLabel := GenerateLabel()
		condGoto := boundnodes.CreateBoundConditionalGotoStatementNode(stmt.Condition, thenLabel, endLabel, stmt.BoundSpan)
		thenLabelStatement := boundnodes.CreateBoundLabelStatementNode(thenLabel, stmt.BoundSpan)
		endLabelStatement := boundnodes.CreateBoundLabelStatementNode(endLabel, stmt.BoundSpan)
		gotoEnd := boundnodes.CreateBoundGotoStatementNode(endLabel, stmt.BoundSpan)
		result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
			condGoto, thenLabelStatement, stmt.ThenStatement, gotoEnd, endLabelStatement,
		}, stmt.BoundSpan)
		return RewriteStatement(result)

	} else {
		// if <condition> { <then> }
		// else { <else> }
		//
		// <- gets lowered into: ->
		//
		// condGoto <condition> then, else
		// then:
		// 	<then>
		// goto end
		// else:
		// 	<else>
		// goto end
		// end:

		thenLabel := GenerateLabel()
		elseLabel := GenerateLabel()
		endLabel := GenerateLabel()

		condGoto := boundnodes.CreateBoundConditionalGotoStatementNode(stmt.Condition, thenLabel, elseLabel, stmt.BoundSpan)
		gotoEnd := boundnodes.CreateBoundGotoStatementNode(endLabel, stmt.BoundSpan)
		thenLabelStatement := boundnodes.CreateBoundLabelStatementNode(thenLabel, stmt.BoundSpan)
		elseLabelStatement := boundnodes.CreateBoundLabelStatementNode(elseLabel, stmt.BoundSpan)
		endLabelStatement := boundnodes.CreateBoundLabelStatementNode(endLabel, stmt.BoundSpan)
		result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
			condGoto, thenLabelStatement, stmt.ThenStatement, gotoEnd, elseLabelStatement, stmt.ElseStatement, gotoEnd, endLabelStatement,
		}, stmt.BoundSpan)
		return RewriteStatement(result)
	}
}

func RewriteWhileStatement(stmt boundnodes.BoundWhileStatementNode) boundnodes.BoundStatementNode {
	// while <condition> { <body> }
	//
	// <- gets lowered into: ->
	//
	// goto continue
	// body:
	// <body>
	// goto continue
	// continue:
	// condGoto <condition> body
	// break:
	bodyLabel := GenerateLabel()

	gotoContinue := boundnodes.CreateBoundGotoStatementNode(stmt.ContinueLabel, stmt.BoundSpan)
	bodyLabelStatement := boundnodes.CreateBoundLabelStatementNode(bodyLabel, stmt.BoundSpan)
	continueLabelStatement := boundnodes.CreateBoundLabelStatementNode(stmt.ContinueLabel, stmt.BoundSpan)
	condGoto := boundnodes.CreateBoundConditionalGotoStatementNode(stmt.Condition, bodyLabel, stmt.BreakLabel, stmt.BoundSpan)
	breakLabelStatement := boundnodes.CreateBoundLabelStatementNode(stmt.BreakLabel, stmt.BoundSpan)

	result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
		gotoContinue, bodyLabelStatement, stmt.Body, gotoContinue, continueLabelStatement, condGoto, breakLabelStatement,
	}, stmt.BoundSpan)
	return RewriteStatement(result)
}

func RewriteForStatement(stmt boundnodes.BoundForStatementNode) boundnodes.BoundStatementNode {
	condition := RewriteExpression(stmt.Condition)
	continueLabelStatement := boundnodes.CreateBoundLabelStatementNode(stmt.ContinueLabel, stmt.BoundSpan)

	gotoContinue := boundnodes.CreateBoundGotoStatementNode(stmt.ContinueLabel, stmt.BoundSpan)
	whileBody := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
		stmt.Body, gotoContinue, continueLabelStatement, stmt.Action,
	}, stmt.BoundSpan)
	whileStatement := boundnodes.CreateBoundWhileStatementNode(condition, whileBody, stmt.BreakLabel, GenerateLabel(), stmt.BoundSpan)

	variable := RewriteStatement(stmt.Variable).(boundnodes.BoundVariableDeclarationStatementNode)

	result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
		variable, whileStatement,
	}, stmt.BoundSpan)
	return RewriteStatement(result)
}

func RewriteFromToStatement(stmt boundnodes.BoundFromToStatementNode) boundnodes.BoundStatementNode {
	// good god what did i just write - RedCube
	lowerBound := RewriteExpression(stmt.LowerBound)
	upperBound := RewriteExpression(stmt.UpperBound)
	variableDeclaration := boundnodes.CreateBoundVariableDeclarationStatementNode(stmt.Variable, lowerBound, stmt.BoundSpan)
	variableExpression := boundnodes.CreateBoundVariableExpressionNode(stmt.Variable, stmt.BoundSpan)
	upperBoundSymbol := symbols.CreateLocalVariableSymbol("upperBound", true, builtins.Int)
	upperBoundDeclaration := boundnodes.CreateBoundVariableDeclarationStatementNode(upperBoundSymbol, upperBound, stmt.BoundSpan)

	condition := boundnodes.CreateBoundBinaryExpressionNode(
		variableExpression,
		boundnodes.BindBinaryOperator(lexer.LessEqualsToken, builtins.Int, builtins.Int),
		boundnodes.CreateBoundVariableExpressionNode(upperBoundSymbol, stmt.BoundSpan), stmt.BoundSpan,
	)
	continueLabelStatement := boundnodes.CreateBoundLabelStatementNode(stmt.ContinueLabel, stmt.BoundSpan)
	increment := boundnodes.CreateBoundExpressionStatementNode(
		boundnodes.CreateBoundAssignmentExpressionNode(
			stmt.Variable,
			boundnodes.CreateBoundBinaryExpressionNode(
				variableExpression,
				boundnodes.BindBinaryOperator(lexer.PlusToken, builtins.Int, builtins.Int),
				boundnodes.CreateBoundLiteralExpressionNodeFromValue(1, stmt.BoundSpan), stmt.BoundSpan,
			), stmt.BoundSpan,
		), stmt.BoundSpan,
	)

	gotoContinue := boundnodes.CreateBoundGotoStatementNode(stmt.ContinueLabel, stmt.BoundSpan)
	whileBody := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
		stmt.Body,
		gotoContinue,
		continueLabelStatement,
		increment,
	}, stmt.BoundSpan)

	whileStatement := boundnodes.CreateBoundWhileStatementNode(condition, whileBody, stmt.BreakLabel, GenerateLabel(), stmt.BoundSpan)

	result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
		variableDeclaration, upperBoundDeclaration, whileStatement,
	}, stmt.BoundSpan)
	return RewriteStatement(result)
}

func RewriteLabelStatement(stmt boundnodes.BoundLabelStatementNode) boundnodes.BoundLabelStatementNode {
	return stmt
}

func RewriteGotoStatement(stmt boundnodes.BoundGotoStatementNode) boundnodes.BoundGotoStatementNode {
	return stmt
}

func RewriteConditionalGotoStatement(stmt boundnodes.BoundConditionalGotoStatementNode) boundnodes.BoundConditionalGotoStatementNode {
	condition := RewriteExpression(stmt.Condition)
	return boundnodes.CreateBoundConditionalGotoStatementNode(condition, stmt.IfLabel, stmt.ElseLabel, stmt.BoundSpan)
}

func RewriteReturnStatement(stmt boundnodes.BoundReturnStatementNode) boundnodes.BoundReturnStatementNode {
	var expression boundnodes.BoundExpressionNode = nil
	if stmt.Expression != nil {
		expression = RewriteExpression(stmt.Expression)
	}

	return boundnodes.CreateBoundReturnStatementNode(expression, stmt.BoundSpan)
}

func RewriteExpressionStatement(stmt boundnodes.BoundExpressionStatementNode) boundnodes.BoundExpressionStatementNode {
	expression := RewriteExpression(stmt.Expression)
	return boundnodes.CreateBoundExpressionStatementNode(expression, stmt.BoundSpan)
}

func RewriteExpression(expr boundnodes.BoundExpressionNode) boundnodes.BoundExpressionNode {
	switch expr.NodeType() {
	case boundnodes.BoundErrorExpression:
		return RewriteErrorExpression(expr.(boundnodes.BoundErrorExpressionNode))
	case boundnodes.BoundLiteralExpression:
		return RewriteLiteralExpression(expr.(boundnodes.BoundLiteralExpressionNode))
	case boundnodes.BoundVariableExpression:
		return RewriteVariableExpression(expr.(boundnodes.BoundVariableExpressionNode))
	case boundnodes.BoundAssignmentExpression:
		return RewriteAssignmentExpression(expr.(boundnodes.BoundAssignmentExpressionNode))
	case boundnodes.BoundUnaryExpression:
		return RewriteUnaryExpression(expr.(boundnodes.BoundUnaryExpressionNode))
	case boundnodes.BoundBinaryExpression:
		return RewriteBinaryExpression(expr.(boundnodes.BoundBinaryExpressionNode))
	case boundnodes.BoundCallExpression:
		return RewriteCallExpression(expr.(boundnodes.BoundCallExpressionNode))
	case boundnodes.BoundPackageCallExpression:
		return RewritePackageCallExpression(expr.(boundnodes.BoundPackageCallExpressionNode))
	case boundnodes.BoundConversionExpression:
		return RewriteConversionExpression(expr.(boundnodes.BoundConversionExpressionNode))
	case boundnodes.BoundTypeCallExpression:
		return RewriteTypeCallExpression(expr.(boundnodes.BoundTypeCallExpressionNode))
	case boundnodes.BoundClassCallExpression:
		return RewriteClassCallExpression(expr.(boundnodes.BoundClassCallExpressionNode))
	case boundnodes.BoundClassFieldAccessExpression:
		return RewriteClassFieldAccessExpression(expr.(boundnodes.BoundClassFieldAccessExpressionNode))
	case boundnodes.BoundClassFieldAssignmentExpression:
		return RewriteClassFieldAssignmentExpression(expr.(boundnodes.BoundClassFieldAssignmentExpressionNode))
	case boundnodes.BoundClassDestructionExpression:
		return RewriteClassDestructionExpression(expr.(boundnodes.BoundClassDestructionExpressionNode))
	case boundnodes.BoundArrayAccessExpression:
		return RewriteArrayAccessExpression(expr.(boundnodes.BoundArrayAccessExpressionNode))
	case boundnodes.BoundArrayAssignmentExpression:
		return RewriteArrayAssignmentExpression(expr.(boundnodes.BoundArrayAssignmentExpressionNode))
	case boundnodes.BoundMakeExpression:
		return RewriteMakeExpression(expr.(boundnodes.BoundMakeExpressionNode))
	case boundnodes.BoundMakeArrayExpression:
		return RewriteMakeArrayExpression(expr.(boundnodes.BoundMakeArrayExpressionNode))
	case boundnodes.BoundMakeStructExpression:
		return RewriteMakeStructExpression(expr.(boundnodes.BoundMakeStructExpressionNode))
	case boundnodes.BoundFunctionExpression:
		return RewriteFunctionExpression(expr.(boundnodes.BoundFunctionExpressionNode))
	case boundnodes.BoundThreadExpression:
		return RewriteThreadExpression(expr.(boundnodes.BoundThreadExpressionNode))
	case boundnodes.BoundTernaryExpression:
		return RewriteTernaryExpression(expr.(boundnodes.BoundTernaryExpressionNode))
	case boundnodes.BoundReferenceExpression:
		return RewriteReferenceExpression(expr.(boundnodes.BoundReferenceExpressionNode))
	case boundnodes.BoundDereferenceExpression:
		return RewriteDereferenceExpression(expr.(boundnodes.BoundDereferenceExpressionNode))
	default:
		print.PrintC(print.Red, "Expression unaccounted for in lowerer! (stuff being in here is important lol)")
		os.Exit(-1)
		return nil
	}
}

func RewriteErrorExpression(expr boundnodes.BoundErrorExpressionNode) boundnodes.BoundErrorExpressionNode {
	return expr
}

func RewriteLiteralExpression(expr boundnodes.BoundLiteralExpressionNode) boundnodes.BoundLiteralExpressionNode {
	return expr
}

func RewriteVariableExpression(expr boundnodes.BoundVariableExpressionNode) boundnodes.BoundVariableExpressionNode {
	return expr
}

func RewriteAssignmentExpression(expr boundnodes.BoundAssignmentExpressionNode) boundnodes.BoundAssignmentExpressionNode {
	expression := RewriteExpression(expr.Expression)
	return boundnodes.CreateBoundAssignmentExpressionNode(expr.Variable, expression, expr.BoundSpan)
}

func RewriteUnaryExpression(expr boundnodes.BoundUnaryExpressionNode) boundnodes.BoundUnaryExpressionNode {
	operand := RewriteExpression(expr.Expression)
	return boundnodes.CreateBoundUnaryExpressionNode(expr.Op, operand, expr.BoundSpan)
}

func RewriteBinaryExpression(expr boundnodes.BoundBinaryExpressionNode) boundnodes.BoundBinaryExpressionNode {
	left := RewriteExpression(expr.Left)
	right := RewriteExpression(expr.Right)
	return boundnodes.CreateBoundBinaryExpressionNode(left, expr.Op, right, expr.BoundSpan)
}

func RewriteCallExpression(expr boundnodes.BoundCallExpressionNode) boundnodes.BoundCallExpressionNode {
	rewrittenArgs := make([]boundnodes.BoundExpressionNode, 0)

	for _, arg := range expr.Arguments {
		rewrittenArgs = append(rewrittenArgs, RewriteExpression(arg))
	}

	return boundnodes.CreateBoundCallExpressionNode(expr.Function, rewrittenArgs, expr.BoundSpan)
}

func RewritePackageCallExpression(expr boundnodes.BoundPackageCallExpressionNode) boundnodes.BoundPackageCallExpressionNode {
	rewrittenArgs := make([]boundnodes.BoundExpressionNode, 0)

	for _, arg := range expr.Arguments {
		rewrittenArgs = append(rewrittenArgs, RewriteExpression(arg))
	}

	return boundnodes.CreateBoundPackageCallExpressionNode(expr.Package, expr.Function, rewrittenArgs, expr.BoundSpan)
}

func RewriteConversionExpression(expr boundnodes.BoundConversionExpressionNode) boundnodes.BoundExpressionNode {
	expression := RewriteExpression(expr.Expression)

	// =================================================================================================================
	// integer type literal optimisations
	// =================================================================================================================
	if expression.NodeType() == boundnodes.BoundLiteralExpression &&
		expression.Type().Fingerprint() == builtins.Int.Fingerprint() {
		value := expression.(boundnodes.BoundLiteralExpressionNode).Value.(int)

		// int literal to byte literal
		if expr.ToType.Fingerprint() == builtins.Byte.Fingerprint() {
			return boundnodes.CreateBoundLiteralExpressionNodeFromValue(byte(value), expression.Span())
		}

		// int literal to long literal
		if expr.ToType.Fingerprint() == builtins.Long.Fingerprint() {
			return boundnodes.CreateBoundLiteralExpressionNodeFromValue(int64(value), expression.Span())
		}
	}

	return boundnodes.CreateBoundConversionExpressionNode(expr.ToType, expression, expr.BoundSpan)
}

func RewriteTypeCallExpression(expr boundnodes.BoundTypeCallExpressionNode) boundnodes.BoundTypeCallExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)

	rewrittenArgs := make([]boundnodes.BoundExpressionNode, 0)

	for _, arg := range expr.Arguments {
		rewrittenArgs = append(rewrittenArgs, RewriteExpression(arg))
	}

	return boundnodes.CreateBoundTypeCallExpressionNode(rewrittenBase, expr.Function, rewrittenArgs, expr.BoundSpan)
}

func RewriteClassCallExpression(expr boundnodes.BoundClassCallExpressionNode) boundnodes.BoundClassCallExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)

	rewrittenArgs := make([]boundnodes.BoundExpressionNode, 0)

	for _, arg := range expr.Arguments {
		rewrittenArgs = append(rewrittenArgs, RewriteExpression(arg))
	}

	return boundnodes.CreateBoundClassCallExpressionNode(rewrittenBase, expr.Function, rewrittenArgs, expr.BoundSpan)
}

func RewriteClassFieldAccessExpression(expr boundnodes.BoundClassFieldAccessExpressionNode) boundnodes.BoundClassFieldAccessExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)

	return boundnodes.CreateBoundClassFieldAccessExpressionNode(rewrittenBase, expr.Field, expr.BoundSpan)
}

func RewriteClassFieldAssignmentExpression(expr boundnodes.BoundClassFieldAssignmentExpressionNode) boundnodes.BoundClassFieldAssignmentExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)
	rewrittenValue := RewriteExpression(expr.Value)

	return boundnodes.CreateBoundClassFieldAssignmentExpressionNode(rewrittenBase, expr.Field, rewrittenValue, expr.BoundSpan)
}

func RewriteClassDestructionExpression(expr boundnodes.BoundClassDestructionExpressionNode) boundnodes.BoundClassDestructionExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)

	return boundnodes.CreateBoundClassDestructionExpressionNode(rewrittenBase, expr.BoundSpan)
}

func RewriteArrayAccessExpression(expr boundnodes.BoundArrayAccessExpressionNode) boundnodes.BoundArrayAccessExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)
	rewrittenIndex := RewriteExpression(expr.Index)

	return boundnodes.CreateBoundArrayAccessExpressionNode(rewrittenBase, rewrittenIndex, expr.IsPointer, expr.BoundSpan)
}

func RewriteArrayAssignmentExpression(expr boundnodes.BoundArrayAssignmentExpressionNode) boundnodes.BoundArrayAssignmentExpressionNode {
	rewrittenBase := RewriteExpression(expr.Base)
	rewrittenIndex := RewriteExpression(expr.Index)
	rewrittenValue := RewriteExpression(expr.Value)

	return boundnodes.CreateBoundArrayAssignmentExpressionNode(rewrittenBase, rewrittenIndex, rewrittenValue, expr.IsPointer, expr.BoundSpan)
}

func RewriteMakeExpression(expr boundnodes.BoundMakeExpressionNode) boundnodes.BoundMakeExpressionNode {
	rewrittenArgs := make([]boundnodes.BoundExpressionNode, 0)

	for _, arg := range expr.Arguments {
		rewrittenArgs = append(rewrittenArgs, RewriteExpression(arg))
	}

	return boundnodes.CreateBoundMakeExpressionNode(expr.BaseType, rewrittenArgs, expr.BoundSpan)
}

func RewriteMakeArrayExpression(expr boundnodes.BoundMakeArrayExpressionNode) boundnodes.BoundMakeArrayExpressionNode {
	if expr.IsLiteral {
		rewrittenLiterals := make([]boundnodes.BoundExpressionNode, 0)
		for _, literal := range expr.Literals {
			rewrittenLiterals = append(rewrittenLiterals, RewriteExpression(literal))
		}
		return boundnodes.CreateBoundMakeArrayExpressionNodeLiteral(expr.BaseType, rewrittenLiterals, expr.BoundSpan)
	}

	rewrittenLength := RewriteExpression(expr.Length)
	return boundnodes.CreateBoundMakeArrayExpressionNode(expr.BaseType, rewrittenLength)
}

func RewriteMakeStructExpression(expr boundnodes.BoundMakeStructExpressionNode) boundnodes.BoundMakeStructExpressionNode {
	rewrittenLiterals := make([]boundnodes.BoundExpressionNode, 0)
	for _, literal := range expr.Literals {
		rewrittenLiterals = append(rewrittenLiterals, RewriteExpression(literal))
	}
	return boundnodes.CreateBoundMakeStructExpressionNode(expr.StructType, rewrittenLiterals, expr.BoundSpan)
}

func RewriteFunctionExpression(expr boundnodes.BoundFunctionExpressionNode) boundnodes.BoundFunctionExpressionNode {
	return expr
}

func RewriteThreadExpression(expr boundnodes.BoundThreadExpressionNode) boundnodes.BoundThreadExpressionNode {
	return expr
}

func RewriteTernaryExpression(expr boundnodes.BoundTernaryExpressionNode) boundnodes.BoundTernaryExpressionNode {
	// dissolve the ternary expression into an if statement

	// a ? b : c
	//
	// <- gets lowered into: ->
	//
	// condGoto <condition> then, else
	// then:
	// 	%v = b
	//  <gc>
	// goto end
	// else:
	//  %v = c
	// 	<gc>
	// goto end
	// end:
	// a = %v

	//thenLabel := GenerateLabel()
	//elseLabel := GenerateLabel()
	//endLabel := GenerateLabel()
	//
	//condGoto := boundnodes.CreateBoundConditionalGotoStatementNode(stmt.Condition, thenLabel, elseLabel)
	//gotoEnd := boundnodes.CreateBoundGotoStatementNode(endLabel)
	//thenLabelStatement := boundnodes.CreateBoundLabelStatementNode(thenLabel)
	//elseLabelStatement := boundnodes.CreateBoundLabelStatementNode(elseLabel)
	//endLabelStatement := boundnodes.CreateBoundLabelStatementNode(endLabel)
	//result := boundnodes.CreateBoundBlockStatementNode([]boundnodes.BoundStatementNode{
	//	condGoto, thenLabelStatement, stmt.ThenStatement, gotoEnd, elseLabelStatement, stmt.ElseStatement, gotoEnd, endLabelStatement,
	//})

	// => was moved to emitter

	expr.IfLabel = GenerateLabel()
	expr.ElseLabel = GenerateLabel()
	expr.EndLabel = GenerateLabel()

	return expr
}

func RewriteReferenceExpression(expr boundnodes.BoundReferenceExpressionNode) boundnodes.BoundReferenceExpressionNode {
	val := RewriteExpression(expr.Expression)
	return boundnodes.CreateBoundReferenceExpressionNode(val, expr.Span())
}

func RewriteDereferenceExpression(expr boundnodes.BoundDereferenceExpressionNode) boundnodes.BoundDereferenceExpressionNode {
	val := RewriteExpression(expr.Expression)
	return boundnodes.CreateBoundDereferenceExpressionNode(val, expr.Span())
}
