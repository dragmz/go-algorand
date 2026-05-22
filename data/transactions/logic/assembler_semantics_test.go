// Copyright (C) 2019-2026 Algorand Foundation Ltd.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceSemanticTokensForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions(`#pragma version 8
#pragma typetrack true
label:
b label
txn Sender
int 1 // comment
byte base64`, SourceToolOptions{Mode: ModeApp})

	tokens := SourceSemanticTokensForTools(result)
	requireSemanticToken(t, tokens, SourceSemanticMacro, 0, 0, len("#pragma"))
	requireSemanticToken(t, tokens, SourceSemanticBool, 1, len("#pragma typetrack "), len("#pragma typetrack true"))
	requireSemanticToken(t, tokens, SourceSemanticSymbol, 2, 0, len("label:"))
	requireSemanticToken(t, tokens, SourceSemanticOpcode, 3, 0, len("b"))
	requireSemanticToken(t, tokens, SourceSemanticReference, 3, len("b "), len("b label"))
	requireSemanticToken(t, tokens, SourceSemanticString, 4, len("txn "), len("txn Sender"))
	requireSemanticToken(t, tokens, SourceSemanticNumber, 5, len("int "), len("int 1"))
	requireSemanticToken(t, tokens, SourceSemanticComment, 5, len("int 1 "), len("int 1 // comment"))
	requireSemanticToken(t, tokens, SourceSemanticKeyword, 6, len("byte "), len("byte base64"))
}

func TestSourceSemanticTokensForToolsEmojiRanges(t *testing.T) {
	name := "👍"
	result := AnalyzeSourceForToolsWithOptions(name+":\nb "+name, SourceToolOptions{Mode: ModeApp})

	tokens := SourceSemanticTokensForTools(result)
	requireSemanticToken(t, tokens, SourceSemanticSymbol, 0, 0, len(name+":"))
	requireSemanticToken(t, tokens, SourceSemanticReference, 1, len("b "), len("b ")+len(name))
}

func requireSemanticToken(t *testing.T, tokens []SourceSemanticToken, kind SourceSemanticTokenKind, line int, column int, endColumn int) {
	t.Helper()
	for _, token := range tokens {
		if token.Kind == kind && token.Range.Line == line && token.Range.Column == column && token.Range.EndColumn == endColumn {
			return
		}
	}
	require.Failf(t, "semantic token not found", "kind=%d line=%d column=%d endColumn=%d tokens=%v", kind, line, column, endColumn, tokens)
}
