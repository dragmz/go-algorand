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

// SourceCompletionItemKind classifies a source completion item.
type SourceCompletionItemKind int

const (
	SourceCompletionItemOpcode SourceCompletionItemKind = iota
	SourceCompletionItemArgument
	SourceCompletionItemDefine
)

// SourceCompletionItem is an editor-neutral completion candidate.
type SourceCompletionItem struct {
	Kind          SourceCompletionItemKind
	Label         string
	Docs          string
	Detail        string
	Version       uint64
	Args          []ToolArg
	ArgsSignature string
	HasValue      bool
	Value         uint64
	Signature     string
}

// SourceHover is an editor-neutral hover response. Text is markdown, and Range
// is the source range the text describes.
type SourceHover struct {
	Text  string
	Range SourceRange
}

// SourceSignatureHelp is editor-neutral signature help.
type SourceSignatureHelp struct {
	Label           string
	Docs            string
	Parameters      []string
	ActiveParameter int
}

// SourceCompletionsForTools returns source-aware completion candidates at a
// byte-column source position.
func SourceCompletionsForTools(result SourceAnalysisResult, line int, column int) []SourceCompletionItem {
	ctx := SourceCompletionContextForTools(result.Lines, result.Program, line, column)
	switch ctx.Mode {
	case SourceCompletionArgument:
		arg, _, ok := SourceToolArgAtForTools(result.Lines, result.Program, line, column)
		if !ok {
			return nil
		}
		return sourceArgumentCompletionsForTools(result, arg)
	default:
		return sourceOpcodeCompletionsForTools(result, ctx.Prefix)
	}
}

// sourceHoverResolvers answer a hover in order, and the first one that matches
// wins. Operations are asked before identifiers so that hovering the mnemonic of
// a branch documents the opcode rather than the label it names.
var sourceHoverResolvers = []func(SourceAnalysisResult, int, int) (SourceHover, bool){
	sourceHoverOperation,
	sourceHoverArgument,
	sourceHoverIdentifier,
}

// SourceHoverForTools returns source-aware hover text at a byte-column source
// position.
func SourceHoverForTools(result SourceAnalysisResult, line int, column int) (SourceHover, bool) {
	for _, resolve := range sourceHoverResolvers {
		if hover, ok := resolve(result, line, column); ok {
			return hover, true
		}
	}
	return SourceHover{}, false
}

// sourceHoverIn pairs hover text with the range it describes. An empty document
// reports false, so no resolver can answer with a blank hover.
func sourceHoverIn(rg SourceRange, text string) (SourceHover, bool) {
	if text == "" {
		return SourceHover{}, false
	}
	return SourceHover{Text: text, Range: rg}, true
}

// sourceHoverOperation documents the opcode or directive under the cursor.
func sourceHoverOperation(result SourceAnalysisResult, line int, column int) (SourceHover, bool) {
	op, ok := SourceOperationAtForTools(result.Lines, result.Program, line, column)
	if !ok || !sourceTokenContains(op.Token, column) {
		return SourceHover{}, false
	}
	meta, ok := ToolOpcodeForTools(op.Name, len(op.Args), result.Mode)
	if !ok {
		return SourceHover{}, false
	}
	return sourceHoverIn(sourceRangeFromToken(op.Token), sourceMarkdownDoc(meta.Docs, meta.ExtraDocs))
}

// sourceHoverArgument documents the named value an argument selects. It resolves
// the operation again rather than sharing it with sourceHoverOperation, which
// keeps each resolver answerable on its own; hover is a user-driven request, so
// the repeated lookup does not matter.
func sourceHoverArgument(result SourceAnalysisResult, line int, column int) (SourceHover, bool) {
	op, ok := SourceOperationAtForTools(result.Lines, result.Program, line, column)
	if !ok {
		return SourceHover{}, false
	}
	for _, arg := range op.Args {
		if !arg.HasValue || !sourceTokenContains(arg.Token, column) {
			continue
		}
		name := arg.ValueName
		if name == "" {
			name = arg.Token.Text
		}
		return sourceHoverIn(sourceRangeFromToken(arg.Token),
			sourceMarkdownDoc(fmt.Sprintf("`%s` = %d", name, arg.Value), arg.Docs))
	}
	return SourceHover{}, false
}

