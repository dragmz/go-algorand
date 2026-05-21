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

func TestSourceIndexLabelsDefinesAndReferences(t *testing.T) {
	result := AnalyzeSourceForTools(`#pragma version 8
#define VALUE 123
b end
int VALUE
switch end missing
end:`)

	idx := result.Index
	require.Equal(t, uint64(8), idx.Version)
	require.NotNil(t, idx.VersionToken)
	require.Equal(t, "8", idx.VersionToken.Text)

	require.Len(t, idx.Symbols, 2)
	require.Equal(t, SourceSymbolDefine, idx.Symbols[0].Kind)
	require.Equal(t, "VALUE", idx.Symbols[0].Name)
	require.Equal(t, SourceSymbolLabel, idx.Symbols[1].Kind)
	require.Equal(t, "end", idx.Symbols[1].Name)

	require.Equal(t, 2, idx.RefCounts["end"])
	require.Equal(t, 1, idx.RefCounts["VALUE"])
	require.Equal(t, 1, idx.RefCounts["missing"])

	require.Len(t, idx.MissingReferences, 1)
	require.Equal(t, SourceReferenceLabel, idx.MissingReferences[0].Kind)
	require.Equal(t, "missing", idx.MissingReferences[0].Name)
}

func TestSourceIndexDefineInvocationReference(t *testing.T) {
	result := AnalyzeSourceForTools(`#pragma version 8
#define VALUE int 1
VALUE`)

	idx := result.Index
	require.Len(t, idx.Symbols, 1)
	require.Equal(t, "VALUE", idx.Symbols[0].Name)
	require.Len(t, idx.References, 1)
	require.Equal(t, SourceReferenceDefine, idx.References[0].Kind)
	require.Equal(t, "VALUE", idx.References[0].Name)
	require.Equal(t, 2, idx.References[0].Line)
}

func TestSourceIndexDefineLabelArgumentReference(t *testing.T) {
	result := AnalyzeSourceForTools(`#pragma version 8
#define TARGET end
b TARGET
end:`)

	idx := result.Index
	require.Len(t, idx.References, 1)
	require.Equal(t, SourceReferenceDefine, idx.References[0].Kind)
	require.Equal(t, "TARGET", idx.References[0].Name)
	require.Empty(t, idx.MissingReferences)
}

func TestSourceIndexDefineReferencesRequirePriorDefinition(t *testing.T) {
	result := AnalyzeSourceForTools(`#pragma version 8
VALUE
#define VALUE int 1`)

	require.Len(t, result.Index.Symbols, 1)
	require.Empty(t, result.Index.References)
	require.Empty(t, result.Index.MissingReferences)
}

func TestSourceIndexRedundants(t *testing.T) {
	result := AnalyzeSourceForTools("unused:\nb next\nnext:")

	var unused, branch bool
	for _, redundant := range result.Index.Redundants {
		switch redundant.Kind {
		case SourceRedundantUnusedLabel:
			unused = redundant.Name == "unused" && redundant.Line == 0
		case SourceRedundantBranchToNextLabel:
			branch = redundant.Name == "next" && redundant.Line == 1
		default:
		}
	}

	require.True(t, unused)
	require.True(t, branch)
}

func TestSourceIndexProtoSignature(t *testing.T) {
	result := AnalyzeSourceForTools("#pragma version 8\nsub:\nproto 2 1")

	require.Len(t, result.Index.Symbols, 1)
	require.Equal(t, "sub", result.Index.Symbols[0].Name)
	require.Equal(t, "in: 2, out: 1", result.Index.Symbols[0].Signature)
}

func TestSourceIndexByteColumns(t *testing.T) {
	result := AnalyzeSourceForTools("👍: b 👍")

	require.Len(t, result.Index.Symbols, 1)
	require.Equal(t, 0, result.Index.Symbols[0].Column)
	require.Equal(t, len("👍:"), result.Index.Symbols[0].EndColumn)

	require.Len(t, result.Index.References, 1)
	require.Equal(t, len("👍: b "), result.Index.References[0].Column)
	require.Equal(t, len("👍: b 👍"), result.Index.References[0].EndColumn)
}
