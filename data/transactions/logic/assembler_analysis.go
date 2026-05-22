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

import "fmt"

// SourceDiagnosticSeverity classifies diagnostics returned by
// AnalyzeSourceForTools.
type SourceDiagnosticSeverity int

const (
	SourceDiagnosticError SourceDiagnosticSeverity = iota + 1
	SourceDiagnosticWarning
	SourceDiagnosticInfo
)

// SourceDiagnostic is a public, normalized assembler diagnostic. Line and
// column values are zero-based, and columns are byte offsets into SourceLine.Text.
type SourceDiagnostic struct {
	Message   string
	Severity  SourceDiagnosticSeverity
	Line      int
	Column    int
	EndColumn int
}

// SourceDiagnosticOptions controls optional diagnostics derived from successful
// source analysis.
type SourceDiagnosticOptions struct {
	ProgramSize bool
}

// SourceAnalysisResult combines assembler source structure with assembly output
// and public diagnostics for editor tooling.
type SourceAnalysisResult struct {
	Lines       []SourceLine
	Index       SourceIndex
	Program     SourceProgram
	Mode        RunMode
	OpStream    *OpStream
	Diagnostics []SourceDiagnostic
	Err         error
}

// AnalyzeSourceForTools analyzes source using the same version behavior as
// AssembleString, including #pragma version and default assembler fallback.
func AnalyzeSourceForTools(source string) SourceAnalysisResult {
	return AnalyzeSourceForToolsWithOptions(source, SourceToolOptions{})
}

// AnalyzeSourceForToolsWithVersion analyzes source using an explicit assembler
// version, matching AssembleStringWithVersion.
func AnalyzeSourceForToolsWithVersion(source string, version uint64) SourceAnalysisResult {
	return AnalyzeSourceForToolsWithOptions(source, SourceToolOptions{Version: version, UseVersion: true})
}

// AnalyzeSourceForToolsWithOptions analyzes source for tooling with explicit
// caller-owned context such as editor-selected run mode.
func AnalyzeSourceForToolsWithOptions(source string, opts SourceToolOptions) SourceAnalysisResult {
	lines := SourceLinesForTools(source)
	version := assemblerNoVersion
	if opts.UseVersion {
		version = opts.Version
	}
	mode := opts.Mode
	if mode == 0 {
		mode = ModeApp
	}
	ops, err := AssembleStringWithVersion(source, version)
	diagnostics := sourceDiagnosticsFromAssembly(lines, ops, err)
	index := sourceIndexForTools(lines, ops)

	return SourceAnalysisResult{
		Lines:       lines,
		Index:       index,
		Program:     sourceProgramForTools(lines, index, opts),
		Mode:        mode,
		OpStream:    ops,
		Diagnostics: diagnostics,
		Err:         err,
	}
}

// SourceDiagnosticsForTools returns diagnostics derived from assembler analysis
// and optional tooling annotations.
func SourceDiagnosticsForTools(result SourceAnalysisResult, opts SourceDiagnosticOptions) []SourceDiagnostic {
	diagnostics := append([]SourceDiagnostic(nil), result.Diagnostics...)
	if opts.ProgramSize && result.Err == nil && result.OpStream != nil {
		diagnostics = append(diagnostics, SourceDiagnostic{
			Message:  fmt.Sprintf("Program size: %d", len(result.OpStream.Program)),
			Severity: SourceDiagnosticInfo,
		})
	}
	return diagnostics
}

// SourcePositionForProgramCounterForTools returns the source position mapped to
// the requested bytecode program counter.
func SourcePositionForProgramCounterForTools(result SourceAnalysisResult, pc int) (SourcePosition, bool) {
	if result.OpStream == nil || result.OpStream.OffsetToSource == nil {
		return SourcePosition{}, false
	}
	loc, ok := result.OpStream.OffsetToSource[pc]
	if !ok {
		return SourcePosition{}, false
	}
	return SourcePosition{Line: loc.Line, Column: loc.Column}, true
}

// SourceMapForTools returns a source map derived from successful source
// analysis.
func SourceMapForTools(result SourceAnalysisResult, sourceNames []string) (SourceMap, bool) {
	if result.OpStream == nil || result.OpStream.OffsetToSource == nil {
		return SourceMap{}, false
	}
	return GetSourceMap(sourceNames, result.OpStream.OffsetToSource), true
}

func sourceDiagnosticsFromAssembly(lines []SourceLine, ops *OpStream, err error) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	hasError := false

	if ops != nil {
		for _, sourceErr := range ops.Errors {
			hasError = true
			diagnostics = append(diagnostics, sourceDiagnosticFromError(lines, sourceErr, SourceDiagnosticError))
		}
		for _, sourceErr := range ops.Warnings {
			diagnostics = append(diagnostics, sourceDiagnosticFromError(lines, sourceErr, SourceDiagnosticWarning))
		}
	}

	if err != nil && !hasError {
		diagnostics = append(diagnostics, SourceDiagnostic{
			Message:  err.Error(),
			Severity: SourceDiagnosticError,
		})
	}

	return diagnostics
}

func sourceDiagnosticFromError(lines []SourceLine, sourceErr sourceError, severity SourceDiagnosticSeverity) SourceDiagnostic {
	line, column := normalizeSourceErrorLocation(sourceErr)
	endColumn := column
	if line >= 0 && line < len(lines) {
		endColumn = sourceDiagnosticEndColumn(lines[line], column)
	}

	return SourceDiagnostic{
		Message:   sourceErr.Err.Error(),
		Severity:  severity,
		Line:      line,
		Column:    column,
		EndColumn: endColumn,
	}
}

func normalizeSourceErrorLocation(sourceErr sourceError) (int, int) {
	line := sourceErr.Line
	if line > 0 {
		line--
	}
	if line < 0 {
		line = 0
	}

	column := sourceErr.Column
	if column < 0 {
		column = 0
	}

	return line, column
}

func sourceDiagnosticEndColumn(line SourceLine, column int) int {
	for _, token := range line.Tokens {
		if column >= token.Column && column < token.EndColumn {
			return token.EndColumn
		}
	}
	if line.Comment != nil && column >= line.Comment.Column && column < line.Comment.EndColumn {
		return line.Comment.EndColumn
	}
	return column
}
