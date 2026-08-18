package lsp

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const protocolTestURI = "file:///test.teal"

const protocolTestSource = `#pragma version 8
#define VALUE 1
start:
  txn Sender
  byte 0x3031
  b start
unused:
  int VALUE

`

type protocolMessage struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *lspError       `json:"error,omitempty"`
}

func TestProtocolInitializeLifecycleAndDocumentCache(t *testing.T) {
	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(map[string]any{
			"semanticTokens": false,
			"inlayNamed":     false,
			"inlayDecoded":   false,
			"programSize":    false,
		})),
		protocolNotification("initialized", map[string]any{}),
		didOpenNotification(protocolTestURI, "int 1"),
		protocolRequest("diag-ok", "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		didChangeNotification(protocolTestURI, "unknown"),
		protocolRequest("diag-err", "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolNotification("textDocument/didSave", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolNotification("textDocument/didClose", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("diag-closed", "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	init := protocolResultAs[lspInitializeResult](t, protocolResponseByID(t, frames, "init"))
	require.NotNil(t, init.Capabilities)
	require.Nil(t, init.Capabilities.SemanticTokensProvider)
	require.NotNil(t, init.Capabilities.InlayHintProvider)
	assert.False(t, *init.Capabilities.InlayHintProvider)

	okDiagnostics := protocolResultAs[lspFullDocumentDiagnosticReport](t, protocolResponseByID(t, frames, "diag-ok"))
	assert.Empty(t, okDiagnostics.Items)

	errDiagnostics := protocolResultAs[lspFullDocumentDiagnosticReport](t, protocolResponseByID(t, frames, "diag-err"))
	require.NotEmpty(t, errDiagnostics.Items)
	assert.Contains(t, errDiagnostics.Items[0].Message, "unknown opcode")

	closedDiagnostics := protocolResultAs[lspFullDocumentDiagnosticReport](t, protocolResponseByID(t, frames, "diag-closed"))
	assert.Empty(t, closedDiagnostics.Items)
	protocolAssertSuccess(t, protocolResponseByID(t, frames, "shutdown"))
}

func TestProtocolTextDocumentOperations(t *testing.T) {
	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(map[string]any{
			"semanticTokens": true,
			"inlayNamed":     true,
			"inlayDecoded":   true,
			"lensRefs":       true,
			"pcLens":         true,
			"pcInlay":        true,
			"programSize":    true,
		})),
		didOpenNotification(protocolTestURI, protocolTestSource),
		protocolRequest("diagnostic", "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("completion-op", "textDocument/completion", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 8, "character": 0},
		}),
		protocolRequest("resolve", "completionItem/resolve", map[string]any{"label": "txn"}),
		protocolRequest("resolve-unknown", "completionItem/resolve", map[string]any{"label": "nosuchopcode"}),
		protocolRequest("completion-arg", "textDocument/completion", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": len("  txn ")},
		}),
		protocolRequest("hover", "textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": len("  tx")},
		}),
		protocolRequest("hover-label", "textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 5, "character": len("  b ")},
		}),
		protocolRequest("hover-none", "textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 8, "character": 0},
		}),
		protocolRequest("signature", "textDocument/signatureHelp", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": len("  txn ")},
		}),
		protocolRequest("definition", "textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 5, "character": len("  b ")},
		}),
		protocolRequest("references", "textDocument/references", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 2, "character": 0},
			"context":      map[string]any{"includeDeclaration": true},
		}),
		protocolRequest("references-only", "textDocument/references", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 5, "character": len("  b ")},
			"context":      map[string]any{"includeDeclaration": false},
		}),
		protocolRequest("selection", "textDocument/selectionRange", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"positions": []any{
				map[string]any{"line": 3, "character": len("  txn Se")},
				map[string]any{"line": 8, "character": 0},
			},
		}),
		protocolRequest("prepare-rename", "textDocument/prepareRename", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 2, "character": 0},
		}),
		protocolRequest("rename", "textDocument/rename", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 2, "character": 0},
			"newName":      "entry",
		}),
		protocolRequest("highlight", "textDocument/documentHighlight", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 2, "character": 0},
		}),
		protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("tokens", "textDocument/semanticTokens/full", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("tokens-range", "textDocument/semanticTokens/range", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"range":        protocolRange(2, 0, 3, 0),
		}),
		protocolRequest("actions", "textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"range":        protocolRange(6, 0, 6, len("unused:")),
			"context":      map[string]any{"diagnostics": []any{}},
		}),
		protocolRequest("lenses", "textDocument/codeLens", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("inlays", "textDocument/inlayHint", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"range":        protocolRange(0, 0, 99, 0),
		}),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	protocolAssertNoResponseErrors(t, frames)
	init := protocolResultAs[lspInitializeResult](t, protocolResponseByID(t, frames, "init"))
	require.NotNil(t, init.Capabilities.SemanticTokensProvider)
	require.NotNil(t, init.Capabilities.InlayHintProvider)
	assert.True(t, *init.Capabilities.InlayHintProvider)

	diagnostics := protocolResultAs[lspFullDocumentDiagnosticReport](t, protocolResponseByID(t, frames, "diagnostic"))
	assert.Equal(t, "full", diagnostics.Kind)
	assert.Empty(t, diagnostics.Items)

	opCompletions := protocolResultAs[[]lspCompletionItem](t, protocolResponseByID(t, frames, "completion-op"))
	opLabels := protocolCompletionLabels(opCompletions)
	assert.Contains(t, opLabels, "soc")
	assert.Contains(t, opLabels, "func")
	assert.Contains(t, opLabels, "txn")

	// Documentation is the bulk of a completion list and travels only on resolve.
	for _, item := range opCompletions {
		assert.Nil(t, item.Documentation, "item %q carries documentation", item.Label)
	}

	resolved := protocolResultAs[lspCompletionItem](t, protocolResponseByID(t, frames, "resolve"))
	assert.Equal(t, "txn", resolved.Label)
	docs, ok := resolved.Documentation.(map[string]any)
	require.True(t, ok, "documentation: %#v", resolved.Documentation)
	assert.Equal(t, "markdown", docs["kind"])
	assert.NotEmpty(t, docs["value"])

	// An item the last list never offered resolves to itself, unchanged.
	unknown := protocolResultAs[lspCompletionItem](t, protocolResponseByID(t, frames, "resolve-unknown"))
	assert.Equal(t, "nosuchopcode", unknown.Label)
	assert.Nil(t, unknown.Documentation)

	argCompletions := protocolResultAs[[]lspCompletionItem](t, protocolResponseByID(t, frames, "completion-arg"))
	assert.Contains(t, protocolCompletionLabels(argCompletions), "Sender")

	hover := protocolResultAs[lspHover](t, protocolResponseByID(t, frames, "hover"))
	assert.NotEmpty(t, hover.Contents.Value)
	assert.Equal(t, "markdown", hover.Contents.Kind)
	require.NotNil(t, hover.Range)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 3, Character: len("  ")},
		End:   LspPosition{Line: 3, Character: len("  txn")},
	}, *hover.Range)

	hoverLabel := protocolResultAs[lspHover](t, protocolResponseByID(t, frames, "hover-label"))
	assert.Contains(t, hoverLabel.Contents.Value, "`start:`")
	assert.Contains(t, hoverLabel.Contents.Value, "1 reference")
	require.NotNil(t, hoverLabel.Range)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 5, Character: len("  b ")},
		End:   LspPosition{Line: 5, Character: len("  b start")},
	}, *hoverLabel.Range)

	// Nothing to describe answers with an explicit null rather than an object
	// that is not a valid Hover.
	assert.Equal(t, "null", string(protocolResponseByID(t, frames, "hover-none").Result))

	signature := protocolResultAs[lspSignatureHelp](t, protocolResponseByID(t, frames, "signature"))
	require.NotEmpty(t, signature.Signatures)
	assert.Contains(t, signature.Signatures[0].Label, "txn")

	definitions := protocolResultAs[[]lspLocation](t, protocolResponseByID(t, frames, "definition"))
	require.Len(t, definitions, 1)
	assert.Equal(t, protocolTestURI, definitions[0].Uri)

	// Asked at the declaration, references answers with the declaration and the
	// one branch that names it, in source order.
	references := protocolResultAs[[]lspLocation](t, protocolResponseByID(t, frames, "references"))
	require.Len(t, references, 2)
	assert.Equal(t, protocolTestURI, references[0].Uri)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 2, Character: 0},
		End:   LspPosition{Line: 2, Character: len("start")},
	}, references[0].Range)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 5, Character: len("  b ")},
		End:   LspPosition{Line: 5, Character: len("  b start")},
	}, references[1].Range)

	// Asked at the branch, with the declaration excluded, only the branch is left.
	referencesOnly := protocolResultAs[[]lspLocation](t, protocolResponseByID(t, frames, "references-only"))
	require.Len(t, referencesOnly, 1)
	assert.Equal(t, references[1].Range, referencesOnly[0].Range)

	selections := protocolResultAs[[]lspSelectionRange](t, protocolResponseByID(t, frames, "selection"))
	require.Len(t, selections, 2)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 3, Character: len("  txn ")},
		End:   LspPosition{Line: 3, Character: len("  txn Sender")},
	}, selections[0].Range)
	require.NotNil(t, selections[0].Parent)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 3, Character: len("  ")},
		End:   LspPosition{Line: 3, Character: len("  txn Sender")},
	}, selections[0].Parent.Range)
	require.NotNil(t, selections[0].Parent.Parent)
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 3, Character: 0},
		End:   LspPosition{Line: 3, Character: len("  txn Sender")},
	}, selections[0].Parent.Parent.Range)
	assert.Nil(t, selections[0].Parent.Parent.Parent)

	// A blank line answers with itself, so the array lines up with the positions.
	assert.Equal(t, LspRange{
		Start: LspPosition{Line: 8, Character: 0},
		End:   LspPosition{Line: 8, Character: 0},
	}, selections[1].Range)
	assert.Nil(t, selections[1].Parent)

	prepareRename := protocolResultAs[lspPrepareRenameResponse](t, protocolResponseByID(t, frames, "prepare-rename"))
	assert.Equal(t, "start", prepareRename.Placeholder)

	rename := protocolResultAs[lspWorkspaceEdit](t, protocolResponseByID(t, frames, "rename"))
	require.Len(t, rename.Changes[protocolTestURI], 2)

	highlights := protocolResultAs[[]lspDocumentHighlight](t, protocolResponseByID(t, frames, "highlight"))
	require.Len(t, highlights, 2)

	symbols := protocolResultAs[[]LspDocumentSymbol](t, protocolResponseByID(t, frames, "symbols"))
	require.NotEmpty(t, symbols)
	for _, symbol := range symbols {
		assert.NotEmpty(t, symbol.Name)
	}

	tokens := protocolResultAs[lspSemanticTokens](t, protocolResponseByID(t, frames, "tokens"))
	require.NotEmpty(t, tokens.Data)
	assert.Zero(t, len(tokens.Data)%5)

	// A range answers with the tokens of the lines it spans and nothing else.
	tokensRange := protocolResultAs[lspSemanticTokens](t, protocolResponseByID(t, frames, "tokens-range"))
	require.NotEmpty(t, tokensRange.Data)
	assert.Zero(t, len(tokensRange.Data)%5)
	assert.Less(t, len(tokensRange.Data), len(tokens.Data))

	actions := protocolResultAs[[]lspCodeAction](t, protocolResponseByID(t, frames, "actions"))
	require.NotEmpty(t, actions)
	assert.Contains(t, protocolCodeActionTitles(actions), "Remove label 'unused'")

	lenses := protocolResultAs[[]LspCodeLens](t, protocolResponseByID(t, frames, "lenses"))
	require.NotEmpty(t, lenses)
	assert.True(t, protocolCodeLensTitleContains(lenses, "refs:"))
	assert.True(t, protocolCodeLensTitleContains(lenses, "pc:"))
	assert.True(t, protocolCodeLensTitleContains(lenses, "size:"))

	inlays := protocolResultAs[[]LspInlayHint](t, protocolResponseByID(t, frames, "inlays"))
	require.NotEmpty(t, inlays)
	assert.Contains(t, protocolInlayLabels(inlays), "01")
	assert.True(t, protocolInlayLabelHasPrefix(inlays, "pc:"))
}

