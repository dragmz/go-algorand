package lsp

import (
	"fmt"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func symbolsFromSourceIndex(lines []logic.SourceLine, idx logic.SourceIndex) []Symbol {
	symbols := make([]Symbol, 0, len(idx.Symbols))
	for _, symbol := range idx.Symbols {
		symbols = append(symbols, labelSymbol{
			n:    symbol.Name,
			l:    symbol.Line,
			b:    sourceIndexUTF16Column(lines, symbol.Line, symbol.Column),
			e:    sourceIndexUTF16Column(lines, symbol.Line, symbol.EndColumn),
			docs: symbol.Docs,
			sig:  symbol.Signature,
		})
	}
	return symbols
}

func referencesFromSourceIndex(lines []logic.SourceLine, refs []logic.SourceReference) []Token {
	tokens := make([]Token, 0, len(refs))
	for _, ref := range refs {
		tokens = append(tokens, Token{
			v: ref.Name,
			l: ref.Line,
			b: sourceIndexUTF16Column(lines, ref.Line, ref.Column),
			e: sourceIndexUTF16Column(lines, ref.Line, ref.EndColumn),
			t: TokenValue,
		})
	}
	return tokens
}

func versionTokenFromSourceIndex(lines []logic.SourceLine, idx logic.SourceIndex) *Token {
	if idx.VersionToken == nil {
		return nil
	}
	if idx.VersionToken.Line < 0 || idx.VersionToken.Line >= len(lines) {
		return nil
	}
	token := tokenFromSourceToken(lines[idx.VersionToken.Line].Text, *idx.VersionToken)
	return &token
}

func definesFromSourceIndex(idx logic.SourceIndex) map[string]bool {
	defines := make(map[string]bool)
	for _, symbol := range idx.Symbols {
		if symbol.Kind == logic.SourceSymbolDefine {
			defines[symbol.Name] = true
		}
	}
	return defines
}

func redundantsFromSourceIndex(idx logic.SourceIndex) []RedundantLine {
	redundants := make([]RedundantLine, 0, len(idx.Redundants))
	for _, redundant := range idx.Redundants {
		pos := position{l: redundant.Line, s: redundant.Statement}
		switch redundant.Kind {
		case logic.SourceRedundantUnusedLabel:
			redundants = append(redundants, RedundantLabelLine{p: pos, name: redundant.Name})
		case logic.SourceRedundantBranchToNextLabel:
			redundants = append(redundants, RedundantBLine{p: pos})
		default:
			redundants = append(redundants, sourceRedundantLine{p: pos, message: redundant.Message})
		}
	}
	return redundants
}

func sourceIndexUTF16Column(lines []logic.SourceLine, line int, column int) int {
	if line < 0 || line >= len(lines) {
		return column
	}
	return utf16ColumnFromByte(lines[line].Text, column)
}

type sourceRedundantLine struct {
	p       position
	message string
}

func (l sourceRedundantLine) Line() int {
	return l.p.l
}

func (l sourceRedundantLine) Statement() int {
	return l.p.s
}

func (l sourceRedundantLine) String() string {
	if l.message != "" {
		return l.message
	}
	return fmt.Sprintf("Remove statement on line %d", l.p.l+1)
}
