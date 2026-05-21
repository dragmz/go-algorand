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

// SourceDiagnosticSeverity classifies diagnostics returned by
// AnalyzeSourceForTools.
type SourceDiagnosticSeverity int

const (
	SourceDiagnosticError SourceDiagnosticSeverity = iota + 1
	SourceDiagnosticWarning
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

// SourceAnalysisResult combines assembler source structure with assembly output
// and public diagnostics for editor tooling.
type SourceAnalysisResult struct {
	Lines       []SourceLine
	Index       SourceIndex
	OpStream    *OpStream
	Diagnostics []SourceDiagnostic
	Err         error
}

// AnalyzeSourceForTools analyzes source using the same version behavior as
// AssembleString, including #pragma version and default assembler fallback.
func AnalyzeSourceForTools(source string) SourceAnalysisResult {
	return analyzeSourceForTools(source, assemblerNoVersion)
}

// AnalyzeSourceForToolsWithVersion analyzes source using an explicit assembler
// version, matching AssembleStringWithVersion.
func AnalyzeSourceForToolsWithVersion(source string, version uint64) SourceAnalysisResult {
	return analyzeSourceForTools(source, version)
}

func analyzeSourceForTools(source string, version uint64) SourceAnalysisResult {
	lines := SourceLinesForTools(source)
	ops, err := AssembleStringWithVersion(source, version)
	diagnostics := sourceDiagnosticsFromAssembly(lines, ops, err)

	return SourceAnalysisResult{
		Lines:       lines,
		Index:       sourceIndexForTools(lines, ops),
		OpStream:    ops,
		Diagnostics: diagnostics,
		Err:         err,
	}
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