func TestProtocolCapabilitiesAdvertiseTestedOperations(t *testing.T) {
	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(map[string]any{
			"semanticTokens": true,
			"inlayNamed":     true,
			"inlayDecoded":   true,
			"lensRefs":       true,
			"pcLens":         true,
			"pcInlay":        true,
			"programSize":    true,
		})),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	init := protocolResultAs[lspInitializeResult](t, protocolResponseByID(t, frames, "init"))
	require.NotNil(t, init.Capabilities)
	assert.NotNil(t, init.Capabilities.DiagnosticProvider)
	assert.NotNil(t, init.Capabilities.CompletionProvider)
	assert.NotNil(t, init.Capabilities.DocumentSymbolProvider)
	assert.True(t, *init.Capabilities.DocumentSymbolProvider)
	assert.NotNil(t, init.Capabilities.CodeActionProvider)
	assert.True(t, *init.Capabilities.CodeActionProvider)
	assert.NotNil(t, init.Capabilities.RenameProvider)
	assert.NotNil(t, init.Capabilities.RenameProvider.PrepareProvider)
	assert.True(t, *init.Capabilities.RenameProvider.PrepareProvider)
	assert.NotNil(t, init.Capabilities.DocumentHighlightProvider)
	assert.True(t, *init.Capabilities.DocumentHighlightProvider)
	require.NotNil(t, init.Capabilities.SemanticTokensProvider)
	require.NotNil(t, init.Capabilities.SemanticTokensProvider.Full)
	assert.True(t, *init.Capabilities.SemanticTokensProvider.Full)
	require.NotNil(t, init.Capabilities.SemanticTokensProvider.Range)
	assert.True(t, *init.Capabilities.SemanticTokensProvider.Range)
	require.NotNil(t, init.Capabilities.CompletionProvider.ResolveProvider)
	assert.True(t, *init.Capabilities.CompletionProvider.ResolveProvider)
	assert.NotNil(t, init.Capabilities.DefinitionProvider)
	assert.True(t, *init.Capabilities.DefinitionProvider)
	require.NotNil(t, init.Capabilities.ReferencesProvider)
	assert.True(t, *init.Capabilities.ReferencesProvider)
	require.NotNil(t, init.Capabilities.SelectionRangeProvider)
	assert.True(t, *init.Capabilities.SelectionRangeProvider)
	assert.NotNil(t, init.Capabilities.HoverProvider)
	assert.True(t, *init.Capabilities.HoverProvider)
	assert.NotNil(t, init.Capabilities.SignatureHelpProvider)
	assert.NotNil(t, init.Capabilities.InlayHintProvider)
	assert.True(t, *init.Capabilities.InlayHintProvider)
	assert.NotNil(t, init.Capabilities.CodeLensProvider)

	require.NotNil(t, init.Capabilities.ExecuteCommandProvider)
	advertised := protocolCommandSet(init.Capabilities.ExecuteCommandProvider.Commands)
	for _, command := range protocolTestedWorkspaceCommands() {
		assert.Contains(t, advertised, command)
	}
}

