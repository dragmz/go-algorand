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

func TestSourceCompletionsForToolsOpcodeVersionModeAndPrefix(t *testing.T) {
	defaultApp := AnalyzeSourceForToolsWithOptions("", SourceToolOptions{Mode: ModeApp})
	defaultNames := sourceCompletionLabelsForTest(SourceCompletionsForTools(defaultApp, 0, 0))
	require.Contains(t, defaultNames, "err")
	require.NotContains(t, defaultNames, "addw")
	require.NotContains(t, defaultNames, "args")

	v2App := AnalyzeSourceForToolsWithOptions("#pragma version 2", SourceToolOptions{Mode: ModeApp})
	v2Names := sourceCompletionLabelsForTest(SourceCompletionsForTools(v2App, 1, 0))
	require.Contains(t, v2Names, "addw")

	v5Sig := AnalyzeSourceForToolsWithOptions("#pragma version 5", SourceToolOptions{Mode: ModeSig})
	v5SigNames := sourceCompletionLabelsForTest(SourceCompletionsForTools(v5Sig, 1, 0))
	require.Contains(t, v5SigNames, "args")

	v4Sig := AnalyzeSourceForToolsWithOptions("#pragma version 4", SourceToolOptions{Mode: ModeSig})
	v4SigNames := sourceCompletionLabelsForTest(SourceCompletionsForTools(v4Sig, 1, 0))
	require.NotContains(t, v4SigNames, "args")

	filtered := AnalyzeSourceForToolsWithOptions("tx", SourceToolOptions{Mode: ModeApp})
	filteredItems := SourceCompletionsForTools(filtered, 0, len("tx"))
	filteredNames := sourceCompletionLabelsForTest(filteredItems)
	require.Contains(t, filteredNames, "txn")
	require.NotContains(t, filteredNames, "int")
	require.NotContains(t, filteredNames, "soc")
	require.NotContains(t, filteredNames, "func")
}

func TestSourceCompletionsForToolsDefinesAndArguments(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions(`#pragma version 8
#define VALUE 1
txn `, SourceToolOptions{Mode: ModeApp})

	opItems := SourceCompletionsForTools(result, 2, 0)
	opNames := sourceCompletionLabelsForTest(opItems)
	require.Contains(t, opNames, "VALUE")

	argItems := SourceCompletionsForTools(result, 2, len("txn "))
	argNames := sourceCompletionLabelsForTest(argItems)
	require.Contains(t, argNames, "Sender")

	sender := sourceCompletionItemByLabelForTest(argItems, "Sender")
	require.Equal(t, SourceCompletionItemArgument, sender.Kind)
	require.True(t, sender.HasValue)
	require.NotEmpty(t, sender.Docs)
}

func TestSourceCompletionsForToolsLabels(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("#pragma version 8\n// docs\nsub:\nproto 2 1\nb ", SourceToolOptions{Mode: ModeApp})

	items := SourceCompletionsForTools(result, 4, len("b "))
	sub := sourceCompletionItemByLabelForTest(items, "sub")
	require.Equal(t, SourceCompletionItemArgument, sub.Kind)
	require.False(t, sub.HasValue)
	require.Equal(t, "in: 2, out: 1", sub.Signature)
	require.Equal(t, "docs", sub.Docs)
}

func TestSourceHoverForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn Sender", SourceToolOptions{Mode: ModeApp})

	hover, ok := SourceHoverForTools(result, 0, 1)
	require.True(t, ok)
	require.Contains(t, hover.Text, "transaction")
	require.Equal(t, SourceRange{Line: 0, Column: 0, EndLine: 0, EndColumn: len("txn")}, hover.Range)

	hover, ok = SourceHoverForTools(result, 0, len("txn "))
	require.True(t, ok)
	require.Contains(t, hover.Text, "`Sender` = ")
	require.Contains(t, hover.Text, "32 byte address")
	// The value and its documentation are separate sections, so markdown must
	// not run them onto one line.
	require.Contains(t, hover.Text, "\r\n\r\n")
	require.Equal(t, SourceRange{Line: 0, Column: len("txn "), EndLine: 0, EndColumn: len("txn Sender")}, hover.Range)

	_, ok = SourceHoverForTools(result, 0, len("txn Sender")+4)
	require.False(t, ok)
}

func TestSourceHoverForToolsLabel(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("#pragma version 8\n// docs\nsub:\nproto 2 1\nretsub\nb sub", SourceToolOptions{Mode: ModeApp})

	definition, ok := SourceHoverForTools(result, 2, 0)
	require.True(t, ok)
	require.Contains(t, definition.Text, "`sub:` in: 2, out: 1")
	require.Contains(t, definition.Text, "docs")
	require.Contains(t, definition.Text, "1 reference")
	require.Equal(t, SourceRange{Line: 2, Column: 0, EndLine: 2, EndColumn: len("sub")}, definition.Range)

	// A reference describes the symbol it names, but reports its own range.
	reference, ok := SourceHoverForTools(result, 5, len("b "))
	require.True(t, ok)
	require.Equal(t, definition.Text, reference.Text)
	require.Equal(t, SourceRange{Line: 5, Column: len("b "), EndLine: 5, EndColumn: len("b sub")}, reference.Range)

	// The mnemonic of a branch documents the opcode, not the label it names.
	mnemonic, ok := SourceHoverForTools(result, 5, 0)
	require.True(t, ok)
	require.NotContains(t, mnemonic.Text, "reference")
	require.Equal(t, SourceRange{Line: 5, Column: 0, EndLine: 5, EndColumn: len("b")}, mnemonic.Range)
}

