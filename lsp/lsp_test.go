package lsp

import (
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/assert"
)

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

func TestSourceEditToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("int 😀")
	edit := sourceEditToLSP(lines, logic.SourceEdit{
		Line:      0,
		Column:    len("int "),
		EndLine:   0,
		EndColumn: len("int 😀"),
		NewText:   "1",
	})
	assert.Equal(t, "1", edit.NewText)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 4},
		End:   LspPosition{Line: 0, Character: 6},
	}, edit.Range)
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

func TestSourceInlayToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("int 😀")
	hint := sourceInlayToLSP(lines, logic.SourceInlay{
		Kind:     logic.SourceInlayNamedValue,
		Position: logic.SourcePosition{Line: 0, Column: len("int 😀")},
		Label:    "value",
	})

	assert.Equal(t, LspPosition{Line: 0, Character: 6}, hint.Position)
	assert.Equal(t, "value", hint.Label)
	assert.NotNil(t, hint.PaddingLeft)
	assert.True(t, *hint.PaddingLeft)
}

func TestSourceInlayRangeFiltering(t *testing.T) {
	lines := logic.SourceLinesForTools("int 1\nint 2")
	inlay := logic.SourceInlay{
		Kind: logic.SourceInlayNamedValue,
		Range: logic.SourceRange{
			Line:      1,
			Column:    len("int "),
			EndLine:   1,
			EndColumn: len("int 2"),
		},
	}

	assert.False(t, sourceInlayInRange(lines, inlay, LspRange{
		Start: LspPosition{Line: 0, Character: 0},
		End:   LspPosition{Line: 0, Character: len("int 1")},
	}))
	assert.True(t, sourceInlayInRange(lines, inlay, LspRange{
		Start: LspPosition{Line: 1, Character: 0},
		End:   LspPosition{Line: 1, Character: len("int 2")},
	}))
}

func TestSourceAnnotationConfigFiltering(t *testing.T) {
	config := tealConfig{
		LensRefs:     true,
		InlayDecoded: true,
	}

	assert.True(t, sourceCodeLensEnabled(config, logic.SourceCodeLens{Kind: logic.SourceCodeLensReferenceCount}))
	assert.False(t, sourceCodeLensEnabled(config, logic.SourceCodeLens{Kind: logic.SourceCodeLensProgramCounter}))
	assert.True(t, sourceInlayEnabled(config, logic.SourceInlay{Kind: logic.SourceInlayDecodedValue}))
	assert.False(t, sourceInlayEnabled(config, logic.SourceInlay{Kind: logic.SourceInlayNamedValue}))
}
