package lsp

import (
	"fmt"
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

func (r ProcessResult) AvailableOps() []logic.ToolOpcode {
	return logic.ToolOpcodesForTools(r.Version, r.Mode)
}

func (r ProcessResult) ArgVals(arg logic.ToolArg) []logic.ToolArgValue {
	if arg.Kind == logic.ToolArgLabel {
		var res []logic.ToolArgValue
		for _, sym := range r.SourceIndex.Symbols {
			res = append(res, logic.ToolArgValue{
				NoValue:   true,
				Name:      sym.Name,
				Docs:      sym.Docs,
				Signature: sym.Signature,
			})
		}
		return res
	}

	return logic.ToolArgValuesForTools(arg.Kind, arg.FieldGroup, r.Version, r.Mode)
}

func (r ProcessResult) DocAt(l int, ch int) string {
	column := r.sourceColumn(l, ch)
	op, ok := logic.SourceOperationAtForTools(r.SourceLines, r.SourceProgram, l, column)
	if !ok {
		return ""
	}
	if column >= op.Token.Column && column <= op.Token.EndColumn {
		meta, ok := logic.ToolOpcodeForTools(op.Name, len(op.Args), r.Mode)
		if ok {
			return MakeFullDoc(meta.Docs, meta.ExtraDocs)
		}
	}
	for _, arg := range op.Args {
		if column >= arg.Token.Column && column <= arg.Token.EndColumn && arg.Docs != "" {
			name := arg.ValueName
			if name == "" {
				name = arg.Token.Text
			}
			return fmt.Sprintf("%s = %d\r\n%s", name, arg.Value, arg.Docs)
		}
	}
	return ""
}

func (r ProcessResult) sourceColumn(line int, character int) int {
	if line < 0 || line >= len(r.SourceLines) {
		return character
	}
	return byteColumnFromUTF16Column(r.SourceLines[line].Text, character)
}

func tokenFromSourceToken(line string, sourceToken logic.SourceToken) Token {
	return Token{
		v: sourceToken.Text,
		l: sourceToken.Line,
		b: utf16ColumnFromByte(line, sourceToken.Column),
		e: utf16ColumnFromByte(line, sourceToken.EndColumn),
		t: tokenTypeFromSourceToken(sourceToken.Kind),
	}
}

func tokenTypeFromSourceToken(kind logic.SourceTokenKind) TokenType {
	switch kind {
	case logic.SourceTokenSemicolon:
		return TokenSemicolon
	case logic.SourceTokenComment:
		return TokenComment
	default:
		return TokenValue
	}
}

func tokenFromSourceTokenInLines(lines []logic.SourceLine, sourceToken logic.SourceToken) (Token, bool) {
	if sourceToken.Line < 0 || sourceToken.Line >= len(lines) {
		return Token{}, false
	}
	return tokenFromSourceToken(lines[sourceToken.Line].Text, sourceToken), true
}

func tokenFromSourceRequiredVersion(lines []logic.SourceLine, required logic.SourceRequiredVersion) (Token, bool) {
	if required.Line < 0 || required.Line >= len(lines) {
		return Token{}, false
	}
	return Token{
		l: required.Line,
		b: utf16ColumnFromByte(lines[required.Line].Text, required.Column),
		e: utf16ColumnFromByte(lines[required.Line].Text, required.EndColumn),
		t: TokenValue,
	}, true
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

func MakeFullDoc(short string, extra string) string {
	if extra == "" {
		return short
	}
	if short == "" {
		return extra
	}
	return fmt.Sprintf("%s\r\n\r\n%s", short, extra)
}

func Process(source string) *ProcessResult {
	initialLines := logic.SourceLinesForTools(source)
	mode := sourceModeForLSP(initialLines)
	analysis := logic.AnalyzeSourceForToolsWithOptions(source, logic.SourceToolOptions{
		Mode: mode,
	})

	return &ProcessResult{
		Mode:                 mode,
		Version:              analysis.Index.Version,
		SourceLines:          analysis.Lines,
		SourceIndex:          analysis.Index,
		SourceProgram:        analysis.Program,
		AssemblerDiagnostics: analysis.Diagnostics,
		OpStream:             analysis.OpStream,
		AssembleError:        analysis.Err,
	}
}