func TestProtocolDocumentSymbolsRejectEmptyNames(t *testing.T) {
	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(nil)),
		didOpenNotification(protocolTestURI, ":"),
		protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	symbols := protocolResultAs[[]LspDocumentSymbol](t, protocolResponseByID(t, frames, "symbols"))
	assert.Empty(t, symbols)
}

func TestProtocolWorkspaceCommands(t *testing.T) {
	ops, err := logic.AssembleString("#pragma version 8\nint 1")
	require.NoError(t, err)

	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(map[string]any{"programSize": false})),
		didOpenNotification(protocolTestURI, protocolTestSource),
		protocolRequest("sourcemap", "workspace/executeCommand", map[string]any{
			"command":   "teal.sourcemap.generate",
			"arguments": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("decompile", "workspace/executeCommand", map[string]any{
			"command":   "teal.decompile",
			"arguments": map[string]any{"bytecode": base64.StdEncoding.EncodeToString(ops.Program)},
		}),
		protocolRequest("pc", "workspace/executeCommand", map[string]any{
			"command":   "teal.pc.resolve",
			"arguments": map[string]any{"uri": protocolTestURI, "pc": 1},
		}),
		protocolRequest("version", "workspace/executeCommand", map[string]any{
			"command":   "teal.version.update",
			"arguments": []any{map[string]any{"uri": protocolTestURI, "version": 9}},
		}),
		protocolRequest("replace", "workspace/executeCommand", map[string]any{
			"command": "teal.value.replace",
			"arguments": []any{map[string]any{
				"uri":   protocolTestURI,
				"range": protocolRange(7, len("  int "), 7, len("  int VALUE")),
				"name":  "1",
			}},
		}),
		protocolRequest("call-remove", "workspace/executeCommand", map[string]any{
			"command":   "teal.call.remove",
			"arguments": []any{map[string]any{"uri": protocolTestURI, "line": 5, "statement": 0}},
		}),
		protocolRequest("label-remove", "workspace/executeCommand", map[string]any{
			"command":   "teal.label.remove",
			"arguments": []any{map[string]any{"uri": protocolTestURI, "name": "unused"}},
		}),
		protocolRequest("label-create", "workspace/executeCommand", map[string]any{
			"command":   "teal.label.create",
			"arguments": []any{map[string]any{"uri": protocolTestURI, "name": "created"}},
		}),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	protocolAssertNoResponseErrors(t, frames)

	sourceMap := protocolResultAs[tealGenerateSourcemapCommandResult](t, protocolResponseByID(t, frames, "sourcemap"))
	assert.NotEmpty(t, sourceMap.SourceMap.Mappings)

	decompiled := protocolResultAs[tealDecompileCommandResult](t, protocolResponseByID(t, frames, "decompile"))
	assert.NotEmpty(t, decompiled.Teal)

	pc := protocolResultAs[LspPosition](t, protocolResponseByID(t, frames, "pc"))
	assert.Equal(t, 3, pc.Line)

	for _, id := range []string{"version", "replace", "call-remove", "label-remove", "label-create"} {
		protocolAssertSuccess(t, protocolResponseByID(t, frames, id))
	}

	applyEdits := protocolRequestsByMethod(frames, "workspace/applyEdit")
	require.Len(t, applyEdits, 5)
	for _, req := range applyEdits {
		var params lspWorkspaceApplyEditRequestParams
		require.NoError(t, json.Unmarshal(req.Params, &params))
		require.NotEmpty(t, params.Edit.DocumentChanges)
	}
}

func TestProtocolWorkspaceCommandErrors(t *testing.T) {
	frames := runProtocol(t,
		protocolRequest("init", "initialize", initializeParams(nil)),
		protocolRequest("unknown-command", "workspace/executeCommand", map[string]any{
			"command":   "teal.unknown",
			"arguments": []any{},
		}),
		protocolRequest("bad-params", "workspace/executeCommand", map[string]any{
			"command":   "teal.label.create",
			"arguments": []any{},
		}),
		protocolRequest("shutdown", "shutdown", nil),
		protocolNotification("exit", nil),
	)

	unknown := protocolResponseByID(t, frames, "unknown-command")
	require.NotNil(t, unknown.Error)
	assert.Equal(t, ErrorCodeMethodNotFound, unknown.Error.Code)

	badParams := protocolResponseByID(t, frames, "bad-params")
	require.NotNil(t, badParams.Error)
	assert.Equal(t, ErrorCodeInvalidParams, badParams.Error.Code)
}

func TestProtocolExamplesSmoke(t *testing.T) {
	root := "examples"
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || filepath.Ext(path) != ".teal" {
			return nil
		}

		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			require.NoError(t, err)
			uri := "file:///" + filepath.ToSlash(path)
			frames := runProtocol(t,
				protocolRequest("init", "initialize", initializeParams(map[string]any{
					"semanticTokens": true,
					"inlayNamed":     true,
					"inlayDecoded":   true,
					"programSize":    true,
				})),
				didOpenNotification(uri, string(source)),
				protocolRequest("diagnostic", "textDocument/diagnostic", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				}),
				protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				}),
				protocolRequest("tokens", "textDocument/semanticTokens/full", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				}),
				protocolRequest("completion", "textDocument/completion", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 0, "character": 0},
				}),
				protocolRequest("actions", "textDocument/codeAction", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"range":        protocolRange(0, 0, 0, 0),
					"context":      map[string]any{"diagnostics": []any{}},
				}),
				protocolRequest("shutdown", "shutdown", nil),
				protocolNotification("exit", nil),
			)

			protocolAssertNoResponseErrors(t, frames)
			diagnostics := protocolResultAs[lspFullDocumentDiagnosticReport](t, protocolResponseByID(t, frames, "diagnostic"))
			for _, diagnostic := range diagnostics.Items {
				assert.NotEmpty(t, diagnostic.Message)
			}
			symbols := protocolResultAs[[]LspDocumentSymbol](t, protocolResponseByID(t, frames, "symbols"))
			for _, symbol := range symbols {
				assert.NotEmpty(t, symbol.Name)
			}
			protocolResultAs[lspSemanticTokens](t, protocolResponseByID(t, frames, "tokens"))
			protocolResultAs[[]lspCompletionItem](t, protocolResponseByID(t, frames, "completion"))
			protocolResultAs[[]lspCodeAction](t, protocolResponseByID(t, frames, "actions"))
		})
		return nil
	})
	require.NoError(t, err)
}

