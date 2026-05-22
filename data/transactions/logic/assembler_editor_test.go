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

	hover, ok = SourceHoverForTools(result, 0, len("txn "))
	require.True(t, ok)
	require.Contains(t, hover.Text, "Sender =")
	require.Contains(t, hover.Text, "32 byte address")
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
