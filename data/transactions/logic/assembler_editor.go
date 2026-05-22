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

// SourceHover is an editor-neutral hover response.
type SourceHover struct {
	Text string
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

// SourceHoverForTools returns source-aware hover text at a byte-column source
// position.
func SourceHoverForTools(result SourceAnalysisResult, line int, column int) (SourceHover, bool) {
	op, ok := SourceOperationAtForTools(result.Lines, result.Program, line, column)
	if !ok {
		return SourceHover{}, false
	}
	if column >= op.Token.Column && column <= op.Token.EndColumn {
		meta, ok := ToolOpcodeForTools(op.Name, len(op.Args), result.Mode)
		if ok {
			text := sourceFullDoc(meta.Docs, meta.ExtraDocs)
			if text != "" {
				return SourceHover{Text: text}, true
			}
		}
	}
	for _, arg := range op.Args {
		if column >= arg.Token.Column && column <= arg.Token.EndColumn && arg.Docs != "" {
			name := arg.ValueName
			if name == "" {
				name = arg.Token.Text
			}
			return SourceHover{Text: fmt.Sprintf("%s = %d\r\n%s", name, arg.Value, arg.Docs)}, true
		}
	}
	return SourceHover{}, false
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
		Docs:            sourceFullDoc(info.Docs, info.ExtraDocs),
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

func sourceFullDoc(short string, extra string) string {
	if extra == "" {
		return short
	}
	if short == "" {
		return extra
	}
	return fmt.Sprintf("%s\r\n\r\n%s", short, extra)
}
