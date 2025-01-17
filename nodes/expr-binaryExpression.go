package nodes

import (
	"fmt"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/lexer"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/print"
)

type BinaryExpressionNode struct {
	ExpressionNode

	Left     ExpressionNode
	Operator lexer.Token
	Right    ExpressionNode
}

// implement node type from interface
func (BinaryExpressionNode) NodeType() NodeType { return BinaryExpression }

// Position returns the starting line and column, and the total length of the statement
// The starting line and column aren't always the absolute beginning of the statement just what's most
// convenient.
func (node BinaryExpressionNode) Span() print.TextSpan {
	return node.Left.Span().SpanBetween(node.Right.Span())
}

// node print function
func (node BinaryExpressionNode) Print(indent string) {
	print.PrintC(print.Yellow, indent+"└ BinaryExpressionNode")
	fmt.Printf("%s  └ Operator: %s\n", indent, node.Operator.Kind)
	fmt.Println(indent + "  └ Left: ")
	node.Left.Print(indent + "    ")
	fmt.Println(indent + "  └ Right: ")
	node.Right.Print(indent + "    ")
}

// "constructor" / ooga booga OOP cave man brain
func CreateBinaryExpressionNode(op lexer.Token, left ExpressionNode, right ExpressionNode) BinaryExpressionNode {
	return BinaryExpressionNode{
		Left:     left,
		Operator: op,
		Right:    right,
	}
}
