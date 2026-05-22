package lsp

import (
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func TestSemantics(t *testing.T) {
	ts := SemanticTokens{
		SemanticToken{Line: 0, Index: 0, Length: 1, Type: 4, Modifiers: 0},
		SemanticToken{Line: 3, Index: 19, Length: 10, Type: 2, Modifiers: 0},
		SemanticToken{Line: 4, Index: 7, Length: 4, Type: 2, Modifiers: 0},
		SemanticToken{Line: 8, Index: 7, Length: 4, Type: 2, Modifiers: 0},
		SemanticToken{Line: 6, Index: 4, Length: 6, Type: 1, Modifiers: 0},
	}

	res := ts.Encode()

	if len(res) != len(ts)*5 {
		t.Error("Unexpected length:", len(ts))
	}
}

func TestSourceSemanticTokenTypeToLSP(t *testing.T) {
	tests := map[logic.SourceSemanticTokenKind]int{
		logic.SourceSemanticOpcode:    semanticTokenKeyword,
		logic.SourceSemanticMacro:     semanticTokenMacro,
		logic.SourceSemanticBool:      semanticTokenValue,
		logic.SourceSemanticNumber:    semanticTokenNumber,
		logic.SourceSemanticString:    semanticTokenString,
		logic.SourceSemanticKeyword:   semanticTokenKeyword,
		logic.SourceSemanticComment:   semanticTokenComment,
		logic.SourceSemanticSymbol:    semanticTokenMethod,
		logic.SourceSemanticReference: semanticTokenString,
	}

	for kind, expected := range tests {
		if got := sourceSemanticTokenTypeToLSP(kind); got != expected {
			t.Errorf("kind %d mapped to %d, expected %d", kind, got, expected)
		}
	}
}
