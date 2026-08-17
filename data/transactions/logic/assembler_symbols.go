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

import "sort"

// SourceIdentifierKind classifies a symbol or reference found at a source
// position.
type SourceIdentifierKind int

const (
	SourceIdentifierSymbol SourceIdentifierKind = iota
	SourceIdentifierReference
)

// SourceIdentifier describes a source identifier found in SourceIndex.
type SourceIdentifier struct {
	Name      string
	Kind      SourceIdentifierKind
	Symbol    *SourceSymbol
	Reference *SourceReference
}

// SourceSymbolAtForTools returns the symbol that overlaps a source byte-offset
// position.
func SourceSymbolAtForTools(idx SourceIndex, line int, column int) (SourceSymbol, bool) {
	for _, symbol := range idx.Symbols {
		if sourceRangesOverlap(sourceRangeAt(line, column), SourceSymbolRangeForTools(symbol)) {
			return symbol, true
		}
	}
	return SourceSymbol{}, false
}

// SourceReferenceAtForTools returns the reference that overlaps a source
// byte-offset position.
func SourceReferenceAtForTools(idx SourceIndex, line int, column int) (SourceReference, bool) {
	for _, ref := range idx.References {
		if sourceRangesOverlap(sourceRangeAt(line, column), SourceReferenceRangeForTools(ref)) {
			return ref, true
		}
	}
	return SourceReference{}, false
}

// SourceIdentifierAtForTools returns the symbol or reference that overlaps a
// source byte-offset position. Symbols are preferred when both match.
func SourceIdentifierAtForTools(idx SourceIndex, line int, column int) (SourceIdentifier, bool) {
	if symbol, ok := SourceSymbolAtForTools(idx, line, column); ok {
		return SourceIdentifier{
			Name:   symbol.Name,
			Kind:   SourceIdentifierSymbol,
			Symbol: &symbol,
		}, true
	}
	if ref, ok := SourceReferenceAtForTools(idx, line, column); ok {
		return SourceIdentifier{
			Name:      ref.Name,
			Kind:      SourceIdentifierReference,
			Reference: &ref,
		}, true
	}
	return SourceIdentifier{}, false
}

// SourceIdentifierRangeForTools returns the source range an identifier was
// found at, which is the symbol's name for a definition and the whole token for
// a reference.
func SourceIdentifierRangeForTools(identifier SourceIdentifier) (SourceRange, bool) {
	if identifier.Symbol != nil {
		return SourceSymbolNameRangeForTools(*identifier.Symbol), true
	}
	if identifier.Reference != nil {
		return SourceReferenceRangeForTools(*identifier.Reference), true
	}
	return SourceRange{}, false
}

// SourceSymbolsByNameForTools returns symbols with the requested name.
func SourceSymbolsByNameForTools(idx SourceIndex, name string) []SourceSymbol {
	var symbols []SourceSymbol
	for _, symbol := range idx.Symbols {
		if symbol.Name == name {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// SourceReferencesByNameForTools returns references with the requested name.
func SourceReferencesByNameForTools(idx SourceIndex, name string) []SourceReference {
	var refs []SourceReference
	for _, ref := range idx.References {
		if ref.Name == name {
			refs = append(refs, ref)
		}
	}
	return refs
}

// SourceDefinedNamesForTools returns names introduced with #define, sorted for
// stable editor presentation.
func SourceDefinedNamesForTools(idx SourceIndex) []string {
	var names []string
	seen := make(map[string]bool)
	for _, symbol := range idx.Symbols {
		if symbol.Kind != SourceSymbolDefine || seen[symbol.Name] {
			continue
		}
		seen[symbol.Name] = true
		names = append(names, symbol.Name)
	}
	sort.Strings(names)
	return names
}

// SourceSymbolNameRangeForTools returns the range of a symbol name, excluding
// label punctuation such as a trailing colon.
func SourceSymbolNameRangeForTools(symbol SourceSymbol) SourceRange {
	return SourceRange{
		Line:      symbol.Line,
		Column:    symbol.Column,
		EndLine:   symbol.Line,
		EndColumn: sourceSymbolNameEndColumn(symbol),
	}
}

// SourceSymbolRangeForTools returns the full source range of a symbol token.
func SourceSymbolRangeForTools(symbol SourceSymbol) SourceRange {
	return SourceRange{
		Line:      symbol.Line,
		Column:    symbol.Column,
		EndLine:   symbol.Line,
		EndColumn: symbol.EndColumn,
	}
}

// SourceReferenceRangeForTools returns the source range of a reference.
func SourceReferenceRangeForTools(ref SourceReference) SourceRange {
	return SourceRange{
		Line:      ref.Line,
		Column:    ref.Column,
		EndLine:   ref.Line,
		EndColumn: ref.EndColumn,
	}
}

func sourceRangeAt(line int, column int) SourceRange {
	return SourceRange{
		Line:      line,
		Column:    column,
		EndLine:   line,
		EndColumn: column,
	}
}