func runProtocol(t testing.TB, messages ...map[string]any) []protocolMessage {
	t.Helper()

	return runProtocolInput(t, protocolFrames(t, messages...))
}

func runProtocolInput(t testing.TB, inputData []byte) []protocolMessage {
	t.Helper()

	input := bytes.NewBuffer(inputData)
	var output bytes.Buffer
	server, err := New(input, &output)
	require.NoError(t, err)

	code, err := server.Run()
	require.NoError(t, err)
	require.Zero(t, code)

	return protocolReadFrames(t, output.Bytes())
}

func protocolFrames(t testing.TB, messages ...map[string]any) []byte {
	t.Helper()

	var input bytes.Buffer
	for _, msg := range messages {
		input.Write(protocolFrame(t, msg))
	}
	return input.Bytes()
}

func protocolFrame(t testing.TB, msg map[string]any) []byte {
	t.Helper()

	body, err := json.Marshal(msg)
	require.NoError(t, err)

	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func protocolReadFrames(t testing.TB, data []byte) []protocolMessage {
	t.Helper()

	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))
	var messages []protocolMessage
	for {
		header, err := reader.ReadMIMEHeader()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		length, err := strconv.Atoi(header.Get("Content-Length"))
		require.NoError(t, err)

		body := make([]byte, length)
		_, err = io.ReadFull(reader.R, body)
		require.NoError(t, err)

		var msg protocolMessage
		require.NoError(t, json.Unmarshal(body, &msg), string(body))
		require.Equal(t, "2.0", msg.JsonRpc, string(body))
		messages = append(messages, msg)
	}
	return messages
}

