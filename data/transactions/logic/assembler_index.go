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
	"strconv"
	"strings"
)

// SourceSymbolKind classifies source symbols returned by SourceIndex.
type SourceSymbolKind int

const (
	SourceSymbolLabel SourceSymbolKind = iota
	SourceSymbolDefine
)

// SourceReferenceKind classifies source references returned by SourceIndex.
type SourceReferenceKind int

const (
	SourceReferenceLabel SourceReferenceKind = iota
	SourceReferenceDefine
)

// SourceRedundantKind classifies source statements that editor tooling may
// offer to remove.
type SourceRedundantKind int

const (
	SourceRedundantUnusedLabel SourceRedundantKind = iota
	SourceRedundantBranchToNextLabel
)

// SourceSymbol is an assembler source symbol. Line and Statement values are
// zero-based. Columns are byte offsets into SourceLine.Text.
type SourceSymbol struct {
	Name      string
	Kind      SourceSymbolKind
	Line      int
	Statement int
	Column    int
	EndColumn int
	Docs      string
	Signature string
}

// SourceReference is an assembler source reference. Line and column values are
// zero-based, and columns are byte offsets into SourceLine.Text.
type SourceReference struct {
	Name      string
	Kind      SourceReferenceKind
	Line      int
	Column    int
	EndColumn int
}

// SourceRedundant describes a source statement that can be removed without
// changing the assembled program. Line and Statement values are zero-based.
// Columns are byte offsets into SourceLine.Text.
type SourceRedundant struct {
	Kind      SourceRedundantKind
	Line      int
	Statement int
	Column    int
	EndColumn int
	Name      string
	Message   string
}

// SourceIndex captures assembler-native source relationships for editor
// tooling. Positions are zero-based byte offsets, matching SourceLine and
// SourceToken.
type SourceIndex struct {
	Version           uint64
	VersionToken      *SourceToken
	Symbols           []SourceSymbol
	References        []SourceReference
	MissingReferences []SourceReference
	RefCounts         map[string]int
	Redundants        []SourceRedundant
}

type sourceIndexedStatement struct {
	line      int
	statement int
	label     *SourceToken
	labelName string
	op        *SourceToken
	args      []SourceToken
}

func sourceIndexForTools(lines []SourceLine, ops *OpStream) SourceIndex {
	idx := SourceIndex{
		Version:   sourceIndexVersion(ops),
		RefCounts: make(map[string]int),
	}

	var statements []sourceIndexedStatement
	labelSymbolIndexes := make(map[string][]int)
	var pendingDocs []string

	for _, line := range lines {
		if len(line.Tokens) == 0 {
			if line.Comment != nil {
				comment := strings.TrimSpace(line.Comment.Text)
				if comment != "" {
					pendingDocs = append(pendingDocs, comment)
				}
			} else {
				pendingDocs = nil
			}
			continue
		}

		for statementIndex, statement := range line.Statements {
			indexed := sourceIndexStatement(statement, statementIndex)
			statements = append(statements, indexed)

			tokens := statement.Tokens
			if len(tokens) == 0 {
				continue
			}

			if sourceStatementIsPragmaVersion(tokens) {
				if version, ok := sourceIndexParseVersion(tokens[2].Text); ok {
					idx.Version = version
					versionToken := tokens[2]
					idx.VersionToken = &versionToken
				}
				continue
			}

			if sourceStatementIsDefine(tokens) {
				name := tokens[1]
				idx.Symbols = append(idx.Symbols, SourceSymbol{
					Name:      name.Text,
					Kind:      SourceSymbolDefine,
					Line:      name.Line,
					Statement: statementIndex,
					Column:    name.Column,
					EndColumn: name.EndColumn,
					Docs:      strings.Join(pendingDocs, "\n"),
				})
				pendingDocs = nil
				continue
			}

			if indexed.label != nil {
				symbol := SourceSymbol{
					Name:      indexed.labelName,
					Kind:      SourceSymbolLabel,
					Line:      indexed.label.Line,
					Statement: indexed.statement,
					Column:    indexed.label.Column,
					EndColumn: indexed.label.EndColumn,
					Docs:      strings.Join(pendingDocs, "\n"),
				}
				labelSymbolIndexes[symbol.Name] = append(labelSymbolIndexes[symbol.Name], len(idx.Symbols))
				idx.Symbols = append(idx.Symbols, symbol)
				pendingDocs = nil
			}
		}

		if len(line.Tokens) > 0 {
			pendingDocs = nil
		}
	}

	lastLabelName := ""
	activeDefines := make(map[string]bool)
	for _, statement := range statements {
		if statement.labelName != "" {
			lastLabelName = statement.labelName
		}
		if statement.op == nil {
			continue
		}
		if statement.op.Text == "#define" {
			if len(statement.args) > 0 {
				activeDefines[statement.args[0].Text] = true
			}
			continue
		}
		if strings.HasPrefix(statement.op.Text, "#") {
			continue
		}

		if activeDefines[statement.op.Text] {
			idx.References = append(idx.References, sourceReferenceFromToken(*statement.op, SourceReferenceDefine))
			continue
		}

		for _, arg := range statement.args {
			if activeDefines[arg.Text] {
				idx.References = append(idx.References, sourceReferenceFromToken(arg, SourceReferenceDefine))
			}
		}

		for _, labelArg := range sourceIndexLabelArguments(*statement.op, statement.args) {
			if activeDefines[labelArg.Text] {
				continue
			}
			idx.References = append(idx.References, sourceReferenceFromToken(labelArg, SourceReferenceLabel))
		}

		if statement.op.Text == "proto" && len(statement.args) >= 2 {
			sourceIndexSetProtoSignature(idx.Symbols, labelSymbolIndexes, lastLabelName, statement)
		}
	}

	symbolsByName := make(map[string]bool)
	for _, symbol := range idx.Symbols {
		symbolsByName[symbol.Name] = true
	}
	for _, ref := range idx.References {
		idx.RefCounts[ref.Name]++
		if !symbolsByName[ref.Name] {
			idx.MissingReferences = append(idx.MissingReferences, ref)
		}
	}

	idx.Redundants = sourceIndexRedundants(statements, idx.Symbols, idx.RefCounts)
	return idx
}

