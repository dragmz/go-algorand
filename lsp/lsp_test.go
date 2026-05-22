package lsp

import (
	"fmt"
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

func TestSourceDiagnosticSeverityToLSP(t *testing.T) {
	assert.Equal(t, DiagnosticSeverity(DiagErr), sourceDiagnosticSeverityToLSP(logic.SourceDiagnosticError))
	assert.Equal(t, DiagnosticSeverity(DiagWarn), sourceDiagnosticSeverityToLSP(logic.SourceDiagnosticWarning))
}

func TestSourceDiagnosticsToLSPProgramSize(t *testing.T) {
	result := logic.AnalyzeSourceForTools("int 1")

	diagnostics := sourceDiagnosticsToLSP(result, true)
	assert.Len(t, diagnostics, 1)
	assert.Equal(t, fmt.Sprintf("Program size: %d", len(result.OpStream.Program)), diagnostics[0].Message)
	assert.NotNil(t, diagnostics[0].Severity)
	assert.Equal(t, int(DiagInfo), *diagnostics[0].Severity)

	result = logic.AnalyzeSourceForTools("unknown")
	for _, diagnostic := range sourceDiagnosticsToLSP(result, true) {
		assert.NotNil(t, diagnostic.Severity)
		assert.NotEqual(t, int(DiagInfo), *diagnostic.Severity)
	}
}

func TestSourceDocumentSymbolToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("😀:\nb 😀")
	symbol := sourceDocumentSymbolToLSP(lines, sourceDocumentSymbol{
		Name:           "😀",
		Range:          logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("😀:")},
		SelectionRange: logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("😀")},
	})

	assert.Equal(t, "😀", symbol.Name)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0},
		End:   LspPosition{Line: 0, Character: 3},
	}, symbol.Range)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0},
		End:   LspPosition{Line: 0, Character: 2},
	}, symbol.SelectionRange)
}

func TestSourceHighlightToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("b 😀")
	highlight := sourceHighlightToLSP(lines, sourceHighlight{
		Range: logic.SourceRange{Line: 0, Column: len("b "), EndLine: 0, EndColumn: len("b 😀")},
	})

	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 2},
		End:   LspPosition{Line: 0, Character: 4},
	}, highlight.Range)
	assert.NotNil(t, highlight.Kind)
	assert.Equal(t, symbolHighlightKind, *highlight.Kind)
}

func TestSourceInlayToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("int 😀")
	hint := sourceInlayToLSP(lines, sourceInlay{
		Kind:     sourceInlayNamedValue,
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
	inlay := sourceInlay{
		Kind: sourceInlayNamedValue,
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

	assert.True(t, sourceCodeLensEnabled(config, sourceCodeLens{Kind: sourceCodeLensReferenceCount}))
	assert.False(t, sourceCodeLensEnabled(config, sourceCodeLens{Kind: sourceCodeLensProgramCounter}))
	assert.True(t, sourceInlayEnabled(config, sourceInlay{Kind: sourceInlayDecodedValue}))
	assert.False(t, sourceInlayEnabled(config, sourceInlay{Kind: sourceInlayNamedValue}))
}

func TestSourceNavigationHelpers(t *testing.T) {
	result := logic.AnalyzeSourceForTools("a:\nb a")

	rename, ok := sourcePrepareRename(result, 0, 0)
	assert.True(t, ok)
	assert.Equal(t, sourcePrepareRenameResult{
		Range:       logic.SourceRange{Line: 0, EndLine: 0, EndColumn: 1},
		Placeholder: "a",
	}, rename)

	rename, ok = sourcePrepareRename(result, 1, len("b "))
	assert.True(t, ok)
	assert.Equal(t, logic.SourceRange{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a")}, rename.Range)

	_, ok = sourcePrepareRename(result, 1, 0)
	assert.False(t, ok)

	assert.Equal(t, []logic.SourceEdit{
		{Line: 0, EndLine: 0, EndColumn: 1, NewText: "next"},
		{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a"), NewText: "next"},
	}, sourceRenameEdits(result, 1, len("b "), "next"))

	assert.Equal(t, []logic.SourceRange{{Line: 0, EndLine: 0, EndColumn: 1}}, sourceDefinitions(result, 1, len("b ")))
	assert.Empty(t, sourceDefinitions(result, 0, 0))

	assert.Equal(t, []sourceHighlight{
		{Range: logic.SourceRange{Line: 0, EndLine: 0, EndColumn: 1}},
		{Range: logic.SourceRange{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a")}},
	}, sourceHighlights(result, 0, 0))

	assert.Equal(t, []sourceDocumentSymbol{{
		Name:           "a",
		Range:          logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("a:")},
		SelectionRange: logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("a")},
	}}, sourceDocumentSymbols(result))
}

func TestSourceDocumentSymbolsSkipEmptyNames(t *testing.T) {
	result := logic.SourceAnalysisResult{
		Index: logic.SourceIndex{
			Symbols: []logic.SourceSymbol{
				{Name: "", Kind: logic.SourceSymbolLabel},
				{Name: "valid", Kind: logic.SourceSymbolLabel, EndColumn: len("valid:")},
			},
		},
	}

	assert.Equal(t, []sourceDocumentSymbol{{
		Name:           "valid",
		Range:          logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("valid:")},
		SelectionRange: logic.SourceRange{Line: 0, EndLine: 0, EndColumn: len("valid")},
	}}, sourceDocumentSymbols(result))
}

func TestSourceCodeLenses(t *testing.T) {
	result := logic.AnalyzeSourceForToolsWithOptions("b a\na:\nunused:", logic.SourceToolOptions{Mode: logic.ModeApp})
	lenses := sourceCodeLenses(result)

	refLens, ok := sourceCodeLensByKindForTest(lenses, sourceCodeLensReferenceCount)
	assert.True(t, ok)
	assert.Equal(t, 1, refLens.ReferenceCount)
	assert.Equal(t, logic.SourceRange{Line: 1, EndLine: 1}, refLens.Range)

	for _, lens := range lenses {
		assert.False(t, lens.Kind == sourceCodeLensReferenceCount && lens.Range.Line == 2)
	}

	pcLenses := sourceCodeLensesByKindForTest(sourceCodeLenses(logic.AnalyzeSourceForTools("int 1\nint 2")), sourceCodeLensProgramCounter)
	assert.NotEmpty(t, pcLenses)
	assertProgramCountersSorted(t, sourceCodeLensPCsForTest(pcLenses))
}

func TestSourceInlays(t *testing.T) {
	result := logic.AnalyzeSourceForToolsWithOptions("txn 0\nbyte 0x3031\nint 1", logic.SourceToolOptions{Mode: logic.ModeApp})
	inlays := sourceInlays(result)

	named, ok := sourceInlayByKindForTest(inlays, sourceInlayNamedValue)
	assert.True(t, ok)
	assert.Equal(t, "Sender", named.Label)
	assert.Equal(t, logic.SourcePosition{Line: 0, Column: len("txn 0")}, named.Position)

	decoded, ok := sourceInlayByKindForTest(inlays, sourceInlayDecodedValue)
	assert.True(t, ok)
	assert.Equal(t, "01", decoded.Label)
	assert.Equal(t, logic.SourcePosition{Line: 1, Column: len("byte 0x3031")}, decoded.Position)

	pcInlays := sourceInlaysByKindForTest(inlays, sourceInlayProgramCounter)
	assert.NotEmpty(t, pcInlays)
	assertProgramCountersSorted(t, sourceInlayPCsForTest(pcInlays))
}

func TestSourceCompletionsAtToLSPAddsSnippetsOnlyForOpcodes(t *testing.T) {
	result := logic.AnalyzeSourceForTools("tx")

	opCompletions := sourceCompletionsAtToLSP(result, 0, len("tx"))
	opLabels := completionLabelsForTest(opCompletions)
	assert.Contains(t, opLabels, "soc")
	assert.Contains(t, opLabels, "func")
	assert.Contains(t, opLabels, "txn")

	result = logic.AnalyzeSourceForTools("txn ")
	argCompletions := sourceCompletionsAtToLSP(result, 0, len("txn "))
	argLabels := completionLabelsForTest(argCompletions)
	assert.NotContains(t, argLabels, "soc")
	assert.NotContains(t, argLabels, "func")
	assert.Contains(t, argLabels, "Sender")
}

func completionLabelsForTest(items []lspCompletionItem) map[string]bool {
	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	return labels
}

func sourceCodeLensByKindForTest(lenses []sourceCodeLens, kind sourceCodeLensKind) (sourceCodeLens, bool) {
	for _, lens := range lenses {
		if lens.Kind == kind {
			return lens, true
		}
	}
	return sourceCodeLens{}, false
}

func sourceCodeLensesByKindForTest(lenses []sourceCodeLens, kind sourceCodeLensKind) []sourceCodeLens {
	var found []sourceCodeLens
	for _, lens := range lenses {
		if lens.Kind == kind {
			found = append(found, lens)
		}
	}
	return found
}

func sourceInlayByKindForTest(inlays []sourceInlay, kind sourceInlayKind) (sourceInlay, bool) {
	for _, inlay := range inlays {
		if inlay.Kind == kind {
			return inlay, true
		}
	}
	return sourceInlay{}, false
}

func sourceInlaysByKindForTest(inlays []sourceInlay, kind sourceInlayKind) []sourceInlay {
	var found []sourceInlay
	for _, inlay := range inlays {
		if inlay.Kind == kind {
			found = append(found, inlay)
		}
	}
	return found
}

func sourceCodeLensPCsForTest(lenses []sourceCodeLens) []int {
	var pcs []int
	for _, lens := range lenses {
		pcs = append(pcs, lens.ProgramCounter)
	}
	return pcs
}

func sourceInlayPCsForTest(inlays []sourceInlay) []int {
	var pcs []int
	for _, inlay := range inlays {
		pcs = append(pcs, inlay.ProgramCounter)
	}
	return pcs
}

func assertProgramCountersSorted(t *testing.T, pcs []int) {
	t.Helper()
	for i := 1; i < len(pcs); i++ {
		assert.LessOrEqual(t, pcs[i-1], pcs[i])
	}
}
