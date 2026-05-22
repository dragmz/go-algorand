package lsp

import (
	"strings"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func sourceModeForLSP(lines []logic.SourceLine) logic.RunMode {
	for _, line := range lines {
		if line.Comment != nil && strings.TrimSpace(line.Comment.Text) == "#pragma mode logicsig" {
			return logic.ModeSig
		}
	}
	return logic.ModeApp
}

func operationTokensFromSourceProgram(lines []logic.SourceLine, program logic.SourceProgram) []Token {
	tokens := make([]Token, 0, len(program.Operations))
	for _, op := range program.Operations {
		if op.Token.Line >= 0 && op.Token.Line < len(lines) {
			tokens = append(tokens, tokenFromSourceToken(lines[op.Token.Line].Text, op.Token))
		}
	}
	return tokens
}

func requiredVersionsFromSourceProgram(lines []logic.SourceLine, program logic.SourceProgram) []RequiredVersion {
	versions := make([]RequiredVersion, 0, len(program.RequiredVersions))
	for _, required := range program.RequiredVersions {
		if required.Line < 0 || required.Line >= len(lines) {
			continue
		}
		versions = append(versions, RequiredVersion{
			Line:    required.Line,
			Begin:   utf16ColumnFromByte(lines[required.Line].Text, required.Column),
			End:     utf16ColumnFromByte(lines[required.Line].Text, required.EndColumn),
			Version: required.Version,
		})
	}
	return versions
}

func tokensOfSourceClass(lines []logic.SourceLine, program logic.SourceProgram, kind logic.SourceTokenClassKind) []Token {
	var tokens []Token
	for _, class := range program.TokenClasses {
		if class.Kind != kind {
			continue
		}
		if class.Token.Line < 0 || class.Token.Line >= len(lines) {
			continue
		}
		tokens = append(tokens, tokenFromSourceToken(lines[class.Token.Line].Text, class.Token))
	}
	return tokens
}

func tokenFromSourceArgument(lines []logic.SourceLine, arg logic.SourceArgument) (Token, bool) {
	if arg.Token.Line < 0 || arg.Token.Line >= len(lines) {
		return Token{}, false
	}
	return tokenFromSourceToken(lines[arg.Token.Line].Text, arg.Token), true
}

func tokenFromSourceOperation(lines []logic.SourceLine, op logic.SourceOperation) (Token, bool) {
	if op.Token.Line < 0 || op.Token.Line >= len(lines) {
		return Token{}, false
	}
	return tokenFromSourceToken(lines[op.Token.Line].Text, op.Token), true
}
