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

import (
	"fmt"
	"strings"
)

// SourceRange is a zero-based byte-offset range in assembler source.
type SourceRange struct {
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}

// SourceEdit is an LSP-neutral text edit in assembler source coordinates.
type SourceEdit struct {
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	NewText   string
}

// SourceActionKind classifies source actions for editor tooling.
type SourceActionKind int

const (
	SourceActionRemoveStatement SourceActionKind = iota
	SourceActionCreateLabel
	SourceActionReplaceValue
	SourceActionUpdateVersion
)

// SourceAction is an LSP-neutral quick action with source edits.
type SourceAction struct {
	Kind  SourceActionKind
	Title string
	Edits []SourceEdit
}

// SourceEditRemoveStatementForTools returns an edit that removes one
// semicolon-delimited source statement.
func SourceEditRemoveStatementForTools(lines []SourceLine, line int, statement int) (SourceEdit, bool) {
	if line < 0 || line >= len(lines) {
		return SourceEdit{}, false
	}
	statements := lines[line].Statements
	if statement < 0 || statement >= len(statements) {
		return SourceEdit{}, false
	}
	stmt := statements[statement]
	return SourceEdit{
		Line:      stmt.Line,
		Column:    stmt.Column,
		EndLine:   stmt.Line,
		EndColumn: stmt.EndColumn,
	}, true
}

// SourceEditCreateLabelForTools returns an edit that appends a label
// definition.
func SourceEditCreateLabelForTools(lines []SourceLine, name string) SourceEdit {
	line := len(lines)
	return SourceEdit{
		Line:      line,
		Column:    0,
		EndLine:   line,
		EndColumn: 0,
		NewText:   fmt.Sprintf("\r\n%s:\r\n", name),
	}
}

// SourceEditsRemoveSymbolForTools returns edits that remove symbol
// definitions with the given name.
func SourceEditsRemoveSymbolForTools(idx SourceIndex, name string) []SourceEdit {
	var edits []SourceEdit
	for _, symbol := range idx.Symbols {
		if symbol.Name != name {
			continue
		}
		edits = append(edits, SourceEdit{
			Line:      symbol.Line,
			Column:    symbol.Column,
			EndLine:   symbol.Line,
			EndColumn: symbol.EndColumn,
		})
	}
	return edits
}

// SourceEditsRenameSymbolForTools returns edits that rename symbol definitions
// and references with the given name.
func SourceEditsRenameSymbolForTools(idx SourceIndex, name string, newName string) []SourceEdit {
	var edits []SourceEdit
	for _, symbol := range idx.Symbols {
		if symbol.Name != name {
			continue
		}
		edits = append(edits, SourceEdit{
			Line:      symbol.Line,
			Column:    symbol.Column,
			EndLine:   symbol.Line,
			EndColumn: sourceSymbolNameEndColumn(symbol),
			NewText:   newName,
		})
	}
	for _, ref := range idx.References {
		if ref.Name != name {
			continue
		}
		edits = append(edits, SourceEdit{
			Line:      ref.Line,
			Column:    ref.Column,
			EndLine:   ref.Line,
			EndColumn: ref.EndColumn,
			NewText:   newName,
		})
	}
	return edits
}

// SourceEditUpdateVersionForTools returns an edit that updates or inserts
// #pragma version.
func SourceEditUpdateVersionForTools(idx SourceIndex, version uint64) SourceEdit {
	if idx.VersionToken != nil {
		return SourceEditReplaceTokenForTools(*idx.VersionToken, fmt.Sprintf("%d", version))
	}
	return SourceEdit{
		Line:      0,
		Column:    0,
		EndLine:   0,
		EndColumn: 0,
		NewText:   fmt.Sprintf("#pragma version %d\r\n", version),
	}
}

// SourceEditReplaceTokenForTools returns an edit that replaces a token.
func SourceEditReplaceTokenForTools(token SourceToken, value string) SourceEdit {
	return SourceEdit{
		Line:      token.Line,
		Column:    token.Column,
		EndLine:   token.Line,
		EndColumn: token.EndColumn,
		NewText:   value,
	}
}

