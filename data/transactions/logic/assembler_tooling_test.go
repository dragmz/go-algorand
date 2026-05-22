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

func TestToolOpcodesForToolsVersionAndMode(t *testing.T) {
	defaultApp := toolOpcodeNamesForTest(ToolOpcodesForTools(1, ModeApp))
	require.Contains(t, defaultApp, "err")
	require.NotContains(t, defaultApp, "addw")
	require.NotContains(t, defaultApp, "args")

	v2App := toolOpcodeNamesForTest(ToolOpcodesForTools(2, ModeApp))
	require.Contains(t, v2App, "addw")

	v4Sig := toolOpcodeNamesForTest(ToolOpcodesForTools(4, ModeSig))
	require.NotContains(t, v4Sig, "args")

	v5Sig := toolOpcodeNamesForTest(ToolOpcodesForTools(5, ModeSig))
	require.Contains(t, v5Sig, "args")
}

func TestToolOpcodeArgsForPseudoOps(t *testing.T) {
	op, ok := toolOpcodeForSource("txn", 1, ModeApp)
	require.True(t, ok)
	require.Equal(t, "txn", op.Name)
	require.Len(t, op.Args, 2)
	require.Equal(t, ToolFieldTxn, op.Args[0].FieldGroup)
	require.True(t, op.Args[1].Optional)
}

func TestToolArgValuesForTools(t *testing.T) {
	values := ToolArgValuesForTools(ToolArgConstInt, ToolFieldNone, 8, ModeApp)
	require.Contains(t, toolArgValueNamesForTest(values), "DeleteApplication")

	fields := ToolArgValuesForTools(ToolArgField, ToolFieldTxn, 8, ModeApp)
	require.Contains(t, toolArgValueNamesForTest(fields), "Sender")
}

func TestSourceProgramForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions(`#pragma version 7
#pragma typetrack true
box_create
txn Sender
byte 0x3031`, SourceToolOptions{Mode: ModeApp})

	require.Len(t, result.Program.RequiredVersions, 1)
	require.Equal(t, uint64(8), result.Program.RequiredVersions[0].Version)

	var opCount, macroCount, boolCount, stringCount int
	for _, class := range result.Program.TokenClasses {
		switch class.Kind {
		case SourceTokenClassOpcode:
			opCount++
		case SourceTokenClassMacro:
			macroCount++
		case SourceTokenClassBool:
			boolCount++
		case SourceTokenClassString:
			stringCount++
		default:
		}
	}
	require.GreaterOrEqual(t, opCount, 3)
	require.Equal(t, 4, macroCount)
	require.Equal(t, 1, boolCount)
	require.Equal(t, 2, stringCount)
}

func TestDecodedHexStringForTools(t *testing.T) {
	decoded, ok := DecodedHexStringForTools("0x3031")
	require.True(t, ok)
	require.Equal(t, "01", decoded)
}

func toolOpcodeNamesForTest(ops []ToolOpcode) map[string]bool {
	names := make(map[string]bool)
	for _, op := range ops {
		names[op.Name] = true
	}
	return names
}

func toolArgValueNamesForTest(values []ToolArgValue) map[string]bool {
	names := make(map[string]bool)
	for _, value := range values {
		names[value.Name] = true
	}
	return names
}
