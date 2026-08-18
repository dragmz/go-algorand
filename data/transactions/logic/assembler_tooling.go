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
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// SourceToolOptions controls source analysis that depends on editor context
// rather than assembler semantics.
type SourceToolOptions struct {
	Mode       RunMode
	Version    uint64
	UseVersion bool
}

// ToolArgKind classifies source-level opcode arguments for tooling.
type ToolArgKind int

const (
	ToolArgNone ToolArgKind = iota
	ToolArgConstInt
	ToolArgBool
	ToolArgUint64
	ToolArgUint8
	ToolArgInt8
	ToolArgBytes
	ToolArgField
	ToolArgLabel
	ToolArgSignature
	ToolArgAddress
	ToolArgPragmaName
	ToolArgDefineName
)

// ToolFieldGroup identifies named immediate value groups.
// ToolFieldGroup is the assembler field group an argument selects a value
// from. It is the assembler's own group rather than a parallel enumeration, so
// a field group added upstream is picked up here without any change.
type ToolFieldGroup = *FieldGroup

// ToolFieldNone marks an argument that does not select from a field group.
var ToolFieldNone ToolFieldGroup

// ToolArg is a source-level argument accepted by an opcode or assembler
// directive.
type ToolArg struct {
	Name       string
	Kind       ToolArgKind
	FieldGroup ToolFieldGroup
	Array      bool
	Optional   bool
}

// ToolOpcode is public, read-only opcode metadata for editor tooling.
type ToolOpcode struct {
	Name          string
	CanonicalName string
	Version       uint64
	Modes         RunMode
	Args          []ToolArg
	ArgsSignature string
	FullSignature string
	Docs          string
	ExtraDocs     string
}

// ToolArgValue is a named or numeric value for an opcode argument.
type ToolArgValue struct {
	NoValue   bool
	Value     uint64
	Name      string
	Docs      string
	Signature string
	Version   uint64
}

// SourceTokenClassKind classifies source tokens for semantic editor features.
type SourceTokenClassKind int

const (
	SourceTokenClassOpcode SourceTokenClassKind = iota
	SourceTokenClassMacro
	SourceTokenClassBool
	SourceTokenClassNumber
	SourceTokenClassString
	SourceTokenClassKeyword
)

// SourceTokenClass is a classified assembler token.
type SourceTokenClass struct {
	Kind  SourceTokenClassKind
	Token SourceToken
}

// SourceArgument is a concrete source argument with its tooling role.
type SourceArgument struct {
	ToolArg
	Token     SourceToken
	HasValue  bool
	Value     uint64
	ValueName string
	Docs      string
}

// SourceOperation is a source opcode/directive occurrence.
type SourceOperation struct {
	Name            string
	CanonicalName   string
	Token           SourceToken
	Statement       int
	EndColumn       int
	ToolArgs        []ToolArg
	Args            []SourceArgument
	RequiredVersion uint64
}

// SourceRequiredVersion reports a source range that requires a newer TEAL
// version for the selected mode.
type SourceRequiredVersion struct {
	Line      int
	Column    int
	EndColumn int
	Version   uint64
}

// SourceProgram is source-level program structure derived from assembler
// metadata.
type SourceProgram struct {
	Operations       []SourceOperation
	RequiredVersions []SourceRequiredVersion
	TokenClasses     []SourceTokenClass
}

// SourceCompletionMode identifies whether completion should offer opcodes or
// arguments at a source position.
type SourceCompletionMode int

const (
	SourceCompletionOpcode SourceCompletionMode = iota
	SourceCompletionArgument
)

// SourceCompletionContext describes editor completion at a byte-column source
// position.
type SourceCompletionContext struct {
	Mode           SourceCompletionMode
	Prefix         string
	Statement      SourceStatement
	StatementIndex int
}

// SourceInlayHintKind classifies assembler-native inlay hints.
type SourceInlayHintKind int

const (
	SourceInlayHintNamed SourceInlayHintKind = iota
	SourceInlayHintDecoded
)