// SourceActionsForTools returns quick actions that are available inside a
// source byte-offset range.
func SourceActionsForTools(lines []SourceLine, idx SourceIndex, program SourceProgram, rg SourceRange) []SourceAction {
	var actions []SourceAction

	for _, redundant := range idx.Redundants {
		if !sourceRangesOverlap(rg, sourceRangeFromRedundant(redundant)) {
			continue
		}
		edit, ok := SourceEditRemoveStatementForTools(lines, redundant.Line, redundant.Statement)
		if !ok {
			continue
		}
		actions = append(actions, SourceAction{
			Kind:  SourceActionRemoveStatement,
			Title: redundant.Message,
			Edits: []SourceEdit{edit},
		})
	}

	for _, ref := range idx.MissingReferences {
		if !sourceRangesOverlap(rg, sourceRangeFromReference(ref)) {
			continue
		}
		actions = append(actions, SourceAction{
			Kind:  SourceActionCreateLabel,
			Title: fmt.Sprintf("Create label '%s'", ref.Name),
			Edits: []SourceEdit{SourceEditCreateLabelForTools(lines, ref.Name)},
		})
	}

	for _, hint := range SourceInlayHintsForTools(lines, program) {
		if !sourceRangesOverlap(rg, sourceRangeFromToken(hint.Token)) {
			continue
		}
		switch hint.Kind {
		case SourceInlayHintNamed:
			actions = append(actions, SourceAction{
				Kind:  SourceActionReplaceValue,
				Title: fmt.Sprintf("Replace with '%s'", hint.Label),
				Edits: []SourceEdit{SourceEditReplaceTokenForTools(hint.Token, hint.Label)},
			})
		case SourceInlayHintDecoded:
			actions = append(actions, SourceAction{
				Kind:  SourceActionReplaceValue,
				Title: fmt.Sprintf("Replace with literal '%s'", hint.Label),
				Edits: []SourceEdit{SourceEditReplaceTokenForTools(hint.Token, fmt.Sprintf("\"%s\"", strings.ReplaceAll(hint.Label, "\"", "\\\"")))},
			})
		default:
		}
	}

	for _, required := range program.RequiredVersions {
		if !sourceRangesOverlap(rg, sourceRangeFromRequiredVersion(required)) {
			continue
		}
		actions = append(actions, SourceAction{
			Kind:  SourceActionUpdateVersion,
			Title: fmt.Sprintf("Update version to %d", required.Version),
			Edits: []SourceEdit{SourceEditUpdateVersionForTools(idx, required.Version)},
		})
	}

	return actions
}

func sourceSymbolNameEndColumn(symbol SourceSymbol) int {
	return symbol.Column + len(symbol.Name)
}

func sourceRangeFromToken(token SourceToken) SourceRange {
	return SourceRange{
		Line:      token.Line,
		Column:    token.Column,
		EndLine:   token.Line,
		EndColumn: token.EndColumn,
	}
}

func sourceRangeFromReference(ref SourceReference) SourceRange {
	return SourceRange{
		Line:      ref.Line,
		Column:    ref.Column,
		EndLine:   ref.Line,
		EndColumn: ref.EndColumn,
	}
}

func sourceRangeFromRedundant(redundant SourceRedundant) SourceRange {
	return SourceRange{
		Line:      redundant.Line,
		Column:    redundant.Column,
		EndLine:   redundant.Line,
		EndColumn: redundant.EndColumn,
	}
}

func sourceRangeFromRequiredVersion(required SourceRequiredVersion) SourceRange {
	return SourceRange{
		Line:      required.Line,
		Column:    required.Column,
		EndLine:   required.Line,
		EndColumn: required.EndColumn,
	}
}

func sourceRangesOverlap(a SourceRange, b SourceRange) bool {
	if a.EndLine < b.Line || b.EndLine < a.Line {
		return false
	}
	if a.EndLine == b.Line && a.EndColumn < b.Column {
		return false
	}
	if b.EndLine == a.Line && b.EndColumn < a.Column {
		return false
	}
	return true
}
