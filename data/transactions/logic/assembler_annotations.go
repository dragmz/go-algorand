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

// SourcePosition is a zero-based byte-offset position in assembler source.
type SourcePosition struct {
	Line   int
	Column int
}

// SourceCodeLensKind classifies editor-neutral code lens candidates.
type SourceCodeLensKind int

const (
	SourceCodeLensReferenceCount SourceCodeLensKind = iota
	SourceCodeLensProgramCounter
)

// SourceCodeLens is an editor-neutral code lens candidate.
type SourceCodeLens struct {
	Kind           SourceCodeLensKind
	Range          SourceRange
	ReferenceCount int
	ProgramCounter int
}

// SourceInlayKind classifies editor-neutral inlay candidates.
type SourceInlayKind int

const (
	SourceInlayNamedValue SourceInlayKind = iota
	SourceInlayDecodedValue
	SourceInlayProgramCounter
)

// SourceInlay is an editor-neutral inlay candidate.
type SourceInlay struct {
	Kind           SourceInlayKind
	Position       SourcePosition
	Range          SourceRange
	Label          string
	ProgramCounter int
}

// SourceCodeLensesForTools returns code lens candidates derived from assembler
// source analysis.
func SourceCodeLensesForTools(result SourceAnalysisResult) []SourceCodeLens {
	var lenses []SourceCodeLens

	for _, symbol := range result.Index.Symbols {
		count := result.Index.RefCounts[symbol.Name]
		if count == 0 {
			continue
		}
		lenses = append(lenses, SourceCodeLens{
			Kind: SourceCodeLensReferenceCount,
			Range: SourceRange{
				Line:    symbol.Line,
				EndLine: symbol.Line,
			},
			ReferenceCount: count,
		})
	}

	for _, pc := range sourceProgramCounters(result) {
		loc := result.OpStream.OffsetToSource[pc]
		lenses = append(lenses, SourceCodeLens{
			Kind:           SourceCodeLensProgramCounter,
			Range:          sourceRangeFromLocation(loc),
			ProgramCounter: pc,
		})
	}

	return lenses
}

// SourceInlaysForTools returns inlay candidates derived from assembler source
// analysis.
func SourceInlaysForTools(result SourceAnalysisResult) []SourceInlay {
	var inlays []SourceInlay

	for _, hint := range SourceInlayHintsForTools(result.Lines, result.Program) {
		inlay := SourceInlay{
			Position: SourcePosition{
				Line:   hint.Token.Line,
				Column: hint.Token.EndColumn,
			},
			Range: sourceRangeFromToken(hint.Token),
			Label: hint.Label,
		}
		switch hint.Kind {
		case SourceInlayHintNamed:
			inlay.Kind = SourceInlayNamedValue
		case SourceInlayHintDecoded:
			inlay.Kind = SourceInlayDecodedValue
		default:
			continue
		}
		inlays = append(inlays, inlay)
	}

	for _, pc := range sourceProgramCounters(result) {
		loc := result.OpStream.OffsetToSource[pc]
		inlays = append(inlays, SourceInlay{
			Kind: SourceInlayProgramCounter,
			Position: SourcePosition{
				Line:   loc.Line,
				Column: loc.Column,
			},
			Range:          sourceRangeFromLocation(loc),
			ProgramCounter: pc,
		})
	}

	return inlays
}

func sourceProgramCounters(result SourceAnalysisResult) []int {
	if result.OpStream == nil || result.OpStream.OffsetToSource == nil {
		return nil
	}
	pcs := make([]int, 0, len(result.OpStream.OffsetToSource))
	for pc := range result.OpStream.OffsetToSource {
		pcs = append(pcs, pc)
	}
	sort.Ints(pcs)
	return pcs
}

func sourceRangeFromLocation(loc SourceLocation) SourceRange {
	return SourceRange{
		Line:      loc.Line,
		Column:    loc.Column,
		EndLine:   loc.Line,
		EndColumn: loc.Column,
	}
}
