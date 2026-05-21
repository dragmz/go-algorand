// Copyright (C) 2019-2025 Algorand, Inc.
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

	"github.com/algorand/go-algorand/test/partitiontest"
)

func sourceTokenTexts(tokens []SourceToken) []string {
	out := make([]string, len(tokens))
	for i, token := range tokens {
		out[i] = token.Text
	}
	return out
}

func sourceStatementTexts(statements []SourceStatement) [][]string {
	out := make([][]string, len(statements))
	for i, statement := range statements {
		out[i] = sourceTokenTexts(statement.Tokens)
	}
	return out
}

func TestSourceLinesForToolsTokenization(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	lines := SourceLinesForTools(`byte "foo bar // not a comment" // comment`)
	require.Len(t, lines, 1)
	require.Equal(t, []string{"byte", `"foo bar // not a comment"`}, sourceTokenTexts(lines[0].Tokens))
	require.NotNil(t, lines[0].Comment)
	require.Equal(t, " comment", lines[0].Comment.Text)
	require.Equal(t, SourceTokenComment, lines[0].Comment.Kind)

	lines = SourceLinesForTools(`byte base64(ABC//==) // comment`)
	require.Len(t, lines, 1)
	require.Equal(t, []string{"byte", "base64(ABC//==)"}, sourceTokenTexts(lines[0].Tokens))
	require.NotNil(t, lines[0].Comment)
	require.Equal(t, " comment", lines[0].Comment.Text)
}

func TestSourceLinesForToolsStatements(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	lines := SourceLinesForTools("int 1;;;int 2")
	require.Len(t, lines, 1)
	require.Equal(t, []string{"int", "1", ";", ";", ";", "int", "2"}, sourceTokenTexts(lines[0].Tokens))
	require.Equal(t, [][]string{
		{"int", "1"},
		{},
		{},
		{"int", "2"},
	}, sourceStatementTexts(lines[0].Statements))

	lines = SourceLinesForTools("int 1;")
	require.Len(t, lines, 1)
	require.Equal(t, [][]string{
		{"int", "1"},
		{},
	}, sourceStatementTexts(lines[0].Statements))
}

func TestSourceLinesForToolsRanges(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	lines := SourceLinesForTools("  int 10 // comment")
	require.Len(t, lines, 1)
	require.Equal(t, []SourceToken{
		{Text: "int", Kind: SourceTokenValue, Line: 0, Column: 2, EndColumn: 5},
		{Text: "10", Kind: SourceTokenValue, Line: 0, Column: 6, EndColumn: 8},
	}, lines[0].Tokens)
	require.NotNil(t, lines[0].Comment)
	require.Equal(t, SourceToken{Text: " comment", Kind: SourceTokenComment, Line: 0, Column: 9, EndColumn: 19}, *lines[0].Comment)
}

func TestSourceLinesForToolsLineSplitting(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	require.Empty(t, SourceLinesForTools(""))

	lines := SourceLinesForTools("int 1\r\nint 2\n")
	require.Len(t, lines, 2)
	require.Equal(t, "int 1", lines[0].Text)
	require.Equal(t, "int 2", lines[1].Text)

	lines = SourceLinesForTools("\n\n")
	require.Len(t, lines, 2)
	require.Empty(t, lines[0].Text)
	require.Empty(t, lines[1].Text)
}
