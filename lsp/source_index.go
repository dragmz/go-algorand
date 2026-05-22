package lsp

import "github.com/algorand/go-algorand/data/transactions/logic"

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

func definesFromSourceIndex(idx logic.SourceIndex) map[string]bool {
	defines := make(map[string]bool)
	for _, symbol := range idx.Symbols {
		if symbol.Kind == logic.SourceSymbolDefine {
			defines[symbol.Name] = true
		}
	}
	return defines
}

func sourceIndexUTF16Column(lines []logic.SourceLine, line int, column int) int {
	if line < 0 || line >= len(lines) {
		return column
	}
	return utf16ColumnFromByte(lines[line].Text, column)
}
