package lsp

import (
	"fmt"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

type Subline struct {
	Tokens []Token
}

type Line struct {
	Tokens []Token
	Subs   []Subline
}

func (ln Line) SublineBegin(i int) int {
	for j := i; j >= 0; j-- {
		s := ln.Subs[j]
		if len(s.Tokens) > 0 {
			return s.Tokens[0].b
		}
	}
	return 0
}

func (ln Line) SublineEnd(i int) int {
	for j := i; j >= 0; j-- {
		s := ln.Subs[j]
		if len(s.Tokens) > 0 {
			return s.Tokens[len(s.Tokens)-1].e
		}
	}
	return 0
}

func (ln Line) SublineByIndex(i int) Subline {
	if len(ln.Subs) == 0 {
		return Subline{}
	}

	fb := ln.SublineBegin(0)
	if i <= fb {
		return ln.Subs[0]
	}

	le := ln.SublineEnd(len(ln.Subs) - 1)
	if i >= le {
		return ln.Subs[len(ln.Subs)-1]
	}

	for sli, sub := range ln.Subs {
		b := ln.SublineBegin(sli)
		e := ln.SublineEnd(sli)

		if i >= b && i <= e {
			return sub
		}
	}

	return Subline{}
}

func (ln Line) Begin() int {
	switch len(ln.Tokens) {
	case 0:
		return 0
	default:
		return ln.Tokens[0].b
	}
}

func (ln Line) End() int {
	switch len(ln.Tokens) {
	case 0:
		return 0
	default:
		return ln.Tokens[len(ln.Tokens)-1].e
	}
}

func (sln Subline) ImmAt(pos int) (Token, int, bool) {
	for idx, tok := range sln.Tokens {
		if idx > 0 && pos >= tok.Begin() && pos <= tok.End() {
			return tok, idx - 1, true
		}
	}

	return Token{}, 0, false
}

type RequiredVersion struct {
	Line  int
	Begin int
	End   int

	Version uint64
}

func (v RequiredVersion) StartLine() int {
	return v.Line
}

func (v RequiredVersion) EndLine() int {
	return v.Line
}

func (v RequiredVersion) StartCharacter() int {
	return v.Begin
}

func (v RequiredVersion) EndCharacter() int {
	return v.End
}

type ProcessResult struct {
	Mode logic.RunMode

	Version      uint64
	VersionToken *Token
	Versions     []RequiredVersion

	MissRefs   []Token
	Symbols    []Symbol
	SymbolRefs []Token

	Tokens []Token
	Lines  []Line

	Ops []Token

	Bools    []Token
	Numbers  []Token
	Strings  []Token
	Keywords []Token
	Macros   []Token

	Redundants []RedundantLine

	RefCounts map[string]int
	Defines   map[string]bool

	SourceLines          []logic.SourceLine
	SourceProgram        logic.SourceProgram
	AssemblerDiagnostics []logic.SourceDiagnostic
	OpStream             *logic.OpStream
	AssembleError        error
}

func (r ProcessResult) AvailableOps() []logic.ToolOpcode {
	return logic.ToolOpcodesForTools(r.Version, r.Mode)
}

func (r ProcessResult) SymbolsForRefWithin(rg Range) []Symbol {
	var res []Symbol

	refs := r.SymbolRefsWithin(rg)

	if len(refs) > 0 {
		ref := refs[0]

		for _, sym := range r.Symbols {
			if sym.Name() == ref.String() {
				res = append(res, sym)
			}
		}
	}

	return res
}

func (r ProcessResult) SymbolsWithin(rg Range) []Symbol {
	var res []Symbol

	for _, sym := range r.Symbols {
		if Overlaps(rg, sym) {
			res = append(res, sym)
		}
	}

	return res
}

func (r ProcessResult) SymbolRefsWithin(rg Range) []Token {
	var res []Token

	for _, ref := range r.SymbolRefs {
		if Overlaps(rg, ref) {
			res = append(res, ref)
		}
	}

	return res
}

type NamedInlayHint struct {
	T    Token
	Name string
}

type DecodedInlayHint struct {
	T     Token
	Value string
}

type InlayHints struct {
	Named   []NamedInlayHint
	Decoded []DecodedInlayHint
}

type InlayHint struct {
	Line      int
	Character int
	Label     string
}

func (r ProcessResult) InlayHints(rg Range) InlayHints {
	var ihs InlayHints

	for _, op := range r.SourceProgram.Operations {
		for _, arg := range op.Args {
			tok, ok := tokenFromSourceArgument(r.SourceLines, arg)
			if !ok || !Overlaps(tok, rg) {
				continue
			}
			if arg.HasValue && arg.ValueName != "" && arg.Token.Text != arg.ValueName {
				ihs.Named = append(ihs.Named, NamedInlayHint{
					T:    tok,
					Name: arg.ValueName,
				})
			}
			if decoded, ok := logic.DecodedHexStringForTools(tok.String()); ok {
				ihs.Decoded = append(ihs.Decoded, DecodedInlayHint{
					T:     tok,
					Value: decoded,
				})
			}
		}
	}

	for _, tok := range r.Tokens {
		if !Overlaps(tok, rg) {
			continue
		}
		if decoded, ok := logic.DecodedHexStringForTools(tok.String()); ok {
			already := false
			for _, existing := range ihs.Decoded {
				if existing.T.Line() == tok.Line() && existing.T.Begin() == tok.Begin() {
					already = true
					break
				}
			}
			if !already {
				ihs.Decoded = append(ihs.Decoded, DecodedInlayHint{
					T:     tok,
					Value: decoded,
				})
			}
		}
	}

	return ihs
}

func (r ProcessResult) ArgVals(arg logic.ToolArg) []logic.ToolArgValue {
	if arg.Kind == logic.ToolArgLabel {
		var res []logic.ToolArgValue
		for _, sym := range r.Symbols {
			res = append(res, logic.ToolArgValue{
				NoValue:   true,
				Name:      sym.Name(),
				Docs:      sym.Docs(),
				Signature: sym.Signature(),
			})
		}
		return res
	}

	return logic.ToolArgValuesForTools(arg.Kind, arg.FieldGroup, r.Version, r.Mode)
}

func (r ProcessResult) SymByName(name string) []Symbol {
	var res []Symbol
	for _, sym := range r.Symbols {
		if sym.Name() == name {
			res = append(res, sym)
		}
	}
	return res
}

func (r ProcessResult) SymRefByName(name string) []Token {
	var res []Token
	for _, sym := range r.SymbolRefs {
		if sym.String() == name {
			res = append(res, sym)
		}
	}
	return res
}

func (r ProcessResult) SymOrRefAt(rg Range) string {
	for _, sym := range r.Symbols {
		if Overlaps(rg, sym) {
			return sym.Name()
		}
	}

	for _, ref := range r.SymbolRefs {
		if Overlaps(rg, ref) {
			return ref.String()
		}
	}

	return ""
}

func (r ProcessResult) ArgAt(l int, ch int) (logic.ToolArg, int, bool) {
	var res logic.ToolArg
	op, ok := r.OperationAt(l, ch)
	if !ok {
		return res, -1, false
	}
	for idx, arg := range op.Args {
		tok, ok := tokenFromSourceArgument(r.SourceLines, arg)
		if ok && ch >= tok.Begin() && ch <= tok.End() {
			return arg.ToolArg, idx, true
		}
	}
	if len(op.ToolArgs) == 0 {
		return res, -1, false
	}
	idx := len(op.Args)
	if idx >= len(op.ToolArgs) {
		idx = len(op.ToolArgs) - 1
	}
	return op.ToolArgs[idx], idx, true
}

func (r ProcessResult) ArgValsAt(l int, ch int) []logic.ToolArgValue {
	arg, _, ok := r.ArgAt(l, ch)
	if !ok {
		return nil
	}

	return r.ArgVals(arg)
}

func (r ProcessResult) DocAt(l int, ch int) string {
	op, ok := r.OperationAt(l, ch)
	if !ok {
		return ""
	}
	opToken, ok := tokenFromSourceOperation(r.SourceLines, op)
	if ok && ch >= opToken.Begin() && ch <= opToken.End() {
		meta, ok := logic.ToolOpcodeForTools(op.Name, len(op.Args), r.Mode)
		if ok {
			return MakeFullDoc(meta.Docs, meta.ExtraDocs)
		}
	}
	for _, arg := range op.Args {
		tok, ok := tokenFromSourceArgument(r.SourceLines, arg)
		if ok && ch >= tok.Begin() && ch <= tok.End() && arg.Docs != "" {
			name := arg.ValueName
			if name == "" {
				name = tok.String()
			}
			return fmt.Sprintf("%s = %d\r\n%s", name, arg.Value, arg.Docs)
		}
	}
	return ""
}

func (r ProcessResult) OperationAt(l int, ch int) (logic.SourceOperation, bool) {
	for _, op := range r.SourceProgram.Operations {
		if op.Token.Line != l || op.Token.Line < 0 || op.Token.Line >= len(r.SourceLines) {
			continue
		}
		start := utf16ColumnFromByte(r.SourceLines[l].Text, op.Token.Column)
		end := utf16ColumnFromByte(r.SourceLines[l].Text, op.EndColumn)
		if ch >= start && ch <= end+1 {
			return op, true
		}
	}
	return logic.SourceOperation{}, false
}

func readAnalyzedSourceLines(sourceLines []logic.SourceLine) ([]Token, []Line) {
	tokens := make([]Token, 0)
	lines := make([]Line, 0, len(sourceLines))

	for _, sourceLine := range sourceLines {
		line := Line{
			Tokens: make([]Token, 0, len(sourceLine.Tokens)),
			Subs:   make([]Subline, 0, len(sourceLine.Statements)),
		}

		for _, sourceToken := range sourceLine.Tokens {
			token := tokenFromSourceToken(sourceLine.Text, sourceToken)
			line.Tokens = append(line.Tokens, token)
			tokens = append(tokens, token)
		}

		if sourceLine.Comment != nil {
			tokens = append(tokens, tokenFromSourceToken(sourceLine.Text, *sourceLine.Comment))
		}

		for _, statement := range sourceLine.Statements {
			subline := Subline{
				Tokens: make([]Token, 0, len(statement.Tokens)),
			}
			for _, sourceToken := range statement.Tokens {
				subline.Tokens = append(subline.Tokens, tokenFromSourceToken(sourceLine.Text, sourceToken))
			}
			line.Subs = append(line.Subs, subline)
		}

		if len(line.Subs) == 0 {
			line.Subs = append(line.Subs, Subline{})
		}

		lines = append(lines, line)
	}

	return tokens, lines
}

func tokenFromSourceToken(line string, sourceToken logic.SourceToken) Token {
	return Token{
		v: sourceToken.Text,
		l: sourceToken.Line,
		b: utf16ColumnFromByte(line, sourceToken.Column),
		e: utf16ColumnFromByte(line, sourceToken.EndColumn),
		t: tokenTypeFromSourceToken(sourceToken.Kind),
	}
}

func tokenTypeFromSourceToken(kind logic.SourceTokenKind) TokenType {
	switch kind {
	case logic.SourceTokenSemicolon:
		return TokenSemicolon
	case logic.SourceTokenComment:
		return TokenComment
	default:
		return TokenValue
	}
}

func utf16ColumnFromByte(line string, column int) int {
	if column < 0 {
		column = 0
	}
	if column > len(line) {
		column = len(line)
	}
	return utf16LenString(line[:column])
}

func MakeFullDoc(short string, extra string) string {
	if extra == "" {
		return short
	}
	if short == "" {
		return extra
	}
	return fmt.Sprintf("%s\r\n\r\n%s", short, extra)
}

func Process(source string) *ProcessResult {
	initialLines := logic.SourceLinesForTools(source)
	mode := sourceModeForLSP(initialLines)
	analysis := logic.AnalyzeSourceForToolsWithOptions(source, logic.SourceToolOptions{
		Mode: mode,
	})
	ts, lines := readAnalyzedSourceLines(analysis.Lines)

	result := &ProcessResult{
		Mode:                 mode,
		Version:              analysis.Index.Version,
		VersionToken:         versionTokenFromSourceIndex(analysis.Lines, analysis.Index),
		MissRefs:             referencesFromSourceIndex(analysis.Lines, analysis.Index.MissingReferences),
		Symbols:              symbolsFromSourceIndex(analysis.Lines, analysis.Index),
		SymbolRefs:           referencesFromSourceIndex(analysis.Lines, analysis.Index.References),
		Tokens:               ts,
		Lines:                lines,
		Ops:                  operationTokensFromSourceProgram(analysis.Lines, analysis.Program),
		Numbers:              tokensOfSourceClass(analysis.Lines, analysis.Program, logic.SourceTokenClassNumber),
		Bools:                tokensOfSourceClass(analysis.Lines, analysis.Program, logic.SourceTokenClassBool),
		Strings:              tokensOfSourceClass(analysis.Lines, analysis.Program, logic.SourceTokenClassString),
		Keywords:             tokensOfSourceClass(analysis.Lines, analysis.Program, logic.SourceTokenClassKeyword),
		Macros:               tokensOfSourceClass(analysis.Lines, analysis.Program, logic.SourceTokenClassMacro),
		Redundants:           redundantsFromSourceIndex(analysis.Index),
		Versions:             requiredVersionsFromSourceProgram(analysis.Lines, analysis.Program),
		RefCounts:            analysis.Index.RefCounts,
		Defines:              definesFromSourceIndex(analysis.Index),
		SourceLines:          analysis.Lines,
		SourceProgram:        analysis.Program,
		AssemblerDiagnostics: analysis.Diagnostics,
		OpStream:             analysis.OpStream,
		AssembleError:        analysis.Err,
	}

	return result
}
