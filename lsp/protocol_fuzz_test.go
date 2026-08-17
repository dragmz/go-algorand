package lsp

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/require"
)

const (
	fuzzMaxProtocolBodyBytes = 4096
	fuzzMaxSourceBytes       = 4096
	fuzzMaxReplacementBytes  = 128
)

func FuzzProtocolFrameBody(f *testing.F) {
	for _, body := range [][]byte{
		[]byte{},
		[]byte(`{`),
		[]byte(`[]`),
		[]byte(`{"jsonrpc":"2.0"}`),
		[]byte(`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"capabilities":{}}}`),
		[]byte(`{"jsonrpc":"2.0","method":"exit"}`),
	} {
		f.Add(body)
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > fuzzMaxProtocolBodyBytes {
			t.Skip()
		}

		input := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
		var output bytes.Buffer
		server, err := New(bytes.NewReader(input), &output)
		require.NoError(t, err)

		_, err = server.Run()
		require.NoError(t, err)
	})
}

func FuzzProtocolTextDocumentRequests(f *testing.F) {
	f.Add("", uint(0), uint(0))
	f.Add("int 1", uint(0), uint(0))
	f.Add(protocolTestSource, uint(7), uint(2))
	f.Add("😀:\nb 😀", uint(1), uint(3))
	f.Add("#pragma version 8\nbyte base64(ABC//==) // comment", uint(1), uint(8))

	f.Fuzz(func(t *testing.T, source string, lineSeed uint, characterSeed uint) {
		if len(source) > fuzzMaxSourceBytes {
			t.Skip()
		}

		line, character, position := fuzzPositionForSource(source, lineSeed, characterSeed)
		frames := runProtocol(t,
			protocolRequest("init", "initialize", initializeParams(map[string]any{
				"semanticTokens": true,
			})),
			didOpenNotification(protocolTestURI, source),
			protocolRequest("diagnostic", "textDocument/diagnostic", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
			}),
			protocolRequest("completion", "textDocument/completion", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"position":     position,
			}),
			protocolRequest("hover", "textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"position":     position,
			}),
			protocolRequest("signature", "textDocument/signatureHelp", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"position":     position,
			}),
			protocolRequest("definition", "textDocument/definition", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"position":     position,
			}),
			protocolRequest("highlight", "textDocument/documentHighlight", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"position":     position,
			}),
			protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
			}),
			protocolRequest("tokens", "textDocument/semanticTokens/full", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
			}),
			protocolRequest("actions", "textDocument/codeAction", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"range":        protocolRange(line, character, line, character),
			}),
			protocolRequest("lenses", "textDocument/codeLens", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
			}),
			protocolRequest("inlays", "textDocument/inlayHint", map[string]any{
				"textDocument": map[string]any{"uri": protocolTestURI},
				"range":        protocolRange(0, 0, len(logic.SourceLinesForTools(source))+1, 0),
			}),
			protocolRequest("shutdown", "shutdown", nil),
			protocolNotification("exit", nil),
		)
		protocolAssertNoResponseErrors(t, frames)
	})
}

func FuzzSourceFeatureConversions(f *testing.F) {
	f.Add("", uint(0), uint(0), "renamed")
	f.Add("int 1", uint(0), uint(0), "renamed")
	f.Add(protocolTestSource, uint(7), uint(6), "renamed")
	f.Add("😀:\nb 😀", uint(1), uint(3), "next")

	f.Fuzz(func(t *testing.T, source string, lineSeed uint, characterSeed uint, replacement string) {
		if len(source) > fuzzMaxSourceBytes || len(replacement) > fuzzMaxReplacementBytes {
			t.Skip()
		}

		result := Process(source)
		line, character, column := fuzzResolvedPosition(result.Lines, lineSeed, characterSeed)

		_ = sourceDiagnosticsToLSP(*result)
		_ = sourceCompletionsAtToLSP(*result, line, column)

		if hover, ok := logic.SourceHoverForTools(*result, line, column); ok {
			require.NotEmpty(t, hover.Text)
		}
		_, _ = logic.SourceSignatureHelpForTools(*result, line, column)

		for _, rg := range sourceDefinitions(*result, line, column) {
			_ = sourceRangeToLSP(result.Lines, rg)
		}
		for _, highlight := range sourceHighlights(*result, line, column) {
			_ = sourceHighlightToLSP(result.Lines, highlight)
		}
		for _, symbol := range sourceDocumentSymbols(*result) {
			_ = sourceDocumentSymbolToLSP(result.Lines, symbol)
		}
		for _, token := range logic.SourceSemanticTokensForTools(*result) {
			_ = sourceSemanticTokenToLSP(result.Lines, token)
		}
		for _, lens := range sourceCodeLenses(*result) {
			_ = sourceCodeLensToLSP(result.Lines, lens)
		}
		for _, inlay := range sourceInlays(*result) {
			_ = sourceInlayToLSP(result.Lines, inlay)
		}

		if rename, ok := sourcePrepareRename(*result, line, column); ok {
			_ = sourceRangeToLSP(result.Lines, rename.Range)
			_ = sourceEditsToLSP(result.Lines, sourceRenameEdits(*result, line, column, replacement))
		}

		rg := logic.SourceRange{Line: line, Column: column, EndLine: line, EndColumn: column}
		for _, action := range logic.SourceActionsForTools(result.Lines, result.Index, result.Program, rg) {
			_ = sourceActionToLSP(protocolTestURI, result.Lines, action)
		}

		_ = sourceRangeFromLSP(result.Lines, protocolRangeToLSP(line, character, line, character))
	})
}

func fuzzPositionForSource(source string, lineSeed uint, characterSeed uint) (int, int, map[string]any) {
	lines := logic.SourceLinesForTools(source)
	line, character, _ := fuzzResolvedPosition(lines, lineSeed, characterSeed)
	return line, character, map[string]any{"line": line, "character": character}
}

func fuzzResolvedPosition(lines []logic.SourceLine, lineSeed uint, characterSeed uint) (int, int, int) {
	if len(lines) == 0 {
		return 0, 0, 0
	}

	line := int(lineSeed % uint(len(lines)))
	maxCharacter := utf16LenString(lines[line].Text) + 2
	character := int(characterSeed % uint(maxCharacter+1))
	return line, character, sourceColumn(lines, line, character)
}

func protocolRangeToLSP(startLine int, startCharacter int, endLine int, endCharacter int) LspRange {
	return LspRange{
		Start: LspPosition{Line: startLine, Character: startCharacter},
		End:   LspPosition{Line: endLine, Character: endCharacter},
	}
}
