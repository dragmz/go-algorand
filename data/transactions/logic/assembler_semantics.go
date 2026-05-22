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

// SourceSemanticTokenKind classifies source tokens for semantic highlighting.
type SourceSemanticTokenKind int

const (
	SourceSemanticOpcode SourceSemanticTokenKind = iota
	SourceSemanticMacro
	SourceSemanticBool
	SourceSemanticNumber
	SourceSemanticString
	SourceSemanticKeyword
	SourceSemanticComment
	SourceSemanticSymbol
	SourceSemanticReference
)

// SourceSemanticToken is an editor-neutral semantic token in source byte
// offsets.
type SourceSemanticToken struct {
	Kind  SourceSemanticTokenKind
	Range SourceRange
}

// SourceSemanticTokensForTools returns semantic tokens derived from assembler
// source analysis.
func SourceSemanticTokensForTools(result SourceAnalysisResult) []SourceSemanticToken {
	var tokens []SourceSemanticToken

	for _, class := range result.Program.TokenClasses {
		tokens = append(tokens, SourceSemanticToken{
			Kind:  sourceSemanticKindFromTokenClass(class.Kind),
			Range: sourceRangeFromToken(class.Token),
		})
	}

	for _, line := range result.Lines {
		if line.Comment == nil {
			continue
		}
		tokens = append(tokens, SourceSemanticToken{
			Kind:  SourceSemanticComment,
			Range: sourceRangeFromToken(*line.Comment),
		})
	}

	for _, symbol := range result.Index.Symbols {
		tokens = append(tokens, SourceSemanticToken{
			Kind:  SourceSemanticSymbol,
			Range: SourceSymbolRangeForTools(symbol),
		})
	}

	for _, ref := range result.Index.References {
		tokens = append(tokens, SourceSemanticToken{
			Kind:  SourceSemanticReference,
			Range: SourceReferenceRangeForTools(ref),
		})
	}

	return tokens
}

func sourceSemanticKindFromTokenClass(kind SourceTokenClassKind) SourceSemanticTokenKind {
	switch kind {
	case SourceTokenClassOpcode:
		return SourceSemanticOpcode
	case SourceTokenClassMacro:
		return SourceSemanticMacro
	case SourceTokenClassBool:
		return SourceSemanticBool
	case SourceTokenClassNumber:
		return SourceSemanticNumber
	case SourceTokenClassString:
		return SourceSemanticString
	case SourceTokenClassKeyword:
		return SourceSemanticKeyword
	default:
		return SourceSemanticString
	}
}
