package lsp

import "testing"

func BenchmarkProtocol(b *testing.B) {
	benchmarks := []struct {
		name     string
		messages []map[string]any
	}{
		{
			name: "diagnostic",
			messages: []map[string]any{
				protocolRequest("init", "initialize", initializeParams(nil)),
				didOpenNotification(protocolTestURI, protocolTestSource),
				protocolRequest("diagnostic", "textDocument/diagnostic", map[string]any{
					"textDocument": map[string]any{"uri": protocolTestURI},
				}),
				protocolRequest("shutdown", "shutdown", nil),
				protocolNotification("exit", nil),
			},
		},
		{
			name: "completion_opcode",
			messages: []map[string]any{
				protocolRequest("init", "initialize", initializeParams(nil)),
				didOpenNotification(protocolTestURI, protocolTestSource),
				protocolRequest("completion", "textDocument/completion", map[string]any{
					"textDocument": map[string]any{"uri": protocolTestURI},
					"position":     map[string]any{"line": 7, "character": 2},
				}),
				protocolRequest("shutdown", "shutdown", nil),
				protocolNotification("exit", nil),
			},
		},
		{
			name: "document_symbols",
			messages: []map[string]any{
				protocolRequest("init", "initialize", initializeParams(nil)),
				didOpenNotification(protocolTestURI, protocolTestSource),
				protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
					"textDocument": map[string]any{"uri": protocolTestURI},
				}),
				protocolRequest("shutdown", "shutdown", nil),
				protocolNotification("exit", nil),
			},
		},
		{
			name: "semantic_tokens",
			messages: []map[string]any{
				protocolRequest("init", "initialize", initializeParams(map[string]any{
					"semanticTokens": true,
				})),
				didOpenNotification(protocolTestURI, protocolTestSource),
				protocolRequest("tokens", "textDocument/semanticTokens/full", map[string]any{
					"textDocument": map[string]any{"uri": protocolTestURI},
				}),
				protocolRequest("shutdown", "shutdown", nil),
				protocolNotification("exit", nil),
			},
		},
		{
			name:     "text_document_batch",
			messages: benchmarkTextDocumentBatchMessages(),
		},
		{
			name:     "workspace_commands",
			messages: benchmarkWorkspaceCommandMessages(),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			input := protocolFrames(b, bm.messages...)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frames := runProtocolInput(b, input)
				protocolAssertNoResponseErrors(b, frames)
			}
		})
	}
}

func benchmarkTextDocumentBatchMessages() []map[string]any {
	return []map[string]any{
		protocolRequest("init", "initialize", initializeParams(map[string]any{
			"semanticTokens": true,
		})),
		didOpenNotification(protocolTestURI, protocolTestSource),
		protocolRequest("diagnostic", "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("completion-op", "textDocument/completion", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 7, "character": 2},
		}),
		protocolRequest("completion-arg", "textDocument/completion", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": len("  txn ")},
		}),
		protocolRequest("hover", "textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": 6},
		}),
		protocolRequest("signature", "textDocument/signatureHelp", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 3, "character": len("  txn ")},
		}),
		protocolRequest("definition", "textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 7, "character": len("  int VA")},
		}),
		protocolRequest("prepare-rename", "textDocument/prepareRename", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 7, "character": len("  int VA")},
		}),
		protocolRequest("rename", "textDocument/rename", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 7, "character": len("  int VA")},
			"newName":      "RENAMED",
		}),
		protocolRequest("highlight", "textDocument/documentHighlight", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"position":     map[string]any{"line": 7, "character": len("  int VA")},
		}),
		protocolRequest("symbols", "textDocument/documentSymbol", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("tokens", "textDocument/semanticTokens/full", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("actions", "textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": protocolTestURI},
			"range":        protocolRange(6, 0, 6, len("unused:")),
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
	}
}

func benchmarkWorkspaceCommandMessages() []map[string]any {
	return []map[string]any{
		protocolRequest("init", "initialize", initializeParams(map[string]any{"programSize": false})),
		didOpenNotification(protocolTestURI, protocolTestSource),
		protocolRequest("sourcemap", "workspace/executeCommand", map[string]any{
			"command":   "teal.sourcemap.generate",
			"arguments": map[string]any{"uri": protocolTestURI},
		}),
		protocolRequest("decompile", "workspace/executeCommand", map[string]any{
			"command":   "teal.decompile",
			"arguments": map[string]any{"uri": protocolTestURI},
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
	}
}
