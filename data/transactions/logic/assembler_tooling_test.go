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
	require.Equal(t, &TxnFields, op.Args[0].FieldGroup)
	require.True(t, op.Args[1].Optional)
}

func TestToolArgValuesForTools(t *testing.T) {
	values := ToolArgValuesForTools(ToolArgConstInt, ToolFieldNone, 8, ModeApp)
	require.Contains(t, toolArgValueNamesForTest(values), "DeleteApplication")

	fields := ToolArgValuesForTools(ToolArgField, &TxnFields, 8, ModeApp)
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

func TestSourceStatementAtForTools(t *testing.T) {
	lines := SourceLinesForTools("int 1; int 2\n;;;")

	statement, idx, ok := SourceStatementAtForTools(lines, 0, 0)
	require.True(t, ok)
	require.Equal(t, 0, idx)
	require.Equal(t, "int", statement.Tokens[0].Text)

	statement, idx, ok = SourceStatementAtForTools(lines, 0, 8)
	require.True(t, ok)
	require.Equal(t, 1, idx)
	require.Equal(t, "int", statement.Tokens[0].Text)

	statement, idx, ok = SourceStatementAtForTools(lines, 1, 2)
	require.True(t, ok)
	require.Equal(t, 2, idx)
	require.Empty(t, statement.Tokens)

	lines = SourceLinesForTools("byte \"😀\"; int 2")
	statement, idx, ok = SourceStatementAtForTools(lines, 0, len("byte \"😀\"; "))
	require.True(t, ok)
	require.Equal(t, 1, idx)
	require.Equal(t, "int", statement.Tokens[0].Text)
}

func TestSourceCompletionContextForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn Sender; int ", SourceToolOptions{Mode: ModeApp})

	ctx := SourceCompletionContextForTools(result.Lines, result.Program, 0, 1)
	require.Equal(t, SourceCompletionOpcode, ctx.Mode)
	require.Equal(t, "txn", ctx.Prefix)

	ctx = SourceCompletionContextForTools(result.Lines, result.Program, 0, len("txn "))
	require.Equal(t, SourceCompletionArgument, ctx.Mode)

	ctx = SourceCompletionContextForTools(result.Lines, result.Program, 0, len("txn Sender; "))
	require.Equal(t, SourceCompletionOpcode, ctx.Mode)
	require.Equal(t, "int", ctx.Prefix)
}

func TestSourceToolArgAtForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn Sender", SourceToolOptions{Mode: ModeApp})

	arg, idx, ok := SourceToolArgAtForTools(result.Lines, result.Program, 0, len("txn S"))
	require.True(t, ok)
	require.Equal(t, 0, idx)
	require.Equal(t, ToolArgField, arg.Kind)
	require.Equal(t, &TxnFields, arg.FieldGroup)
}

func TestSourceInlayHintsForTools(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn 0\nbyte 0x3031", SourceToolOptions{Mode: ModeApp})

	hints := SourceInlayHintsForTools(result.Lines, result.Program, SourceAllLines)
	var named, decoded bool
	for _, hint := range hints {
		switch hint.Kind {
		case SourceInlayHintNamed:
			named = named || hint.Label == "Sender"
		case SourceInlayHintDecoded:
			decoded = decoded || hint.Label == "01"
		default:
		}
	}
	require.True(t, named)
	require.True(t, decoded)
}

func TestSourceInlayHintsForToolsLineRange(t *testing.T) {
	result := AnalyzeSourceForToolsWithOptions("txn 0\nbyte 0x3031", SourceToolOptions{Mode: ModeApp})

	// Both the operation walk and the leftover-token walk honour the range, so a
	// hint outside it is never built.
	first := SourceInlayHintsForTools(result.Lines, result.Program, SourceLineRange{Start: 0, End: 1})
	require.Len(t, first, 1)
	require.Equal(t, SourceInlayHintNamed, first[0].Kind)
	require.Equal(t, "Sender", first[0].Label)

	second := SourceInlayHintsForTools(result.Lines, result.Program, SourceLineRange{Start: 1, End: 2})
	require.Len(t, second, 1)
	require.Equal(t, SourceInlayHintDecoded, second[0].Kind)
	require.Equal(t, "01", second[0].Label)

	require.Empty(t, SourceInlayHintsForTools(result.Lines, result.Program, SourceLineRange{}))
}

