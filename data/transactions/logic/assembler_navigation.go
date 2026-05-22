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

// SourcePrepareRename describes an identifier that can be renamed by editor
// tooling.
type SourcePrepareRename struct {
	Range       SourceRange
	Placeholder string
}

// SourceHighlight describes a source range that should be highlighted with the
// identifier at the requested position.
type SourceHighlight struct {
	Range SourceRange
}

// SourceDocumentSymbol describes a symbol suitable for editor outline views.
type SourceDocumentSymbol struct {
	Name           string
	Range          SourceRange
	SelectionRange SourceRange
}

// SourcePrepareRenameForTools returns the rename range and placeholder for the
// identifier at a source byte-offset position.
func SourcePrepareRenameForTools(result SourceAnalysisResult, line int, column int) (SourcePrepareRename, bool) {
	identifier, ok := SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return SourcePrepareRename{}, false
	}
	if identifier.Symbol != nil {
		return SourcePrepareRename{
			Range:       SourceSymbolNameRangeForTools(*identifier.Symbol),
			Placeholder: identifier.Name,
		}, true
	}
	if identifier.Reference != nil {
		return SourcePrepareRename{
			Range:       SourceReferenceRangeForTools(*identifier.Reference),
			Placeholder: identifier.Name,
		}, true
	}
	return SourcePrepareRename{}, false
}

// SourceRenameEditsForTools returns source edits that rename the identifier at
// a source byte-offset position.
func SourceRenameEditsForTools(result SourceAnalysisResult, line int, column int, newName string) []SourceEdit {
	identifier, ok := SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	return SourceEditsRenameSymbolForTools(result.Index, identifier.Name, newName)
}

// SourceDefinitionsForTools returns definition ranges for the reference at a
// source byte-offset position.
func SourceDefinitionsForTools(result SourceAnalysisResult, line int, column int) []SourceRange {
	ref, ok := SourceReferenceAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	var ranges []SourceRange
	for _, symbol := range SourceSymbolsByNameForTools(result.Index, ref.Name) {
		ranges = append(ranges, SourceSymbolNameRangeForTools(symbol))
	}
	return ranges
}

// SourceHighlightsForTools returns definition and reference ranges matching the
// identifier at a source byte-offset position.
func SourceHighlightsForTools(result SourceAnalysisResult, line int, column int) []SourceHighlight {
	identifier, ok := SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	var highlights []SourceHighlight
	for _, symbol := range SourceSymbolsByNameForTools(result.Index, identifier.Name) {
		highlights = append(highlights, SourceHighlight{Range: SourceSymbolNameRangeForTools(symbol)})
	}
	for _, ref := range SourceReferencesByNameForTools(result.Index, identifier.Name) {
		highlights = append(highlights, SourceHighlight{Range: SourceReferenceRangeForTools(ref)})
	}
	return highlights
}

// SourceDocumentSymbolsForTools returns source symbols for editor outline
// views.
func SourceDocumentSymbolsForTools(result SourceAnalysisResult) []SourceDocumentSymbol {
	symbols := make([]SourceDocumentSymbol, 0, len(result.Index.Symbols))
	for _, symbol := range result.Index.Symbols {
		symbols = append(symbols, SourceDocumentSymbol{
			Name:           symbol.Name,
			Range:          SourceSymbolRangeForTools(symbol),
			SelectionRange: SourceSymbolNameRangeForTools(symbol),
		})
	}
	return symbols
}
