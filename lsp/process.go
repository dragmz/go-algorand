package lsp

import (
	"unicode/utf8"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

type ProcessResult struct {
	Mode logic.RunMode

	Version uint64

	SourceLines          []logic.SourceLine
	SourceIndex          logic.SourceIndex
	SourceProgram        logic.SourceProgram
	AssemblerDiagnostics []logic.SourceDiagnostic
	OpStream             *logic.OpStream
	AssembleError        error
}

func (r ProcessResult) sourceAnalysis() logic.SourceAnalysisResult {
	return logic.SourceAnalysisResult{
		Lines:       r.SourceLines,
		Index:       r.SourceIndex,
		Program:     r.SourceProgram,
		Mode:        r.Mode,
		OpStream:    r.OpStream,
		Diagnostics: r.AssemblerDiagnostics,
		Err:         r.AssembleError,
	}
}

func (r ProcessResult) sourceColumn(line int, character int) int {
	if line < 0 || line >= len(r.SourceLines) {
		return character
	}
	return byteColumnFromUTF16Column(r.SourceLines[line].Text, character)
}

func utf16ColumnFromByte(line string, column int) int {
	if column < 0 {
		column = 0
	}
	if column > len(line) {
		column = len(line)
	}
	return utf16LenString(line[:column])
}

func byteColumnFromUTF16Column(line string, character int) int {
	if character <= 0 {
		return 0
	}
	units := 0
	for i, r := range line {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > character {
			return i
		}
		units += width
		if units == character {
			return i + utf8.RuneLen(r)
		}
	}
	return len(line)
}

func Process(source string) *ProcessResult {
	initialLines := logic.SourceLinesForTools(source)
	mode := sourceModeForLSP(initialLines)
	analysis := logic.AnalyzeSourceForToolsWithOptions(source, logic.SourceToolOptions{
		Mode: mode,
	})

	return &ProcessResult{
		Mode:                 analysis.Mode,
		Version:              analysis.Index.Version,
		SourceLines:          analysis.Lines,
		SourceIndex:          analysis.Index,
		SourceProgram:        analysis.Program,
		AssemblerDiagnostics: analysis.Diagnostics,
		OpStream:             analysis.OpStream,
		AssembleError:        analysis.Err,
	}
}