func TestSourceHoverForToolsDefine(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("#pragma version 8\n// value docs\n#define VALUE 1\nint VALUE", SourceToolOptions{Mode: ModeApp})

	hover, ok := SourceHoverForTools(result, 2, len("#define "))
	require.True(t, ok)
	require.Contains(t, hover.Text, "`#define VALUE`")
	require.Contains(t, hover.Text, "value docs")
	require.Contains(t, hover.Text, "1 reference")

	reference, ok := SourceHoverForTools(result, 3, len("int "))
	require.True(t, ok)
	require.Equal(t, hover.Text, reference.Text)
}

func TestSourceHoverForToolsUnreferencedSymbolOmitsCount(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("#pragma version 8\nsub:\nretsub", SourceToolOptions{Mode: ModeApp})

	hover, ok := SourceHoverForTools(result, 1, 0)
	require.True(t, ok)
	require.Equal(t, "`sub:`", hover.Text)
}

func TestSourceSelectionRangesForTools(t *testing.T) {
	lines := SourceLinesForTools("  txn Sender\n")

	// Token, then the statement holding it, then the whole line with its indent.
	ranges := SourceSelectionRangesForTools(lines, 0, len("  txn Se"))
	require.Equal(t, []SourceRange{
		{Line: 0, Column: len("  txn "), EndLine: 0, EndColumn: len("  txn Sender")},
		{Line: 0, Column: len("  "), EndLine: 0, EndColumn: len("  txn Sender")},
		{Line: 0, Column: 0, EndLine: 0, EndColumn: len("  txn Sender")},
	}, ranges)

	// Every rung contains the one before it, which is what the protocol requires
	// of the parent chain.
	for i := 1; i < len(ranges); i++ {
		require.True(t, sourceRangeWithin(ranges[i-1], ranges[i]), "rung %d escapes its parent", i-1)
	}
}

func TestSourceSelectionRangesForToolsSkipsRungsThatDoNotWiden(t *testing.T) {
	// One token filling the whole line collapses the ladder to a single rung.
	lines := SourceLinesForTools("retsub")
	require.Equal(t, []SourceRange{
		{Line: 0, Column: 0, EndLine: 0, EndColumn: len("retsub")},
	}, SourceSelectionRangesForTools(lines, 0, 0))

	// A statement is only a rung when it holds the token, so a cursor in a
	// trailing comment widens straight to the line.
	lines = SourceLinesForTools("int 1 // note")
	require.Equal(t, []SourceRange{
		{Line: 0, Column: len("int 1 "), EndLine: 0, EndColumn: len("int 1 // note")},
		{Line: 0, Column: 0, EndLine: 0, EndColumn: len("int 1 // note")},
	}, SourceSelectionRangesForTools(lines, 0, len("int 1 // no")))
}

func TestSourceSelectionRangesForToolsSemicolonStatements(t *testing.T) {
	lines := SourceLinesForTools("int 1; int 2")

	ranges := SourceSelectionRangesForTools(lines, 0, len("int 1; in"))
	require.Equal(t, []SourceRange{
		{Line: 0, Column: len("int 1; "), EndLine: 0, EndColumn: len("int 1; int")},
		{Line: 0, Column: len("int 1; "), EndLine: 0, EndColumn: len("int 1; int 2")},
		{Line: 0, Column: 0, EndLine: 0, EndColumn: len("int 1; int 2")},
	}, ranges)
}

func TestSourceSelectionRangesForToolsOutsideDocument(t *testing.T) {
	lines := SourceLinesForTools("int 1")

	require.Nil(t, SourceSelectionRangesForTools(lines, -1, 0))
	require.Nil(t, SourceSelectionRangesForTools(lines, 7, 0))

	// A blank line still widens to itself, so a position always has an answer.
	blank := SourceLinesForTools("\nint 1")
	require.Equal(t, []SourceRange{{Line: 0}}, SourceSelectionRangesForTools(blank, 0, 0))
}

func TestSourceMarkdownDoc(t *testing.T) {
	require.Equal(t, "", sourceMarkdownDoc())
	require.Equal(t, "", sourceMarkdownDoc("", ""))
	require.Equal(t, "only", sourceMarkdownDoc("", "only", ""))
	require.Equal(t, "first\r\n\r\nsecond", sourceMarkdownDoc("first", "", "second"))
}

func TestSourceSignatureHelpForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn Sender", SourceToolOptions{Mode: ModeApp})

	help, ok := SourceSignatureHelpForTools(result, 0, len("txn "))
	require.True(t, ok)
	require.Equal(t, "txn {transaction field index : f} [{uint8 : i}]", help.Label)
	require.NotEmpty(t, help.Docs)
	require.Equal(t, []string{"f", "i"}, help.Parameters)
	require.Equal(t, 0, help.ActiveParameter)
}

func sourceCompletionLabelsForTest(items []SourceCompletionItem) map[string]bool {
	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	return labels
}

func sourceCompletionItemByLabelForTest(items []SourceCompletionItem, label string) SourceCompletionItem {
	for _, item := range items {
		if item.Label == label {
			return item
		}
	}
	return SourceCompletionItem{}
}
