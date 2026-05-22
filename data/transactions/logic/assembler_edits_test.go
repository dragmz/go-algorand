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

func TestSourceEditRemoveStatementForTools(t *testing.T) {
	lines := SourceLinesForTools("int 1; byte \"😀\"; int 2")

	edit, ok := SourceEditRemoveStatementForTools(lines, 0, 1)
	require.True(t, ok)
	require.Equal(t, SourceEdit{
		Line:      0,
		Column:    len("int 1; "),
		EndLine:   0,
		EndColumn: len("int 1; byte \"😀\""),
	}, edit)
}

func TestSourceEditCreateLabelForTools(t *testing.T) {
	edit := SourceEditCreateLabelForTools(SourceLinesForTools("int 1"), "missing")
	require.Equal(t, SourceEdit{
		Line:      1,
		Column:    0,
		EndLine:   1,
		EndColumn: 0,
		NewText:   "\r\nmissing:\r\n",
	}, edit)
}

func TestSourceEditSymbolForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("a:\nb a", SourceToolOptions{Mode: ModeApp})

	rename := SourceEditsRenameSymbolForTools(result.Index, "a", "b")
	require.Equal(t, []SourceEdit{
		{Line: 0, Column: 0, EndLine: 0, EndColumn: 1, NewText: "b"},
		{Line: 1, Column: 2, EndLine: 1, EndColumn: 3, NewText: "b"},
	}, rename)

	remove := SourceEditsRemoveSymbolForTools(result.Index, "a")
	require.Equal(t, []SourceEdit{
		{Line: 0, Column: 0, EndLine: 0, EndColumn: 2},
	}, remove)
}

func TestSourceEditUpdateVersionForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("#pragma version 8", SourceToolOptions{Mode: ModeApp})
	require.Equal(t, SourceEdit{
		Line:      0,
		Column:    len("#pragma version "),
		EndLine:   0,
		EndColumn: len("#pragma version 8"),
		NewText:   "9",
	}, SourceEditUpdateVersionForTools(result.Index, 9))

	result = AnalyzeSourceForToolsWithOptions("int 1", SourceToolOptions{Mode: ModeApp})
	require.Equal(t, SourceEdit{
		Line:      0,
		Column:    0,
		EndLine:   0,
		EndColumn: 0,
		NewText:   "#pragma version 8\r\n",
	}, SourceEditUpdateVersionForTools(result.Index, 8))
}

func TestSourceActionsForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions(`#pragma version 7
a:
b a
unused:
b missing
box_create
txn 0
byte 0x3031`, SourceToolOptions{Mode: ModeApp})

	actions := SourceActionsForTools(result.Lines, result.Index, result.Program, SourceRange{
		Line:      0,
		Column:    0,
		EndLine:   10,
		EndColumn: 0,
	})
	names := map[string]bool{}
	for _, action := range actions {
		names[action.Title] = true
		require.NotEmpty(t, action.Edits, action.Title)
	}
	require.True(t, names["Remove label 'unused'"])
	require.True(t, names["Create label 'missing'"])
	require.True(t, names["Update version to 8"])
	require.True(t, names["Replace with 'Sender'"])
	require.True(t, names["Replace with literal '01'"])
}
