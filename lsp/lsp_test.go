package lsp

import (
	"fmt"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSourceDiagnosticsToLSPReportsOnlyAssemblyDiagnostics(t *testing.T) {
	assert.Empty(t, sourceDiagnosticsToLSP(logic.AnalyzeSourceForTools("int 1")))

	for _, diagnostic := range sourceDiagnosticsToLSP(logic.AnalyzeSourceForTools("unknown")) {
		assert.NotNil(t, diagnostic.Severity)
		assert.NotEqual(t, int(DiagInfo), *diagnostic.Severity)
	}
}

func TestSourceCodeLensesProgramSize(t *testing.T) {
	result := logic.AnalyzeSourceForTools("int 1")

	lens, ok := sourceCodeLensByKindForTest(sourceCodeLenses(result, tealConfig{ProgramSize: true}), sourceCodeLensProgramSize)
	require.True(t, ok)
	assert.Equal(t, len(result.OpStream.Program), lens.ProgramSize)

	cl := sourceCodeLensToLSP(result.Lines, protocolTestURI, lens)
	require.NotNil(t, cl.Command)
	assert.Equal(t, fmt.Sprintf("size: %d bytes", len(result.OpStream.Program)), cl.Command.Title)
	assert.Equal(t, LspRange{}, cl.Range)
	// A label needs no resolving, so it carries its command straight away.
	assert.Nil(t, cl.Data)

	// A program that does not assemble has no size to report.
	_, ok = sourceCodeLensByKindForTest(sourceCodeLenses(logic.AnalyzeSourceForTools("unknown"), tealConfig{ProgramSize: true}), sourceCodeLensProgramSize)
	assert.False(t, ok)
}

func TestSourceCodeLensReferenceCountResolves(t *testing.T) {
	const source = "#pragma version 8\nstart:\n  b start\n  b start"
	result := logic.AnalyzeSourceForTools(source)

	lens, ok := sourceCodeLensByKindForTest(sourceCodeLenses(result, tealConfig{LensRefs: true}), sourceCodeLensReferenceCount)
	require.True(t, ok)

	// On the wire the lens is bare: no command, and no locations to carry, only
	// where to look them up.
	cl := sourceCodeLensToLSP(result.Lines, protocolTestURI, lens)
	assert.Nil(t, cl.Command)
	require.Equal(t, &lspCodeLensData{Uri: protocolTestURI, Line: 1, Column: 0}, cl.Data)

	resolved := sourceCodeLensResolveToLSP(result, lspCodeLensResolveParams{
		Range: cl.Range,
		Data:  cl.Data.(*lspCodeLensData),
	})

	assert.Equal(t, cl.Range, resolved.Range)
	require.NotNil(t, resolved.Command)
	assert.Equal(t, "refs: 2", resolved.Command.Title)
	assert.Equal(t, tealShowReferencesCommand, resolved.Command.Command)

	// The client gets what the built-in peek needs: where it was invoked, and
	// every place the name is used.
	require.Len(t, resolved.Command.Arguments, 3)
	assert.Equal(t, protocolTestURI, resolved.Command.Arguments[0])
	assert.Equal(t, LspPosition{Line: 1, Character: 0}, resolved.Command.Arguments[1])
	locations, ok := resolved.Command.Arguments[2].([]lspLocation)
	require.True(t, ok)
	require.Len(t, locations, 2)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 2, Character: len("  b ")},
		End:   LspPosition{Line: 2, Character: len("  b start")},
	}, locations[0].Range)

	// The count the lens shows comes from the same walk as the locations, so it
	// matches what the peek lists.
	assert.Equal(t, fmt.Sprintf("refs: %d", len(locations)), resolved.Command.Title)
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
	highlight := sourceHighlightToLSP(lines,
		logic.SourceRange{Line: 0, Column: len("b "), EndLine: 0, EndColumn: len("b 😀")})

	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 2},
		End:   LspPosition{Line: 0, Character: 4},
	}, highlight.Range)
	assert.NotNil(t, highlight.Kind)
	assert.Equal(t, symbolHighlightKind, *highlight.Kind)
}

func TestSourceSelectionRangeToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("int 😀")

	sr := sourceSelectionRangeToLSP(lines, LspPosition{}, []logic.SourceRange{
		{Line: 0, Column: len("int "), EndLine: 0, EndColumn: len("int 😀")},
		{Line: 0, Column: 0, EndLine: 0, EndColumn: len("int 😀")},
	})

	// The innermost rung is the answer and the wider ones hang off it. Columns
	// are UTF-16, so the emoji counts for two.
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 4},
		End:   LspPosition{Line: 0, Character: 6},
	}, sr.Range)
	require.NotNil(t, sr.Parent)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 0},
		End:   LspPosition{Line: 0, Character: 6},
	}, sr.Parent.Range)
	assert.Nil(t, sr.Parent.Parent)

	// A position with nothing to widen still answers, because the client pairs
	// answers with the positions it sent by index.
	empty := sourceSelectionRangeToLSP(lines, LspPosition{Line: 9, Character: 3}, nil)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 9, Character: 3},
		End:   LspPosition{Line: 9, Character: 3},
	}, empty.Range)
	assert.Nil(t, empty.Parent)
}

func TestSourceLocationsToLSP(t *testing.T) {
	lines := logic.SourceLinesForTools("b 😀")

	locations := sourceLocationsToLSP("file:///a.teal", lines, []logic.SourceRange{
		{Line: 0, Column: len("b "), EndLine: 0, EndColumn: len("b 😀")},
	})

	require.Len(t, locations, 1)
	assert.Equal(t, "file:///a.teal", locations[0].Uri)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 0, Character: 2},
		End:   LspPosition{Line: 0, Character: 4},
	}, locations[0].Range)

	assert.Empty(t, sourceLocationsToLSP("file:///a.teal", lines, nil))
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

func TestSourceCodeLensesBuildOnlyEnabledKinds(t *testing.T) {
	result := logic.AnalyzeSourceForTools("#pragma version 8\nstart:\nb start")

	assert.Empty(t, sourceCodeLenses(result, tealConfig{}))

	refs := sourceCodeLenses(result, tealConfig{LensRefs: true})
	require.Len(t, refs, 1)
	assert.Equal(t, sourceCodeLensReferenceCount, refs[0].Kind)

	pcs := sourceCodeLenses(result, tealConfig{PcLens: true})
	require.NotEmpty(t, pcs)
	for _, lens := range pcs {
		assert.Equal(t, sourceCodeLensProgramCounter, lens.Kind)
	}
}

func TestSourceInlaysBuildOnlyEnabledKindsInRange(t *testing.T) {
	result := logic.AnalyzeSourceForTools("txn 0\nbyte 0x3031")

	assert.Empty(t, sourceInlays(result, tealConfig{}, logic.SourceAllLines))

	named := sourceInlays(result, tealConfig{InlayNamed: true}, logic.SourceAllLines)
	require.Len(t, named, 1)
	assert.Equal(t, sourceInlayNamedValue, named[0].Kind)

	decoded := sourceInlays(result, tealConfig{InlayDecoded: true}, logic.SourceAllLines)
	require.Len(t, decoded, 1)
	assert.Equal(t, sourceInlayDecodedValue, decoded[0].Kind)

	// The range narrows what is built, not what is kept.
	both := tealConfig{InlayNamed: true, InlayDecoded: true}
	assert.Len(t, sourceInlays(result, both, logic.SourceAllLines), 2)
	assert.Len(t, sourceInlays(result, both, logic.SourceLineRange{Start: 1, End: 2}), 1)
	assert.Empty(t, sourceInlays(result, both, logic.SourceLineRange{}))
}

