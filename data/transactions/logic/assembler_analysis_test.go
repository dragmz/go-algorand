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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/test/partitiontest"
)

func TestAnalyzeSourceForToolsAssemblerError(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForTools("int 1\nunknown")
	require.Error(t, result.Err)
	require.NotNil(t, result.OpStream)
	require.Len(t, result.Lines, 2)
	require.NotEmpty(t, result.Diagnostics)

	diag := result.Diagnostics[0]
	require.Equal(t, SourceDiagnosticError, diag.Severity)
	require.Equal(t, "unknown opcode: unknown", diag.Message)
	require.Equal(t, 1, diag.Line)
	require.Equal(t, 0, diag.Column)
	require.Equal(t, len("unknown"), diag.EndColumn)
}

func TestAnalyzeSourceForToolsAssemblerWarning(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForTools("#pragma version 4\nintcblock 1\nint 1")
	require.NoError(t, result.Err)
	require.NotNil(t, result.OpStream)
	require.Len(t, result.Diagnostics, 1)

	diag := result.Diagnostics[0]
	require.Equal(t, SourceDiagnosticWarning, diag.Severity)
	require.Equal(t, "int 1 used with explicit intcblock. must pushint", diag.Message)
	require.Equal(t, 2, diag.Line)
	require.Equal(t, 4, diag.Column)
	require.Equal(t, 5, diag.EndColumn)
}

func TestAnalyzeSourceForToolsFallbackDiagnostic(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	diagnostics := sourceDiagnosticsFromAssembly(nil, nil, errors.New("fallback"))
	require.Equal(t, []SourceDiagnostic{{
		Message:  "fallback",
		Severity: SourceDiagnosticError,
	}}, diagnostics)
}

func TestAnalyzeSourceForToolsWithVersion(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForToolsWithVersion("int 1", 2)
	require.NoError(t, result.Err)
	require.NotNil(t, result.OpStream)
	require.Equal(t, uint64(2), result.OpStream.Version)
}

func TestSourceDiagnosticsForToolsProgramSize(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForTools("int 1")

	require.Empty(t, SourceDiagnosticsForTools(result, SourceDiagnosticOptions{}))
	require.Equal(t, []SourceDiagnostic{{
		Message:  fmt.Sprintf("Program size: %d", len(result.OpStream.Program)),
		Severity: SourceDiagnosticInfo,
	}}, SourceDiagnosticsForTools(result, SourceDiagnosticOptions{ProgramSize: true}))

	result = AnalyzeSourceForTools("unknown")
	for _, diagnostic := range SourceDiagnosticsForTools(result, SourceDiagnosticOptions{ProgramSize: true}) {
		require.NotEqual(t, SourceDiagnosticInfo, diagnostic.Severity)
	}
}

func TestSourcePositionForProgramCounterForTools(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForTools("int 1")
	pcs := sourceProgramCounters(result)
	require.NotEmpty(t, pcs)

	position, ok := SourcePositionForProgramCounterForTools(result, pcs[0])
	require.True(t, ok)
	require.Equal(t, SourcePosition{}, position)

	_, ok = SourcePositionForProgramCounterForTools(result, -1)
	require.False(t, ok)

	_, ok = SourcePositionForProgramCounterForTools(SourceAnalysisResult{}, 0)
	require.False(t, ok)
}

func TestSourceMapForTools(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	result := AnalyzeSourceForTools("int 1")

	sourceMap, ok := SourceMapForTools(result, []string{"test.teal"})
	require.True(t, ok)
	require.Equal(t, []string{"test.teal"}, sourceMap.Sources)
	require.NotEmpty(t, sourceMap.Mappings)

	_, ok = SourceMapForTools(SourceAnalysisResult{}, []string{"test.teal"})
	require.False(t, ok)
}
