package nodes

import (
	"fmt"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/lexer"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/print"
)

type EnumDeclarationMember struct {
	MemberNode

	StructKeyword lexer.Token
	Identifier    lexer.Token
	Fields        map[lexer.Token]*LiteralExpressionNode
	ClosingToken  lexer.Token
}

// implement node type from interface
func (EnumDeclarationMember) NodeType() NodeType { return EnumDeclaration }

func (node EnumDeclarationMember) Span() print.TextSpan {
	return node.StructKeyword.Span.SpanBetween(node.ClosingToken.Span)
}

// node print function
func (node EnumDeclarationMember) Print(indent string) {
	print.PrintC(print.Cyan, indent+"- EnumDeclarationMember")
	fmt.Printf("%s  └ Identifier: %s\n", indent, node.Identifier.Kind)
}

// "constructor" / ooga booga OOP cave man brain
func CreateEnumDeclarationMember(kw lexer.Token, id lexer.Token, fields map[lexer.Token]*LiteralExpressionNode, closing lexer.Token) EnumDeclarationMember {
	return EnumDeclarationMember{
		StructKeyword: kw,
		Identifier:    id,
		Fields:        fields,
		ClosingToken:  closing,
	}
}
