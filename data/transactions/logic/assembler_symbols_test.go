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

func TestSourceIdentifierQueriesForTools(t *testing.T) {
	result := AnalyzeSourceForTools(`#pragma version 8
#define VALUE 1
start:
b start
int VALUE`)
	idx := result.Index

	define, ok := SourceSymbolAtForTools(idx, 1, len("#define "))
	require.True(t, ok)
	require.Equal(t, SourceSymbolDefine, define.Kind)
	require.Equal(t, "VALUE", define.Name)

	label, ok := SourceSymbolAtForTools(idx, 2, 1)
	require.True(t, ok)
	require.Equal(t, SourceSymbolLabel, label.Kind)
	require.Equal(t, "start", label.Name)

	_, ok = SourceSymbolAtForTools(idx, 0, 0)
	require.False(t, ok)

	labelRef, ok := SourceReferenceAtForTools(idx, 3, len("b "))
	require.True(t, ok)
	require.Equal(t, SourceReferenceLabel, labelRef.Kind)
	require.Equal(t, "start", labelRef.Name)

	defineRef, ok := SourceReferenceAtForTools(idx, 4, len("int "))
	require.True(t, ok)
	require.Equal(t, SourceReferenceDefine, defineRef.Kind)
	require.Equal(t, "VALUE", defineRef.Name)

	identifier, ok := SourceIdentifierAtForTools(idx, 1, len("#define "))
	require.True(t, ok)
	require.Equal(t, SourceIdentifierSymbol, identifier.Kind)
	require.Equal(t, "VALUE", identifier.Name)
	require.NotNil(t, identifier.Symbol)
	require.Nil(t, identifier.Reference)

	identifier, ok = SourceIdentifierAtForTools(idx, 3, len("b "))
	require.True(t, ok)
	require.Equal(t, SourceIdentifierReference, identifier.Kind)
	require.Equal(t, "start", identifier.Name)
	require.Nil(t, identifier.Symbol)
	require.NotNil(t, identifier.Reference)

	require.Equal(t, []SourceSymbol{define}, SourceSymbolsByNameForTools(idx, "VALUE"))
	require.Equal(t, []SourceReference{defineRef}, SourceReferencesByNameForTools(idx, "VALUE"))
	require.Equal(t, []string{"VALUE"}, SourceDefinedNamesForTools(idx))
}

func TestSourceSymbolRangesForTools(t *testing.T) {
	name := "👍"
	result := AnalyzeSourceForTools(name + ":\nb " + name)
	require.Len(t, result.Index.Symbols, 1)
	require.Len(t, result.Index.References, 1)

	symbol := result.Index.Symbols[0]
	require.Equal(t, SourceRange{
		Line:      0,
		Column:    0,
		EndLine:   0,
		EndColumn: len(name),
	}, SourceSymbolNameRangeForTools(symbol))
	require.Equal(t, SourceRange{
		Line:      0,
		Column:    0,
		EndLine:   0,
		EndColumn: len(name + ":"),
	}, SourceSymbolRangeForTools(symbol))

	ref := result.Index.References[0]
	require.Equal(t, SourceRange{
		Line:      1,
		Column:    len("b "),
		EndLine:   1,
		EndColumn: len("b ") + len(name),
	}, SourceReferenceRangeForTools(ref))
}
