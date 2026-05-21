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