// TestToolFieldGroupsCoverAllImmediates checks that every field immediate an
// opcode declares resolves to values an editor can offer, at the version the
// opcode was introduced and at every later version. A field group the tooling
// cannot resolve still reports as ToolArgField, but yields no completions and
// no documentation.
func TestToolFieldGroupsCoverAllImmediates(t *testing.T) {
	for _, spec := range OpSpecs {
		for _, imm := range spec.Immediates {
			if imm.Group == nil {
				continue
			}

			for version := spec.Version; version <= LogicVersion; version++ {
				for _, mode := range []RunMode{ModeApp, ModeSig} {
					if spec.Modes&mode == 0 {
						continue
					}
					values := toolFieldValues(imm.Group, version, mode)
					require.NotEmptyf(t, values,
						"field group %q used by %s (v%d) resolves to no values at v%d mode %d",
						imm.Group.Name, spec.Name, spec.Version, version, mode)
				}
			}
		}
	}
}

// TestToolOpcodeArgsResolveAllVersions checks the editor-visible argument
// surface of every mnemonic across every version and mode: a field argument
// must name a group, and that group must resolve to values.
func TestToolOpcodeArgsResolveAllVersions(t *testing.T) {
	for _, name := range toolOpcodeNames() {
		for _, mode := range []RunMode{ModeApp, ModeSig} {
			for argCount := 0; argCount <= 3; argCount++ {
				op, ok := ToolOpcodeForTools(name, argCount, mode)
				if !ok {
					continue
				}
				for i, arg := range op.Args {
					if arg.Kind != ToolArgField {
						continue
					}
					require.NotNilf(t, arg.FieldGroup,
						"%s arg %d (%s) is a field argument with no group", name, i, arg.Name)

					for version := op.Version; version <= LogicVersion; version++ {
						require.NotEmptyf(t,
							ToolArgValuesForTools(arg.Kind, arg.FieldGroup, version, mode),
							"%s arg %d (%s) resolves to no values at v%d mode %d",
							name, i, arg.Name, version, mode)
					}
				}
			}
		}
	}
}

// TestToolOpcodesForToolsNewestVersions checks that opcodes added in the most
// recent versions expose their immediates to editors.
func TestToolOpcodesForToolsNewestVersions(t *testing.T) {
	poseidon2, ok := ToolOpcodeForTools("poseidon2", 1, ModeApp)
	require.True(t, ok)
	require.Equal(t, uint64(poseidon2Version), poseidon2.Version)
	require.Len(t, poseidon2.Args, 1)
	require.Equal(t, ToolArgField, poseidon2.Args[0].Kind)
	require.Equal(t, &Poseidon2Configs, poseidon2.Args[0].FieldGroup)
	poseidon2Values := toolArgValueNamesForTest(
		ToolArgValuesForTools(poseidon2.Args[0].Kind, poseidon2.Args[0].FieldGroup, LogicVersion, ModeApp))
	require.Contains(t, poseidon2Values, "BN254t2")

	appParamsSet, ok := ToolOpcodeForTools("app_params_set", 1, ModeApp)
	require.True(t, ok)
	require.Equal(t, uint64(foreignBoxVersion), appParamsSet.Version)
	require.Len(t, appParamsSet.Args, 1)
	require.Equal(t, ToolArgField, appParamsSet.Args[0].Kind)
	require.Equal(t, &AppParamsSettableFields, appParamsSet.Args[0].FieldGroup)
	require.NotEmpty(t,
		ToolArgValuesForTools(appParamsSet.Args[0].Kind, appParamsSet.Args[0].FieldGroup, LogicVersion, ModeApp))

	// sumhash512 is the newest opcode and takes no immediates.
	sumhash, ok := ToolOpcodeForTools("sumhash512", 0, ModeApp)
	require.True(t, ok)
	require.Equal(t, uint64(sumhashVersion), sumhash.Version)
	require.Empty(t, sumhash.Args)

	// Opcodes only reachable at the newest versions must be offered there.
	newest := toolOpcodeNamesForTest(ToolOpcodesForTools(LogicVersion, ModeApp))
	require.Contains(t, newest, "poseidon2")
	require.Contains(t, newest, "app_params_set")
	require.Contains(t, newest, "sumhash512")
	require.Contains(t, newest, "app_box_create")

	older := toolOpcodeNamesForTest(ToolOpcodesForTools(sumhashVersion-1, ModeApp))
	require.NotContains(t, older, "sumhash512")
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
