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
type ToolFieldGroup int

const (
	ToolFieldNone ToolFieldGroup = iota
	ToolFieldTxn
	ToolFieldTxna
	ToolFieldItxn
	ToolFieldGlobal
	ToolFieldEcdsaCurve
	ToolFieldEcGroup
	ToolFieldBase64Encoding
	ToolFieldJSONRef
	ToolFieldVrfStandard
	ToolFieldBlock
	ToolFieldAssetHolding
	ToolFieldAssetParams
	ToolFieldAppParams
	ToolFieldAcctParams
	ToolFieldVoterParams
	ToolFieldMimc
)

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
	"txn":     {args: []ToolArg{{Name: "f", Kind: ToolArgField, FieldGroup: ToolFieldTxn}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 1, modes: modeAny},
	"gtxn":    {args: []ToolArg{{Name: "t", Kind: ToolArgUint8}, {Name: "f", Kind: ToolArgField, FieldGroup: ToolFieldTxn}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 1, modes: modeAny},
	"gtxns":   {args: []ToolArg{{Name: "f", Kind: ToolArgField, FieldGroup: ToolFieldTxn}, {Name: "i", Kind: ToolArgUint8, Optional: true}}, version: 3, modes: modeAny},
	"extract": {args: []ToolArg{{Name: "s", Kind: ToolArgUint8, Optional: true}, {Name: "l", Kind: ToolArgUint8, Optional: true}}, version: 5, modes: modeAny},
	"replace": {args: []ToolArg{{Name: "s", Kind: ToolArgUint8, Optional: true}}, version: 7, modes: modeAny},
}

// ToolOpcodesForTools returns source-level opcode metadata available at the
// requested version and mode.
func ToolOpcodesForTools(version uint64, mode RunMode) []ToolOpcode {
	if mode == 0 {
		mode = ModeApp
	}
	if version == 0 {
		version = AssemblerDefaultVersion
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
	return ops
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

func toolOpcodeNames() []string {
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

func toolOpcodeForSource(name string, argCount int, mode RunMode) (ToolOpcode, bool) {
	if overlay, ok := toolSourceOverlays[name]; ok {
		desc := OpDescOf(toolCanonicalName(name, argCount))
		return toolOpcodeFromParts(name, toolCanonicalName(name, argCount), overlay.version, overlay.modes, overlay.args, desc), true
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
	if imm.Group == nil {
		return ToolFieldNone
	}
	return toolFieldGroupFromName(imm.Group.Name)
}

func toolFieldGroupFromName(name string) ToolFieldGroup {
	switch name {
	case TxnFields.Name, TxnScalarFields.Name:
		return ToolFieldTxn
	case TxnArrayFields.Name:
		return ToolFieldTxna
	case GlobalFields.Name:
		return ToolFieldGlobal
	case EcdsaCurves.Name:
		return ToolFieldEcdsaCurve
	case EcGroups.Name:
		return ToolFieldEcGroup
	case Base64Encodings.Name:
		return ToolFieldBase64Encoding
	case JSONRefTypes.Name:
		return ToolFieldJSONRef
	case VrfStandards.Name:
		return ToolFieldVrfStandard
	case BlockFields.Name:
		return ToolFieldBlock
	case AssetHoldingFields.Name:
		return ToolFieldAssetHolding
	case AssetParamsFields.Name:
		return ToolFieldAssetParams
	case AppParamsFields.Name:
		return ToolFieldAppParams
	case AcctParamsFields.Name:
		return ToolFieldAcctParams
	case VoterParamsFields.Name:
		return ToolFieldVoterParams
	case MimcConfigs.Name:
		return ToolFieldMimc
	default:
		return ToolFieldNone
	}
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

func toolFieldGroupString(group ToolFieldGroup) string {
	switch group {
	case ToolFieldTxna:
		return "transaction array field index"
	case ToolFieldTxn:
		return "transaction field index"
	case ToolFieldItxn:
		return "internal transaction field index"
	case ToolFieldGlobal:
		return "global field index"
	case ToolFieldEcdsaCurve:
		return "ECDSA Curve"
	case ToolFieldEcGroup:
		return "EC group field index"
	case ToolFieldBase64Encoding:
		return "base64 encoding"
	case ToolFieldJSONRef:
		return "json_Ref"
	case ToolFieldVrfStandard:
		return "parameters index"
	case ToolFieldBlock:
		return "block field"
	case ToolFieldAssetHolding:
		return "asset holding field index"
	case ToolFieldAssetParams:
		return "asset params field index"
	case ToolFieldAppParams:
		return "app params field index"
	case ToolFieldAcctParams:
		return "account params field index"
	case ToolFieldVoterParams:
		return "voter params field index"
	case ToolFieldMimc:
		return "MiMC field index"
	default:
		return "(none)"
	}
}

// ToolArgValuesForTools returns named values for a tooling argument kind.
func ToolArgValuesForTools(kind ToolArgKind, group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
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
	values = toolFieldValues(group, version, mode)
	sortToolArgValues(values)
	return values
}

func sortToolArgValues(values []ToolArgValue) {
	sort.Slice(values, func(i, j int) bool {
		return values[i].Name < values[j].Name
	})
}

func toolFieldValues(group ToolFieldGroup, version uint64, mode RunMode) []ToolArgValue {
	switch group {
	case ToolFieldTxn:
		return toolTxnFieldValues(version, mode, false, false)
	case ToolFieldTxna:
		return toolTxnFieldValues(version, mode, true, false)
	case ToolFieldItxn:
		return toolTxnFieldValues(version, mode, false, true)
	case ToolFieldGlobal:
		return toolFieldGroupValues(&GlobalFields, version, mode)
	case ToolFieldEcdsaCurve:
		return toolFieldGroupValues(&EcdsaCurves, version, mode)
	case ToolFieldEcGroup:
		return toolFieldGroupValues(&EcGroups, version, mode)
	case ToolFieldBase64Encoding:
		return toolFieldGroupValues(&Base64Encodings, version, mode)
	case ToolFieldJSONRef:
		return toolFieldGroupValues(&JSONRefTypes, version, mode)
	case ToolFieldVrfStandard:
		return toolFieldGroupValues(&VrfStandards, version, mode)
	case ToolFieldBlock:
		return toolFieldGroupValues(&BlockFields, version, mode)
	case ToolFieldAssetHolding:
		return toolFieldGroupValues(&AssetHoldingFields, version, mode)
	case ToolFieldAssetParams:
		return toolFieldGroupValues(&AssetParamsFields, version, mode)
	case ToolFieldAppParams:
		return toolFieldGroupValues(&AppParamsFields, version, mode)
	case ToolFieldAcctParams:
		return toolFieldGroupValues(&AcctParamsFields, version, mode)
	case ToolFieldVoterParams:
		return toolFieldGroupValues(&VoterParamsFields, version, mode)
	case ToolFieldMimc:
		return toolFieldGroupValues(&MimcConfigs, version, mode)
	default:
		return nil
	}
}

func toolTxnFieldValues(version uint64, mode RunMode, array bool, itxn bool) []ToolArgValue {
	var values []ToolArgValue
	for _, name := range TxnFields.Names {
		if name == "" {
			continue
		}
		fs, ok := TxnFields.SpecByName(name)
		if !ok {
			continue
		}
		txnSpec := fs.(txnFieldSpec)
		if array && !txnSpec.array {
			continue
		}
		fieldVersion := txnSpec.Version()
		if itxn {
			fieldVersion = txnSpec.itxVersion
			if fieldVersion == 0 {
				continue
			}
		}
		if version > 0 && fieldVersion > version {
			continue
		}
		values = append(values, ToolArgValue{
			Value:   uint64(txnSpec.Field()),
			Name:    name,
			Docs:    txnSpec.Note(),
			Version: fieldVersion,
		})
	}
	return values
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
		for _, statement := range line.Statements {
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
	for _, value := range toolFieldValues(arg.FieldGroup, version, mode) {
		if value.Name == text {
			return value, true
		}
		if numeric, err := strconv.ParseUint(text, 0, 64); err == nil && value.Value == numeric {
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