func sourceIndexVersion(ops *OpStream) uint64 {
	if ops == nil || ops.Version == assemblerNoVersion {
		return AssemblerDefaultVersion
	}
	return ops.Version
}

func sourceStatementIsPragmaVersion(tokens []SourceToken) bool {
	return len(tokens) >= 3 && tokens[0].Text == "#pragma" && tokens[1].Text == "version"
}

func sourceStatementIsDefine(tokens []SourceToken) bool {
	return len(tokens) >= 2 && tokens[0].Text == "#define"
}

func sourceIndexParseVersion(text string) (uint64, bool) {
	version, err := strconv.ParseUint(text, 0, 64)
	return version, err == nil
}

func sourceIndexStatement(statement SourceStatement, statementIndex int) sourceIndexedStatement {
	indexed := sourceIndexedStatement{
		line:      statement.Line,
		statement: statementIndex,
	}

	tokens := statement.Tokens
	if len(tokens) == 0 {
		return indexed
	}

	if strings.HasSuffix(tokens[0].Text, ":") {
		labelName := strings.TrimSuffix(tokens[0].Text, ":")
		if labelName != "" {
			label := tokens[0]
			indexed.label = &label
			indexed.labelName = labelName
			tokens = tokens[1:]
		}
	}

	if len(tokens) > 0 {
		op := tokens[0]
		indexed.op = &op
		indexed.args = tokens[1:]
	}

	return indexed
}

func sourceReferenceFromToken(token SourceToken, kind SourceReferenceKind) SourceReference {
	return SourceReference{
		Name:      token.Text,
		Kind:      kind,
		Line:      token.Line,
		Column:    token.Column,
		EndColumn: token.EndColumn,
	}
}

func sourceIndexLabelArguments(op SourceToken, args []SourceToken) []SourceToken {
	spec, ok := sourceIndexSpec(op.Text, len(args))
	if !ok {
		return nil
	}

	var labels []SourceToken
	argIndex := 0
	for _, imm := range spec.Immediates {
		switch imm.kind {
		case immLabel:
			if argIndex < len(args) {
				labels = append(labels, args[argIndex])
			}
			argIndex++
		case immLabels:
			labels = append(labels, args[argIndex:]...)
			argIndex = len(args)
		default:
			argIndex++
		}
	}
	return labels
}

func sourceIndexSpec(name string, argCount int) (OpSpec, bool) {
	if pseudoSpecs, ok := pseudoOps[name]; ok {
		pseudo, ok := pseudoSpecs[argCount]
		if !ok {
			pseudo, ok = pseudoSpecs[anyImmediates]
		}
		if !ok {
			return OpSpec{}, false
		}
		if isFullSpec(pseudo) {
			return pseudo, true
		}
		spec, ok := OpsByName[AssemblerMaxVersion][pseudo.Name]
		return spec, ok
	}

	spec, ok := OpsByName[AssemblerMaxVersion][name]
	return spec, ok
}

func sourceIndexSetProtoSignature(symbols []SourceSymbol, labelIndexes map[string][]int, labelName string, statement sourceIndexedStatement) {
	if labelName == "" {
		return
	}

	args, err := strconv.ParseUint(statement.args[0].Text, 0, 64)
	if err != nil {
		return
	}
	results, err := strconv.ParseUint(statement.args[1].Text, 0, 64)
	if err != nil {
		return
	}

	for _, symbolIndex := range labelIndexes[labelName] {
		symbols[symbolIndex].Signature = fmt.Sprintf("in: %d, out: %d", args, results)
	}
}

func sourceIndexRedundants(statements []sourceIndexedStatement, symbols []SourceSymbol, refCounts map[string]int) []SourceRedundant {
	var redundants []SourceRedundant

	for _, symbol := range symbols {
		if symbol.Kind == SourceSymbolLabel && refCounts[symbol.Name] == 0 {
			redundants = append(redundants, SourceRedundant{
				Kind:      SourceRedundantUnusedLabel,
				Line:      symbol.Line,
				Statement: symbol.Statement,
				Column:    symbol.Column,
				EndColumn: symbol.EndColumn,
				Name:      symbol.Name,
				Message:   fmt.Sprintf("Remove label '%s'", symbol.Name),
			})
		}
	}

	for i, statement := range statements {
		if statement.op == nil || statement.op.Text != "b" || len(statement.args) != 1 {
			continue
		}
		if sourceIndexNextLabel(statements, i+1) == statement.args[0].Text {
			redundants = append(redundants, SourceRedundant{
				Kind:      SourceRedundantBranchToNextLabel,
				Line:      statement.op.Line,
				Statement: statement.statement,
				Column:    statement.op.Column,
				EndColumn: statement.op.EndColumn,
				Name:      statement.args[0].Text,
				Message:   "Remove b call",
			})
		}
	}

	return redundants
}

func sourceIndexNextLabel(statements []sourceIndexedStatement, start int) string {
	for i := start; i < len(statements); i++ {
		statement := statements[i]
		if statement.labelName != "" {
			return statement.labelName
		}
		if statement.op == nil || statement.op.Text == "nop" {
			continue
		}
		return ""
	}
	return ""
}