func protocolRequest(id string, method string, params any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
}

func protocolNotification(method string, params any) map[string]any {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	return msg
}

func didOpenNotification(uri string, source string) map[string]any {
	return protocolNotification("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": 1,
			"text":    source,
		},
	})
}

func didChangeNotification(uri string, source string) map[string]any {
	return protocolNotification("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": 2,
		},
		"contentChanges": []any{map[string]any{"text": source}},
	})
}

func initializeParams(options map[string]any) map[string]any {
	params := map[string]any{
		"processId": 1,
		"clientInfo": map[string]any{
			"name":    "protocol-test",
			"version": "1",
		},
		"capabilities": map[string]any{},
	}
	if options != nil {
		params["initializationOptions"] = options
	}
	return params
}

func protocolRange(startLine int, startCharacter int, endLine int, endCharacter int) map[string]any {
	return map[string]any{
		"start": map[string]any{"line": startLine, "character": startCharacter},
		"end":   map[string]any{"line": endLine, "character": endCharacter},
	}
}

func protocolResponseByID(t testing.TB, messages []protocolMessage, id string) protocolMessage {
	t.Helper()

	for _, msg := range messages {
		if msg.Method == "" && protocolMessageID(t, msg) == id {
			return msg
		}
	}
	require.Failf(t, "response not found", "id %q not found in %d messages", id, len(messages))
	return protocolMessage{}
}