// SourceInlayHint is an editor hint tied to a source token.
type SourceInlayHint struct {
	Kind  SourceInlayHintKind
	Token SourceToken
	Label string
}

type toolOpcodeOverlay struct {
	args    []ToolArg
	version uint64
	modes   RunMode
}

var toolSourceOverlays = map[string]toolOpcodeOverlay{
	"#pragma": {args: []ToolArg{{Name: "name", Kind: ToolArgPragmaName}, {Name: "value", Kind: ToolArgUint64, Optional: true}}, version: 1, modes: modeAny},
	"#define": {args: []ToolArg{{Name: "name", Kind: ToolArgDefineName}, {Name: "value", Kind: ToolArgNone, Array: true}}, version: 1, modes: modeAny},
	"addr":    {args: []ToolArg{{Name: "address", Kind: ToolArgAddress}}, version: 1, modes: modeAny},
	"byte":    {args: []ToolArg{{Name: "value", Kind: ToolArgBytes}}, version: 1, modes: modeAny},
	"int":     {args: []ToolArg{{Name: "value", Kind: ToolArgConstInt}}, version: 1, modes: modeAny},
	"method":  {args: []ToolArg{{Name: "signature", Kind: ToolArgSignature}}, version: 1, modes: modeAny},
	"txn":     {args: []ToolArg{{Name: "f", Kind: ToolArgField, FieldGroup: &TxnFields}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 1, modes: modeAny},
	"gtxn":    {args: []ToolArg{{Name: "t", Kind: ToolArgUint8}, {Name: "f", Kind: ToolArgField, FieldGroup: &TxnFields}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 1, modes: modeAny},
	"gtxns":   {args: []ToolArg{{Name: "f", Kind: ToolArgField, FieldGroup: &TxnFields}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 3, modes: modeAny},
	"extract": {args: []ToolArg{{Name: "s", Kind: ToolArgUint8, Optional: true}, {Name: "l", Kind: ToolArgUint8, Optional: true}}, version: 5, modes: modeAny},
	"replace": {args: []ToolArg{{Name: "s", Kind: ToolArgUint8, Optional: true}}, version: 7, modes: modeAny},
}

type toolOpcodeSetKey struct {
	version uint64
	mode    RunMode
}

// toolOpcodeSets memoizes ToolOpcodesForTools. Editors ask for the same
// (version, mode) pair on every completion request, and the answer only
// depends on assembler tables that are fixed after package init.
var toolOpcodeSets sync.Map // toolOpcodeSetKey -> []ToolOpcode

// ToolOpcodesForTools returns source-level opcode metadata available at the
// requested version and mode.
//
// The result is cached and shared between callers, so it must be treated as
// read only.
func ToolOpcodesForTools(version uint64, mode RunMode) []ToolOpcode {
	if mode == 0 {
		mode = ModeApp
	}
	if version == 0 {
		version = AssemblerDefaultVersion
	}
	if version > AssemblerMaxVersion {
		// No opcode is filtered out beyond the newest assembler version, so
		// every higher version selects the same set. Clamping keeps a source
		// with an out of range "#pragma version" from growing the cache.
		version = AssemblerMaxVersion
	}

	key := toolOpcodeSetKey{version: version, mode: mode}
	if cached, ok := toolOpcodeSets.Load(key); ok {
		return cached.([]ToolOpcode)
	}

	names := toolOpcodeNames()
	ops := make([]ToolOpcode, 0, len(names))
	for _, name := range names {
		op, ok := toolOpcodeForSource(name, 0, mode)
		if !ok || op.Version == 0 || op.Version > version {
			continue
		}
		ops = append(ops, op)
	}

	cached, _ := toolOpcodeSets.LoadOrStore(key, ops)
	return cached.([]ToolOpcode)
}

// ToolOpcodeForTools returns source-level opcode metadata for one source
// mnemonic. argCount selects pseudo-op forms whose bytecode opcode depends on
// immediate count.
func ToolOpcodeForTools(name string, argCount int, mode RunMode) (ToolOpcode, bool) {
	if mode == 0 {
		mode = ModeApp
	}
	return toolOpcodeForSource(name, argCount, mode)
}

// toolOpcodeNames returns the sorted set of source mnemonics. The assembler
// tables it reads are fixed after package init, so it is built once.
//
// The result is shared between callers and must be treated as read only.
var toolOpcodeNames = sync.OnceValue(buildToolOpcodeNames)

func buildToolOpcodeNames() []string {
	seen := make(map[string]bool)
	for name := range OpsByName[AssemblerMaxVersion] {
		seen[name] = true
	}
	for name := range pseudoOps {
		seen[name] = true
	}
	for name := range toolSourceOverlays {
		if strings.HasPrefix(name, "#") {
			continue
		}
		seen[name] = true
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type toolOpcodeKey struct {
	name     string
	argCount int
	mode     RunMode
}

// toolOpcodes memoizes toolOpcodeForSource, which rebuilds a mnemonic's
// argument list and signature strings once per operation in a document.
//
// Only successful lookups are cached. Mnemonics come from source text, and
// caching misses would let a document full of unknown words grow the cache
// without bound, while every hit is a name that exists in the assembler
// tables.
var toolOpcodes sync.Map // toolOpcodeKey -> ToolOpcode

// toolOpcodeArgCountKey collapses argument counts that select the same spec.
// Only pseudo-ops choose a spec by immediate count, and they fall back to
// anyImmediates for counts they do not name, so everything else shares one
// cache entry.
func toolOpcodeArgCountKey(name string, argCount int) int {
	specs, ok := pseudoOps[name]
	if !ok {
		return anyImmediates
	}
	if _, ok := specs[argCount]; ok {
		return argCount
	}
	return anyImmediates
}

func toolOpcodeForSource(name string, argCount int, mode RunMode) (ToolOpcode, bool) {
	key := toolOpcodeKey{
		name:     name,
		argCount: toolOpcodeArgCountKey(name, argCount),
		mode:     mode,
	}
	if cached, ok := toolOpcodes.Load(key); ok {
		return cached.(ToolOpcode), true
	}

	op, ok := buildToolOpcodeForSource(name, argCount, mode)
	if !ok {
		return ToolOpcode{}, false
	}

	cached, _ := toolOpcodes.LoadOrStore(key, op)
	return cached.(ToolOpcode), true
}

func buildToolOpcodeForSource(name string, argCount int, mode RunMode) (ToolOpcode, bool) {
	if overlay, ok := toolSourceOverlays[name]; ok {
		canonical := toolCanonicalName(name, argCount)
		return toolOpcodeFromParts(name, canonical, overlay.version, overlay.modes, overlay.args, OpDescOf(canonical)), true
	}

	spec, ok := sourceIndexSpec(name, argCount)
	if !ok {
		return ToolOpcode{}, false
	}
	version := toolOpcodeVersion(name, mode)
	if version == 0 {
		return ToolOpcode{}, false
	}
	args := toolArgsFromSpec(spec)
	desc := OpDescOf(spec.Name)
	return toolOpcodeFromParts(name, spec.Name, version, spec.Modes, args, desc), true
}

func toolOpcodeFromParts(name, canonical string, version uint64, modes RunMode, args []ToolArg, desc OpDesc) ToolOpcode {
	return ToolOpcode{
		Name:          name,
		CanonicalName: canonical,
		Version:       version,
		Modes:         modes,
		Args:          args,
		ArgsSignature: toolArgsSignature(args, false),
		FullSignature: toolFullSignature(name, args),
		Docs:          desc.Short,
		ExtraDocs:     desc.Extra,
	}
}

func toolCanonicalName(name string, argCount int) string {
	if name == "#pragma" || name == "#define" {
		return name
	}
	spec, ok := sourceIndexSpec(name, argCount)
	if ok {
		return spec.Name
	}
	return name
}

func toolOpcodeVersion(name string, mode RunMode) uint64 {
	for v := uint64(1); v <= AssemblerMaxVersion; v++ {
		spec, ok := OpsByName[v][name]
		if !ok {
			if pseudoSpecs, ok := pseudoOps[name]; ok {
				spec, ok = toolPseudoSpecForVersion(pseudoSpecs, v)
			}
		}
		if ok && spec.Modes&mode != 0 {
			return v
		}
	}
	return 0
}

func toolPseudoSpecForVersion(specs map[int]OpSpec, version uint64) (OpSpec, bool) {
	if spec, ok := specs[anyImmediates]; ok && isFullSpec(spec) {
		spec.Version = version
		return spec, true
	}
	for _, pseudo := range specs {
		if isFullSpec(pseudo) {
			pseudo.Version = version
			return pseudo, true
		}
		spec, ok := OpsByName[version][pseudo.Name]
		if ok {
			return spec, true
		}
	}
	return OpSpec{}, false
}

func toolArgsFromSpec(spec OpSpec) []ToolArg {
	args := make([]ToolArg, 0, len(spec.Immediates))
	for _, imm := range spec.Immediates {
		arg := ToolArg{
			Name:       strings.TrimSuffix(imm.Name, " ..."),
			Kind:       toolArgKindFromImmediate(imm),
			FieldGroup: toolFieldGroupFromImmediate(imm),
			Array:      imm.kind == immInts || imm.kind == immBytess || imm.kind == immLabels,
		}
		args = append(args, arg)
	}
	return args
}

func toolArgKindFromImmediate(imm immediate) ToolArgKind {
	if imm.Group != nil {
		return ToolArgField
	}
	switch imm.kind {
	case immByte:
		return ToolArgUint8
	case immInt8:
		return ToolArgInt8
	case immLabel, immLabels:
		return ToolArgLabel
	case immInt, immInts:
		return ToolArgUint64
	case immBytes, immBytess:
		return ToolArgBytes
	default:
		return ToolArgNone
	}
}

func toolFieldGroupFromImmediate(imm immediate) ToolFieldGroup {
	return imm.Group
}

func toolArgsSignature(args []ToolArg, typed bool) string {
	var parts []string
	optional := false
	for _, arg := range args {
		name := arg.Name
		if typed {
			name = "{" + toolArgKindString(arg) + " : " + name + "}"
		}
		if arg.Array {
			name += ", ..."
		}
		if arg.Optional && !optional {
			optional = true
			name = "[" + name
		}
		parts = append(parts, name)
	}
	if optional && len(parts) > 0 {
		parts[len(parts)-1] += "]"
	}
	return strings.Join(parts, " ")
}

func toolFullSignature(name string, args []ToolArg) string {
	argsSignature := toolArgsSignature(args, true)
	if argsSignature == "" {
		return name + " "
	}
	return name + " " + argsSignature
}

func toolArgKindString(arg ToolArg) string {
	switch arg.Kind {
	case ToolArgBool:
		return "bool"
	case ToolArgConstInt, ToolArgUint64:
		return "uint64"
	case ToolArgUint8:
		return "uint8"
	case ToolArgInt8:
		return "int8"
	case ToolArgBytes:
		return "bytes"
	case ToolArgField:
		return toolFieldGroupString(arg.FieldGroup)
	case ToolArgLabel:
		return "label name"
	case ToolArgSignature:
		return "signature"
	case ToolArgAddress:
		return "address"
	case ToolArgPragmaName:
		return "pragma name"
	case ToolArgDefineName:
		return "define name"
	default:
		return "(none)"
	}
}

// toolFieldGroupLabels names the type shown for a field argument in signature
// help. Groups without an entry fall back to the assembler's own group name,
// so a field group added upstream is still described.
var toolFieldGroupLabels = map[string]string{
	TxnFields.Name:          "transaction field index",
	TxnScalarFields.Name:    "transaction field index",
	TxnArrayFields.Name:     "transaction array field index",
	GlobalFields.Name:       "global field index",
	EcdsaCurves.Name:        "ECDSA Curve",
	EcGroups.Name:           "EC group field index",
	Base64Encodings.Name:    "base64 encoding",
	JSONRefTypes.Name:       "json_Ref",
	VrfStandards.Name:       "parameters index",
	BlockFields.Name:        "block field",
	AssetHoldingFields.Name: "asset holding field index",
	AssetParamsFields.Name:  "asset params field index",
	AppParamsFields.Name:    "app params field index",
	AcctParamsFields.Name:   "account params field index",
	VoterParamsFields.Name:  "voter params field index",
	MimcConfigs.Name:        "MiMC field index",
}

func toolFieldGroupString(group ToolFieldGroup) string {
	if group == nil {
		return "(none)"
	}
	if label, ok := toolFieldGroupLabels[group.Name]; ok {
		return label
	}
	return group.Name
}

// ToolArgValuesForTools returns named values for a tooling argument kind.
type toolArgValuesKey struct {
	kind    ToolArgKind
	group   ToolFieldGroup
	version uint64
	mode    RunMode
}

// toolArgValues memoizes ToolArgValuesForTools, which rebuilds and re-sorts
// the same field tables on every argument completion.
var toolArgValues sync.Map // toolArgValuesKey -> []ToolArgValue

// ToolArgValuesForTools returns the values an argument accepts at the given
// version and mode.
//
// The result is cached and shared between callers, so it must be treated as
// read only.
func ToolArgValuesForTools(kind ToolArgKind, group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
	if version > AssemblerMaxVersion {
		// Field tables are unchanged past the newest assembler version, so a
		// source pinning a higher version selects the same values.
		version = AssemblerMaxVersion
	}

	key := toolArgValuesKey{kind: kind, group: group, version: version, mode: mode}
	if cached, ok := toolArgValues.Load(key); ok {
		return cached.([]ToolArgValue)
	}

	cached, _ := toolArgValues.LoadOrStore(key, buildToolArgValues(kind, group, version, mode))
	return cached.([]ToolArgValue)
}

func buildToolArgValues(kind ToolArgKind, group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
	var values []ToolArgValue
	if kind == ToolArgConstInt {
		for name, value := range txnTypeMap {
			if value != 0 {
				values = append(values, ToolArgValue{Name: name, Value: value})
			}
		}
		for name, value := range onCompletionMap {
			values = append(values, ToolArgValue{Name: name, Value: value})
		}
		sortToolArgValues(values)
		return values
	}
	if kind != ToolArgField {
		return nil
	}
	// toolFieldValues hands back a shared slice, so sort a copy of it.
	values = append(values, toolFieldValues(group, version, mode)...)
	sortToolArgValues(values)
	return values
}

func sortToolArgValues(values []ToolArgValue) {
	sort.Slice(values, func(i, j int) bool {
		return values[i].Name < values[j].Name
	})
}

type toolFieldValuesKey struct {
	group   ToolFieldGroup
	version uint64
	mode    RunMode
}

// toolFieldValueSets memoizes toolFieldValues. Resolving the named value of a
// field argument rebuilds the whole group table, which happens once per field
// argument in a document.
var toolFieldValueSets sync.Map // toolFieldValuesKey -> []ToolArgValue

// toolFieldValues returns the values a field group accepts at the given
// version and mode, in assembler table order.
//
// The result is cached and shared between callers, so it must be treated as
// read only.
func toolFieldValues(group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
	if version > AssemblerMaxVersion {
		// Field tables do not grow past the newest assembler version.
		version = AssemblerMaxVersion
	}

	key := toolFieldValuesKey{group: group, version: version, mode: mode}
	if cached, ok := toolFieldValueSets.Load(key); ok {
		return cached.([]ToolArgValue)
	}

	cached, _ := toolFieldValueSets.LoadOrStore(key, buildToolFieldValues(group, version, mode))
	return cached.([]ToolArgValue)
}

func buildToolFieldValues(group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
	if group == nil {
		return nil
	}
	// txn, gtxn and gtxns take an optional index immediate, so they accept the
	// array fields as well as the scalar ones. Widen the scalar group rather
	// than offering an incomplete list.
	if group == &TxnScalarFields {
		group = &TxnFields
	}
	return toolFieldGroupValues(group, version, mode)
}

func toolFieldGroupValues(group *FieldGroup, version uint64, mode RunMode) []ToolArgValue {
	var values []ToolArgValue
	for _, name := range group.Names {
		if name == "" {
			continue
		}
		fs, ok := group.SpecByName(name)
		if !ok {
			continue
		}
		if version > 0 && fs.Version() > version {
			continue
		}
		if gf, ok := fs.(globalFieldSpec); ok && gf.mode&mode == 0 {
			continue
		}
		values = append(values, ToolArgValue{
			Value:   uint64(fs.Field()),
			Name:    name,
			Docs:    fs.Note(),
			Version: fs.Version(),
		})
	}
	return values
}

func sourceProgramForTools(lines []SourceLine, idx SourceIndex, opts SourceToolOptions) SourceProgram {
	mode := opts.Mode
	if mode == 0 {
		mode = ModeApp
	}
	version := idx.Version
	if opts.UseVersion {
		version = opts.Version
	}
	if version == 0 {
		version = AssemblerDefaultVersion
	}

	program := SourceProgram{}
	activeDefines := make(map[string]bool)
	for _, line := range lines {
		for statementIndex, statement := range line.Statements {
			opToken, argTokens, ok := sourceOperationTokens(statement)
			if !ok {
				continue
			}

			if opToken.Text == "#define" {
				program.TokenClasses = append(program.TokenClasses, SourceTokenClass{Kind: SourceTokenClassMacro, Token: opToken})
				if len(argTokens) > 0 {
					activeDefines[argTokens[0].Text] = true
				}
			}

			if activeDefines[opToken.Text] && opToken.Text != "#define" {
				continue
			}

			op, ok := toolOpcodeForSource(opToken.Text, len(argTokens), mode)
			if !ok {
				continue
			}

			operation := SourceOperation{
				Name:          op.Name,
				CanonicalName: op.CanonicalName,
				Token:         opToken,
				Statement:     statementIndex,
				EndColumn:     sourceOperationEndColumn(opToken, argTokens),
				ToolArgs:      op.Args,
				Args:          sourceArgumentsForTools(op.Name, op.Args, argTokens, version, mode, activeDefines),
			}
			if op.Version > version {
				operation.RequiredVersion = op.Version
				program.RequiredVersions = append(program.RequiredVersions, SourceRequiredVersion{
					Line:      opToken.Line,
					Column:    opToken.Column,
					EndColumn: operation.EndColumn,
					Version:   op.Version,
				})
			}
			program.Operations = append(program.Operations, operation)
			if !strings.HasPrefix(op.Name, "#") {
				program.TokenClasses = append(program.TokenClasses, SourceTokenClass{Kind: SourceTokenClassOpcode, Token: opToken})
			}
			program.TokenClasses = append(program.TokenClasses, sourceTokenClassesForOperation(operation)...)
		}
	}
	return program
}

// SourceStatementAtForTools returns the statement at a byte-column source
// position.
func SourceStatementAtForTools(lines []SourceLine, line int, column int) (SourceStatement, int, bool) {
	if line < 0 || line >= len(lines) {
		return SourceStatement{}, 0, false
	}
	statements := lines[line].Statements
	if len(statements) == 0 {
		return SourceStatement{}, 0, false
	}
	if column <= statements[0].Column {
		return statements[0], 0, true
	}
	lastIndex := len(statements) - 1
	if column >= statements[lastIndex].EndColumn {
		return statements[lastIndex], lastIndex, true
	}
	for i, statement := range statements {
		if column >= statement.Column && column <= statement.EndColumn {
			return statement, i, true
		}
	}
	return SourceStatement{}, 0, false
}

// SourceOperationAtForTools returns the operation at a byte-column source
// position.
func SourceOperationAtForTools(lines []SourceLine, program SourceProgram, line int, column int) (SourceOperation, bool) {
	if line < 0 || line >= len(lines) {
		return SourceOperation{}, false
	}
	for _, op := range program.Operations {
		if op.Token.Line != line {
			continue
		}
		if column >= op.Token.Column && column <= op.EndColumn+1 {
			return op, true
		}
	}
	return SourceOperation{}, false
}

// SourceToolArgAtForTools returns the source-level argument role at a
// byte-column source position.
func SourceToolArgAtForTools(lines []SourceLine, program SourceProgram, line int, column int) (ToolArg, int, bool) {
	var res ToolArg
	op, ok := SourceOperationAtForTools(lines, program, line, column)
	if !ok {
		return res, -1, false
	}
	for idx, arg := range op.Args {
		if sourceTokenContains(arg.Token, column) {
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

// SourceCompletionContextForTools returns opcode-or-argument completion context
// at a byte-column source position.
func SourceCompletionContextForTools(lines []SourceLine, program SourceProgram, line int, column int) SourceCompletionContext {
	statement, statementIndex, ok := SourceStatementAtForTools(lines, line, column)
	if !ok || len(statement.Tokens) == 0 {
		return SourceCompletionContext{Mode: SourceCompletionOpcode, Statement: statement, StatementIndex: statementIndex}
	}
	first := statement.Tokens[0]
	if column <= first.EndColumn {
		return SourceCompletionContext{
			Mode:           SourceCompletionOpcode,
			Prefix:         first.Text,
			Statement:      statement,
			StatementIndex: statementIndex,
		}
	}
	return SourceCompletionContext{
		Mode:           SourceCompletionArgument,
		Statement:      statement,
		StatementIndex: statementIndex,
	}
}

// SourceInlayHintsForTools returns assembler-native inlay hints for the lines in
// rg. Pass SourceAllLines for a whole document.
func SourceInlayHintsForTools(lines []SourceLine, program SourceProgram, rg SourceLineRange) []SourceInlayHint {
	var hints []SourceInlayHint
	seenDecoded := make(map[SourceToken]bool)
	for _, op := range program.Operations {
		for _, arg := range op.Args {
			if arg.Token.Line < 0 || arg.Token.Line >= len(lines) || !rg.Contains(arg.Token.Line) {
				continue
			}
			if arg.HasValue && arg.ValueName != "" && arg.Token.Text != arg.ValueName {
				hints = append(hints, SourceInlayHint{
					Kind:  SourceInlayHintNamed,
					Token: arg.Token,
					Label: arg.ValueName,
				})
			}
			if decoded, ok := DecodedHexStringForTools(arg.Token.Text); ok {
				seenDecoded[arg.Token] = true
				hints = append(hints, SourceInlayHint{
					Kind:  SourceInlayHintDecoded,
					Token: arg.Token,
					Label: decoded,
				})
			}
		}
	}
	for _, line := range lines {
		if !rg.Contains(line.Line) {
			continue
		}
		for _, token := range line.Tokens {
			if seenDecoded[token] {
				continue
			}
			if decoded, ok := DecodedHexStringForTools(token.Text); ok {
				hints = append(hints, SourceInlayHint{
					Kind:  SourceInlayHintDecoded,
					Token: token,
					Label: decoded,
				})
			}
		}
	}
	return hints
}

func sourceOperationTokens(statement SourceStatement) (SourceToken, []SourceToken, bool) {
	tokens := statement.Tokens
	if len(tokens) == 0 {
		return SourceToken{}, nil, false
	}
	if strings.HasSuffix(tokens[0].Text, ":") {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return SourceToken{}, nil, false
	}
	return tokens[0], tokens[1:], true
}

func sourceOperationEndColumn(op SourceToken, args []SourceToken) int {
	if len(args) == 0 {
		return op.EndColumn
	}
	return args[len(args)-1].EndColumn
}

func sourceArgumentsForTools(opName string, args []ToolArg, tokens []SourceToken, version uint64, mode RunMode, defines map[string]bool) []SourceArgument {
	if opName == "#pragma" {
		args = toolPragmaArgs(tokens)
	}
	var result []SourceArgument
	for i, token := range tokens {
		if defines[token.Text] {
			continue
		}
		arg, ok := toolArgAt(args, i)
		if !ok {
			continue
		}
		sourceArg := SourceArgument{
			ToolArg: arg,
			Token:   token,
		}
		if value, ok := sourceArgValue(arg, token.Text, version, mode); ok {
			sourceArg.HasValue = true
			sourceArg.Value = value.Value
			sourceArg.ValueName = value.Name
			sourceArg.Docs = value.Docs
		}
		result = append(result, sourceArg)
	}
	return result
}

func toolPragmaArgs(tokens []SourceToken) []ToolArg {
	args := []ToolArg{{Name: "name", Kind: ToolArgPragmaName}}
	if len(tokens) == 0 {
		return args
	}
	switch tokens[0].Text {
	case "typetrack":
		args = append(args, ToolArg{Name: "typetrack value", Kind: ToolArgBool})
	case "version":
		args = append(args, ToolArg{Name: "version value", Kind: ToolArgUint64})
	default:
		args = append(args, ToolArg{Name: "value", Kind: ToolArgNone})
	}
	return args
}

func toolArgAt(args []ToolArg, index int) (ToolArg, bool) {
	if index < len(args) {
		return args[index], true
	}
	if len(args) > 0 && args[len(args)-1].Array {
		return args[len(args)-1], true
	}
	return ToolArg{}, false
}

func sourceArgValue(arg ToolArg, text string, version uint64, mode RunMode) (ToolArgValue, bool) {
	if arg.Kind != ToolArgField {
		return ToolArgValue{}, false
	}
	numeric, numericErr := strconv.ParseUint(text, 0, 64)
	for _, value := range toolFieldValues(arg.FieldGroup, version, mode) {
		if value.Name == text {
			return value, true
		}
		if numericErr == nil && value.Value == numeric {
			return value, true
		}
	}
	return ToolArgValue{}, false
}

func sourceTokenClassesForOperation(operation SourceOperation) []SourceTokenClass {
	var classes []SourceTokenClass
	if strings.HasPrefix(operation.Name, "#") {
		classes = append(classes, SourceTokenClass{Kind: SourceTokenClassMacro, Token: operation.Token})
	}
	for _, arg := range operation.Args {
		switch arg.Kind {
		case ToolArgBool:
			if arg.Token.Text == "true" || arg.Token.Text == "false" {
				classes = append(classes, SourceTokenClass{Kind: SourceTokenClassBool, Token: arg.Token})
			}
		case ToolArgConstInt, ToolArgUint64, ToolArgUint8, ToolArgInt8:
			classes = append(classes, SourceTokenClass{Kind: SourceTokenClassNumber, Token: arg.Token})
		case ToolArgBytes, ToolArgSignature, ToolArgAddress:
			classes = append(classes, sourceBytesTokenClasses(arg.Token)...)
		case ToolArgField:
			if arg.HasValue && arg.ValueName == arg.Token.Text {
				classes = append(classes, SourceTokenClass{Kind: SourceTokenClassString, Token: arg.Token})
			} else {
				classes = append(classes, SourceTokenClass{Kind: SourceTokenClassNumber, Token: arg.Token})
			}
		case ToolArgPragmaName:
			classes = append(classes, SourceTokenClass{Kind: SourceTokenClassMacro, Token: arg.Token})
		default:
		}
	}
	return classes
}

func sourceBytesTokenClasses(token SourceToken) []SourceTokenClass {
	text := token.Text
	if text == "base32" || text == "b32" || text == "base64" || text == "b64" {
		return []SourceTokenClass{{Kind: SourceTokenClassKeyword, Token: token}}
	}
	return []SourceTokenClass{{Kind: SourceTokenClassString, Token: token}}
}

// DecodedHexStringForTools decodes printable ASCII hex byte tokens for editor
// hints. Non-hex or non-printable tokens return false.
func DecodedHexStringForTools(text string) (string, bool) {
	if !strings.HasPrefix(text, "0x") {
		return "", false
	}
	bs, err := hex.DecodeString(text[2:])
	if err != nil {
		return "", false
	}
	decoded := string(bs)
	for _, r := range decoded {
		if r > 127 || r < 32 {
			return "", false
		}
	}
	return decoded, true
}