// sourceHoverIdentifier documents the label or macro under the cursor, both at
// its definition and at every reference to it.
func sourceHoverIdentifier(result SourceAnalysisResult, line int, column int) (SourceHover, bool) {
	identifier, ok := SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return SourceHover{}, false
	}
	rg, ok := SourceIdentifierRangeForTools(identifier)
	if !ok {
		return SourceHover{}, false
	}
	symbols := SourceSymbolsByNameForTools(result.Index, identifier.Name)
	if len(symbols) == 0 {
		return SourceHover{}, false
	}
	symbol := symbols[0]
	return sourceHoverIn(rg, sourceMarkdownDoc(
		sourceSymbolDeclaration(symbol),
		symbol.Docs,
		sourceSymbolReferences(result.Index.RefCounts[symbol.Name]),
	))
}

// sourceSymbolDeclaration renders the line naming a symbol, which is the only
// place a hover over a label or macro says what the name is.
func sourceSymbolDeclaration(symbol SourceSymbol) string {
	declaration := symbol.Name + ":"
	if symbol.Kind == SourceSymbolDefine {
		declaration = "#define " + symbol.Name
	}
	if symbol.Signature != "" {
		return fmt.Sprintf("`%s` %s", declaration, symbol.Signature)
	}
	return fmt.Sprintf("`%s`", declaration)
}

// sourceSymbolReferences renders how many references a symbol has, and nothing
// at all for a symbol nothing refers to.
func sourceSymbolReferences(count int) string {
	switch count {
	case 0:
		return ""
	case 1:
		return "1 reference"
	default:
		return fmt.Sprintf("%d references", count)
	}
}

// SourceSignatureHelpForTools returns source-aware signature help at a
// byte-column source position.
func SourceSignatureHelpForTools(result SourceAnalysisResult, line int, column int) (SourceSignatureHelp, bool) {
	op, ok := SourceOperationAtForTools(result.Lines, result.Program, line, column)
	if !ok {
		return SourceSignatureHelp{}, false
	}
	info, ok := ToolOpcodeForTools(op.Name, len(op.Args), result.Mode)
	if !ok {
		return SourceSignatureHelp{}, false
	}
	_, active, ok := SourceToolArgAtForTools(result.Lines, result.Program, line, column)
	if !ok {
		active = -1
	}
	help := SourceSignatureHelp{
		Label:           info.FullSignature,
		Docs:            sourceMarkdownDoc(info.Docs, info.ExtraDocs),
		ActiveParameter: active,
	}
	for _, arg := range info.Args {
		help.Parameters = append(help.Parameters, arg.Name)
	}
	return help, true
}

func sourceArgumentCompletionsForTools(result SourceAnalysisResult, arg ToolArg) []SourceCompletionItem {
	values := sourceArgumentValuesForTools(result, arg)
	items := make([]SourceCompletionItem, 0, len(values))
	for _, value := range values {
		items = append(items, SourceCompletionItem{
			Kind:      SourceCompletionItemArgument,
			Label:     value.Name,
			Docs:      value.Docs,
			HasValue:  !value.NoValue,
			Value:     value.Value,
			Signature: value.Signature,
		})
	}
	return items
}

func sourceArgumentValuesForTools(result SourceAnalysisResult, arg ToolArg) []ToolArgValue {
	if arg.Kind == ToolArgLabel {
		values := make([]ToolArgValue, 0, len(result.Index.Symbols))
		for _, sym := range result.Index.Symbols {
			values = append(values, ToolArgValue{
				NoValue:   true,
				Name:      sym.Name,
				Docs:      sym.Docs,
				Signature: sym.Signature,
			})
		}
		return values
	}
	return ToolArgValuesForTools(arg.Kind, arg.FieldGroup, result.Index.Version, result.Mode)
}

func sourceOpcodeCompletionsForTools(result SourceAnalysisResult, prefix string) []SourceCompletionItem {
	var items []SourceCompletionItem

	for _, name := range SourceDefinedNamesForTools(result.Index) {
		items = append(items, SourceCompletionItem{
			Kind:  SourceCompletionItemDefine,
			Label: name,
		})
	}

	for _, info := range ToolOpcodesForTools(result.Index.Version, result.Mode) {
		if !strings.HasPrefix(info.Name, prefix) {
			continue
		}
		items = append(items, SourceCompletionItem{
			Kind:          SourceCompletionItemOpcode,
			Label:         info.Name,
			Docs:          info.Docs,
			Version:       info.Version,
			Args:          info.Args,
			ArgsSignature: info.ArgsSignature,
		})
	}
	return items
}

// sourceMarkdownDoc joins the sections of a hover or signature document.
// Sections are separated by a blank line, the only separator markdown renders as
// a break, so no caller can accidentally run two sections onto one line. Empty
// sections drop out, which lets callers pass optional parts unconditionally.
func sourceMarkdownDoc(sections ...string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if section != "" {
			parts = append(parts, section)
		}
	}
	return strings.Join(parts, "\r\n\r\n")
}
