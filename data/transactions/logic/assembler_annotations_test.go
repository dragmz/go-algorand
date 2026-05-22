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

func TestSourceCodeLensesForToolsReferenceCounts(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("b a\na:\nunused:", SourceToolOptions{Mode: ModeApp})

	lenses := SourceCodeLensesForTools(result)
	refLens, ok := sourceCodeLensByKindForTest(lenses, SourceCodeLensReferenceCount)
	require.True(t, ok)
	require.Equal(t, 1, refLens.ReferenceCount)
	require.Equal(t, SourceRange{Line: 1, EndLine: 1}, refLens.Range)

	for _, lens := range lenses {
		require.False(t, lens.Kind == SourceCodeLensReferenceCount && lens.Range.Line == 2)
	}
}

func TestSourceCodeLensesForToolsProgramCounters(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("int 1\nint 2", SourceToolOptions{Mode: ModeApp})

	lenses := sourceCodeLensesByKindForTest(SourceCodeLensesForTools(result), SourceCodeLensProgramCounter)
	require.NotEmpty(t, lenses)
	require.Equal(t, sourceProgramCounters(result)[0], lenses[0].ProgramCounter)
	require.Equal(t, SourceRange{}, lenses[0].Range)
	requireProgramCountersSorted(t, sourceCodeLensPCsForTest(lenses))
}

func TestSourceInlaysForToolsNamedDecodedAndProgramCounters(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn 0\nbyte 0x3031\nint 1", SourceToolOptions{Mode: ModeApp})

	inlays := SourceInlaysForTools(result)

	named, ok := sourceInlayByKindForTest(inlays, SourceInlayNamedValue)
	require.True(t, ok)
	require.Equal(t, "Sender", named.Label)
	require.Equal(t, SourcePosition{Line: 0, Column: len("txn 0")}, named.Position)
	require.Equal(t, SourceRange{Line: 0, Column: len("txn "), EndLine: 0, EndColumn: len("txn 0")}, named.Range)

	decoded, ok := sourceInlayByKindForTest(inlays, SourceInlayDecodedValue)
	require.True(t, ok)
	require.Equal(t, "01", decoded.Label)
	require.Equal(t, SourcePosition{Line: 1, Column: len("byte 0x3031")}, decoded.Position)
	require.Equal(t, SourceRange{Line: 1, Column: len("byte "), EndLine: 1, EndColumn: len("byte 0x3031")}, decoded.Range)

	pcInlays := sourceInlaysByKindForTest(inlays, SourceInlayProgramCounter)
	require.NotEmpty(t, pcInlays)
	require.Equal(t, sourceProgramCounters(result)[0], pcInlays[0].ProgramCounter)
	require.Equal(t, SourcePosition{}, pcInlays[0].Position)
	require.Equal(t, SourceRange{}, pcInlays[0].Range)
	requireProgramCountersSorted(t, sourceInlayPCsForTest(pcInlays))
}

func sourceCodeLensByKindForTest(lenses []SourceCodeLens, kind SourceCodeLensKind) (SourceCodeLens, bool) {
	for _, lens := range lenses {
		if lens.Kind == kind {
			return lens, true
		}
	}
	return SourceCodeLens{}, false
}

func sourceCodeLensesByKindForTest(lenses []SourceCodeLens, kind SourceCodeLensKind) []SourceCodeLens {
	var found []SourceCodeLens
	for _, lens := range lenses {
		if lens.Kind == kind {
			found = append(found, lens)
		}
	}
	return found
}

func sourceInlayByKindForTest(inlays []SourceInlay, kind SourceInlayKind) (SourceInlay, bool) {
	for _, inlay := range inlays {
		if inlay.Kind == kind {
			return inlay, true
		}
	}
	return SourceInlay{}, false
}

func sourceInlaysByKindForTest(inlays []SourceInlay, kind SourceInlayKind) []SourceInlay {
	var found []SourceInlay
	for _, inlay := range inlays {
		if inlay.Kind == kind {
			found = append(found, inlay)
		}
	}
	return found
}

func sourceCodeLensPCsForTest(lenses []SourceCodeLens) []int {
	var pcs []int
	for _, lens := range lenses {
		pcs = append(pcs, lens.ProgramCounter)
	}
	return pcs
}

func sourceInlayPCsForTest(inlays []SourceInlay) []int {
	var pcs []int
	for _, inlay := range inlays {
		pcs = append(pcs, inlay.ProgramCounter)
	}
	return pcs
}

func requireProgramCountersSorted(t *testing.T, pcs []int) {
	t.Helper()
	for i := 1; i < len(pcs); i++ {
		require.LessOrEqual(t, pcs[i-1], pcs[i])
	}
}
