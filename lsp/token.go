package lsp

type TokenType string

const (
	TokenSemicolon = ";"
	TokenValue     = "Value"
	TokenComment   = "Comment"
)

type Token struct {
	v string

	l int
	b int
	e int

	t TokenType
}

func (t Token) StartLine() int {
	return t.l
}

func (t Token) StartCharacter() int {
	return t.b
}

func (t Token) EndLine() int {
	return t.l
}

func (t Token) EndCharacter() int {
	return t.e
}

func (t Token) String() string {
	return t.v
}

func (t Token) Line() int {
	return t.l
}

func (t Token) Begin() int {
	return t.b
}

func (t Token) End() int {
	return t.e
}

func (t Token) Type() TokenType {
	return t.t
}