func TestSourceLineRangeFromLSP(t *testing.T) {
	rg := sourceLineRangeFromLSP(LspRange{
		Start: LspPosition{Line: 3, Character: 7},
		End:   LspPosition{Line: 5, Character: 0},
	})

	assert.False(t, rg.Contains(2))
	assert.True(t, rg.Contains(3))
	// The end line counts, even though the range stops at its first character.
	assert.True(t, rg.Contains(5))
	assert.False(t, rg.Contains(6))
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

	declaration := logic.SourceRange{Line: 0, EndLine: 0, EndColumn: 1}
	reference := logic.SourceRange{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a")}

	// Highlighting wants the declaration, a references request only when asked.
	assert.Equal(t, []logic.SourceRange{declaration, reference}, sourceOccurrences(result, 0, 0, true))
	assert.Equal(t, []logic.SourceRange{reference}, sourceOccurrences(result, 0, 0, false))

	// The same answer from the reference as from the declaration.
	assert.Equal(t, []logic.SourceRange{declaration, reference}, sourceOccurrences(result, 1, len("b "), true))
	assert.Empty(t, sourceOccurrences(result, 1, 0, true))

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

// allAnnotationsForTest enables every lens and inlay kind, for tests about what
// gets built rather than about the configuration gate.
var allAnnotationsForTest = tealConfig{
	SemanticTokens: true,
	InlayNamed:     true,
	InlayDecoded:   true,
	LensRefs:       true,
	PcLens:         true,
	PcInlay:        true,
	ProgramSize:    true,
}

func TestSourceCodeLenses(t *testing.T) {
	result := logic.AnalyzeSourceForToolsWithOptions("b a\na:\nunused:", logic.SourceToolOptions{Mode: logic.ModeApp})
	lenses := sourceCodeLenses(result, allAnnotationsForTest)

	refLens, ok := sourceCodeLensByKindForTest(lenses, sourceCodeLensReferenceCount)
	assert.True(t, ok)
	assert.Equal(t, 1, refLens.ReferenceCount)
	assert.Equal(t, logic.SourceRange{Line: 1, EndLine: 1}, refLens.Range)

	for _, lens := range lenses {
		assert.False(t, lens.Kind == sourceCodeLensReferenceCount && lens.Range.Line == 2)
	}

	pcLenses := sourceCodeLensesByKindForTest(sourceCodeLenses(logic.AnalyzeSourceForTools("int 1\nint 2"), allAnnotationsForTest), sourceCodeLensProgramCounter)
	assert.NotEmpty(t, pcLenses)
	assertProgramCountersSorted(t, sourceCodeLensPCsForTest(pcLenses))
}

func TestSourceInlays(t *testing.T) {
	result := logic.AnalyzeSourceForToolsWithOptions("txn 0\nbyte 0x3031\nint 1", logic.SourceToolOptions{Mode: logic.ModeApp})
	inlays := sourceInlays(result, allAnnotationsForTest, logic.SourceAllLines)

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
	opLabels := completionLabelsForTest(opCompletions.Items)
	assert.Contains(t, opLabels, "soc")
	assert.Contains(t, opLabels, "func")
	assert.Contains(t, opLabels, "txn")

	result = logic.AnalyzeSourceForTools("txn ")
	argCompletions := sourceCompletionsAtToLSP(result, 0, len("txn "))
	argLabels := completionLabelsForTest(argCompletions.Items)
	assert.NotContains(t, argLabels, "soc")
	assert.NotContains(t, argLabels, "func")
	assert.Contains(t, argLabels, "Sender")
}

func TestSourceCompletionsAtToLSPHoldsBackDocumentation(t *testing.T) {
	result := logic.AnalyzeSourceForTools("tx")

	completions := sourceCompletionsAtToLSP(result, 0, len("tx"))
	for _, item := range completions.Items {
		assert.Nil(t, item.Documentation, "item %q carries documentation", item.Label)
	}

	// The documentation is kept aside under the label that resolve will ask for.
	assert.NotEmpty(t, completions.Docs["txn"])

	// Snippets have no documentation to hold back.
	assert.NotContains(t, completions.Docs, "soc")

	args := sourceCompletionsAtToLSP(logic.AnalyzeSourceForTools("txn "), 0, len("txn "))
	assert.NotEmpty(t, args.Docs["Sender"])
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
