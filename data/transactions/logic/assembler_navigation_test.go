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

func TestSourcePrepareRenameForTools(t *testing.T) {
	result := AnalyzeSourceForTools("a:\nb a")

	rename, ok := SourcePrepareRenameForTools(result, 0, 0)
	require.True(t, ok)
	require.Equal(t, SourcePrepareRename{
		Range:       SourceRange{Line: 0, EndLine: 0, EndColumn: 1},
		Placeholder: "a",
	}, rename)

	rename, ok = SourcePrepareRenameForTools(result, 1, len("b "))
	require.True(t, ok)
	require.Equal(t, SourcePrepareRename{
		Range:       SourceRange{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a")},
		Placeholder: "a",
	}, rename)

	_, ok = SourcePrepareRenameForTools(result, 1, 0)
	require.False(t, ok)
}

func TestSourceRenameEditsForTools(t *testing.T) {
	result := AnalyzeSourceForTools("a:\nb a")

	edits := SourceRenameEditsForTools(result, 1, len("b "), "next")
	require.Equal(t, []SourceEdit{
		{Line: 0, EndLine: 0, EndColumn: 1, NewText: "next"},
		{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a"), NewText: "next"},
	}, edits)
}

func TestSourceDefinitionsForTools(t *testing.T) {
	result := AnalyzeSourceForTools("a:\nb a")

	ranges := SourceDefinitionsForTools(result, 1, len("b "))
	require.Equal(t, []SourceRange{{Line: 0, EndLine: 0, EndColumn: 1}}, ranges)

	require.Empty(t, SourceDefinitionsForTools(result, 0, 0))
}

func TestSourceHighlightsForTools(t *testing.T) {
	result := AnalyzeSourceForTools("a:\nb a")

	highlights := SourceHighlightsForTools(result, 0, 0)
	require.Equal(t, []SourceHighlight{
		{Range: SourceRange{Line: 0, EndLine: 0, EndColumn: 1}},
		{Range: SourceRange{Line: 1, Column: len("b "), EndLine: 1, EndColumn: len("b a")}},
	}, highlights)
}

func TestSourceDocumentSymbolsForTools(t *testing.T) {
	result := AnalyzeSourceForTools("a:\nb a")

	symbols := SourceDocumentSymbolsForTools(result)
	require.Equal(t, []SourceDocumentSymbol{{
		Name:           "a",
		Range:          SourceRange{Line: 0, EndLine: 0, EndColumn: len("a:")},
		SelectionRange: SourceRange{Line: 0, EndLine: 0, EndColumn: len("a")},
	}}, symbols)
}
