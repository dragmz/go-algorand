package lsp

import (
	"fmt"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/assert"
)

func TestPrepareVersionEditForNil(t *testing.T) {
	type test struct {
		token   Range
		version uint64

		text string
		rg   LspRange
	}

	tests := []test{
		{
			token:   nil,
			version: 8,
			text:    "#pragma version 8\r\n",
		},
		{
			token: LspRange{
				Start: LspPosition{
					Line:      1,
					Character: 10,
				},
				End: LspPosition{
					Line:      1,
					Character: 11,
				},
			},
			version: 9,
			text:    "9",
			rg: LspRange{
				Start: LspPosition{
					Line:      1,
					Character: 10,
				},
				End: LspPosition{
					Line:      1,
					Character: 11,
				},
			},
		},
	}

	for i, ts := range tests {
		name := fmt.Sprintf("test #%d", i)

		e := prepareVersionEdit(ts.token, ts.version)

		assert.Equal(t, ts.text, e.NewText, name)
		assert.Equal(t, ts.rg, e.Range, name)
	}
}

func TestPrepareRemoveLineEdit(t *testing.T) {
	e := prepareRemoveLineEdit(123)
	assert.Equal(t, "", e.NewText)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 123},
		End:   LspPosition{Line: 124},
	}, e.Range)
}

func TestUtf16LenString(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"hello", 5},
		{"€", 1},
		{"Café", 4},
		{"e\u0301", 2},
		{"e\u0301\u0323", 3},
		{"😀", 2},
		{"a😀b", 4},
		{"👍\U0001F3FD", 4},
		{"✌️", 2},
		{"1\uFE0F\u20E3", 3},
		{"🇺🇸", 4},
		{"👨\u200D👩\u200D👦", 8},
		{"漢字", 2},
		{"\U0002000B", 2},
		{"\U0001D11E\U0001D11E\U0001D11E", 6},
		{"\U00010437\u0301", 3},
	}

	for _, tt := range tests {
		got := utf16LenString(tt.s)
		if got != tt.want {
			t.Fatalf("utf16LenString(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestByteColumnFromUTF16Column(t *testing.T) {
	assert.Equal(t, len("int "), byteColumnFromUTF16Column("int 😀", 4))
	assert.Equal(t, len("int 😀"), byteColumnFromUTF16Column("int 😀", 6))
	assert.Equal(t, len("int 😀"), byteColumnFromUTF16Column("int 😀", 99))
}

func TestSourceDiagnosticToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("int 😀")
	diag := sourceDiagnosticToLSP(lines, logic.SourceDiagnostic{
		Message:   "bad emoji",
		Severity:  logic.SourceDiagnosticError,
		Line:      0,
		Column:    len("int "),
		EndColumn: len("int 😀"),
	})

	assert.Equal(t, "bad emoji", diag.Message)
	assert.NotNil(t, diag.Severity)
	assert.Equal(t, int(DiagErr), *diag.Severity)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 4},
		End:   LspPosition{Line: 0, Character: 6},
	}, diag.Range)
}