func protocolRequestsByMethod(messages []protocolMessage, method string) []protocolMessage {
	var requests []protocolMessage
	for _, msg := range messages {
		if msg.Method == method {
			requests = append(requests, msg)
		}
	}
	return requests
}

func protocolMessageID(t testing.TB, msg protocolMessage) string {
	t.Helper()

	var id string
	require.NoError(t, json.Unmarshal(msg.ID, &id))
	return id
}

func protocolResultAs[T any](t testing.TB, msg protocolMessage) T {
	t.Helper()

	require.Nil(t, msg.Error)
	require.NotEmpty(t, msg.Result, "response %s has no result", protocolMessageID(t, msg))

	var result T
	require.NoError(t, json.Unmarshal(msg.Result, &result), string(msg.Result))
	return result
}

func protocolAssertSuccess(t testing.TB, msg protocolMessage) {
	t.Helper()

	require.Nil(t, msg.Error)
	assert.NotEmpty(t, msg.ID)
}

func protocolAssertNoResponseErrors(t testing.TB, messages []protocolMessage) {
	t.Helper()

	for _, msg := range messages {
		if msg.Method == "" {
			require.Nil(t, msg.Error, "response %s failed: %+v", protocolMessageID(t, msg), msg.Error)
		}
	}
}

func protocolCompletionLabels(items []lspCompletionItem) map[string]bool {
	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	return labels
}

