package nodes

import (
	"fmt"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/lexer"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/print"
)

// basic global statement member
type FunctionDeclarationMember struct {
	MemberNode

	FunctionKeyword lexer.Token
	Identifier      lexer.Token
	Parameters      []ParameterNode
	TypeClause      TypeClauseNode
	Body            BlockStatementNode
	IsPublic        bool
}

// implement node type from interface
func (FunctionDeclarationMember) NodeType() NodeType { return FunctionDeclaration }

// Position returns the starting line and column, and the total length of the statement
// The starting line and column aren't always the absolute beginning of the statement just what's most
// convenient.
// For FunctionDeclarationMember we don't get the length of the body, only the keyword, name, parameters, and type
func (node FunctionDeclarationMember) Span() print.TextSpan {
	span := node.FunctionKeyword.Span

	if node.FunctionKeyword.Kind == lexer.FunctionKeyword {
		span = span.SpanBetween(node.Body.Span())
	} else {
		span = span.SpanBetween(node.Identifier.Span)
	}

	return span
}

// node print function
func (node FunctionDeclarationMember) Print(indent string) {
	print.PrintC(print.Cyan, indent+"- FunctionDeclarationMember")
	fmt.Printf("%s  └ Identifier: %s\n", indent, node.Identifier.Kind)
	fmt.Printf("%s  └ IsPublic: %t\n", indent, node.IsPublic)

	fmt.Println(indent + "  └ Parameters: ")
	for _, param := range node.Parameters {
		param.Print(indent + "    ")
	}

	if !node.TypeClause.ClauseIsSet {
		fmt.Printf("%s  └ TypeClause: none\n", indent)
	} else {
		fmt.Println(indent + "  └ TypeClause: ")
		node.TypeClause.Print(indent + "    ")
	}

	fmt.Println(indent + "  └ Body: ")
	node.Body.Print(indent + "    ")
}

// "constructor" / ooga booga OOP cave man brain
func CreateFunctionDeclarationMember(kw lexer.Token, id lexer.Token, params []ParameterNode, typeClause TypeClauseNode, body BlockStatementNode, public bool) FunctionDeclarationMember {
	return FunctionDeclarationMember{
		FunctionKeyword: kw,
		Identifier:      id,
		Parameters:      params,
		TypeClause:      typeClause,
		Body:            body,
		IsPublic:        public,
	}
}
