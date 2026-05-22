package lsp

import (
	"strings"
	"unicode/utf8"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func sourceColumn(lines []logic.SourceLine, line int, character int) int {
	if line < 0 || line >= len(lines) {
		return character
	}
	return byteColumnFromUTF16Column(lines[line].Text, character)
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

func sourceModeForLSP(lines []logic.SourceLine) logic.RunMode {
	for _, line := range lines {
		if line.Comment != nil && strings.TrimSpace(line.Comment.Text) == "#pragma mode logicsig" {
			return logic.ModeSig
		}
	}
	return logic.ModeApp
}

func Process(source string) *logic.SourceAnalysisResult {
	initialLines := logic.SourceLinesForTools(source)
	mode := sourceModeForLSP(initialLines)
	analysis := logic.AnalyzeSourceForToolsWithOptions(source, logic.SourceToolOptions{
		Mode: mode,
	})

	return &analysis
}