func protocolCodeActionTitles(actions []lspCodeAction) map[string]bool {
	titles := make(map[string]bool)
	for _, action := range actions {
		titles[action.Title] = true
	}
	return titles
}

func protocolCodeLensTitleContains(lenses []LspCodeLens, text string) bool {
	for _, lens := range lenses {
		if lens.Command != nil && strings.Contains(lens.Command.Title, text) {
			return true
		}
	}
	return false
}

func protocolInlayLabels(inlays []LspInlayHint) map[string]bool {
	labels := make(map[string]bool)
	for _, inlay := range inlays {
		labels[inlay.Label] = true
	}
	return labels
}

func protocolInlayLabelHasPrefix(inlays []LspInlayHint, prefix string) bool {
	for _, inlay := range inlays {
		if strings.HasPrefix(inlay.Label, prefix) {
			return true
		}
	}
	return false
}

func protocolTestedWorkspaceCommands() []string {
	return []string{
		"teal.sourcemap.generate",
		"teal.decompile",
		"teal.pc.resolve",
		"teal.version.update",
		"teal.value.replace",
		"teal.call.remove",
		"teal.label.remove",
		"teal.label.create",
	}
}

func protocolCommandSet(commands []string) map[string]bool {
	set := make(map[string]bool)
	for _, command := range commands {
		set[command] = true
	}
	return set
}
