// Copyright (C) 2019-2025 Algorand, Inc.
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

// SourceTokenKind classifies tokens returned by SourceLinesForTools.
type SourceTokenKind int

const (
	SourceTokenValue SourceTokenKind = iota
	SourceTokenSemicolon
	SourceTokenComment
)

// SourceToken is an assembler-tokenized source token. Line and column values
// are zero-based, and columns are byte offsets into SourceLine.Text.
type SourceToken struct {
	Text      string
	Kind      SourceTokenKind
	Line      int
	Column    int
	EndColumn int
}

// SourceStatement is a semicolon-delimited TEAL source statement.
type SourceStatement struct {
	Tokens    []SourceToken
	Line      int
	Column    int
	EndColumn int
}

// SourceLine is a single TEAL source line tokenized with assembler rules.
type SourceLine struct {
	Line       int
	Text       string
	Tokens     []SourceToken
	Comment    *SourceToken
	Statements []SourceStatement
}

// SourcePosition is a zero-based byte-offset position in assembler source.
type SourcePosition struct {
	Line   int
	Column int
}

// SourceLinesForTools tokenizes TEAL source using assembler source rules while
// preserving comments and statement boundaries for editor tooling.
func SourceLinesForTools(source string) []SourceLine {
	sourceLines := splitSourceLinesForTools(source)
	lines := make([]SourceLine, 0, len(sourceLines))
	for lineNo, text := range sourceLines {
		tokens := sourceTokensFromLine(text, lineNo)
		lines = append(lines, SourceLine{
			Line:       lineNo,
			Text:       text,
			Tokens:     nonCommentSourceTokens(tokens),
			Comment:    sourceLineComment(tokens),
			Statements: sourceStatementsFromTokens(nonCommentSourceTokens(tokens), lineNo),
		})
	}

	return lines
}

func splitSourceLinesForTools(source string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			lines = append(lines, source[start:i])
			if i+1 < len(source) && source[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n':
			lines = append(lines, source[start:i])
			start = i + 1
		}
	}
	if start < len(source) {
		lines = append(lines, source[start:])
	}
	return lines
}

func sourceTokensFromLine(sourceLine string, lineNo int) []SourceToken {
	var tokens []SourceToken

	i := 0
	for i < len(sourceLine) && tokenSeparators[sourceLine[i]] {
		if sourceLine[i] == ';' {
			tokens = append(tokens, SourceToken{
				Text:      ";",
				Kind:      SourceTokenSemicolon,
				Line:      lineNo,
				Column:    i,
				EndColumn: i + 1,
			})
		}
		i++
	}

	start := i
	inString := false // tracked to allow spaces and comments inside
	inBase64 := false // tracked to allow '//' inside
	for i < len(sourceLine) {
		if !tokenSeparators[sourceLine[i]] { // if not space
			switch sourceLine[i] {
			case '"': // is a string literal?
				if !inString {
					if i == 0 || i > 0 && tokenSeparators[sourceLine[i-1]] {
						inString = true
					}
				} else {
					if sourceLine[i-1] != '\\' { // if not escape symbol
						inString = false
					}
				}
			case '/': // is a comment?
				if i < len(sourceLine)-1 && sourceLine[i+1] == '/' && !inBase64 && !inString {
					if start != i { // if a comment without whitespace
						tokens = append(tokens, sourceValueToken(sourceLine, lineNo, start, i))
					}
					tokens = append(tokens, SourceToken{
						Text:      sourceLine[i+2:],
						Kind:      SourceTokenComment,
						Line:      lineNo,
						Column:    i,
						EndColumn: len(sourceLine),
					})
					return tokens
				}
			case '(': // is base64( seq?
				prefix := sourceLine[start:i]
				if prefix == "base64" || prefix == "b64" {
					inBase64 = true
				}
			case ')': // is ) as base64( completion
				if inBase64 {
					inBase64 = false
				}
			default:
			}
			i++
			continue
		}

		// we've hit a space, end last token unless inString

		if !inString {
			s := sourceLine[start:i]
			tokens = append(tokens, SourceToken{
				Text:      s,
				Kind:      SourceTokenValue,
				Line:      lineNo,
				Column:    start,
				EndColumn: i,
			})
			if sourceLine[i] == ';' {
				tokens = append(tokens, SourceToken{
					Text:      ";",
					Kind:      SourceTokenSemicolon,
					Line:      lineNo,
					Column:    i,
					EndColumn: i + 1,
				})
			}
			if inBase64 {
				inBase64 = false
			} else if s == "base64" || s == "b64" {
				inBase64 = true
			}
		}
		i++

		// gobble up consecutive whitespace (but notice semis)
		if !inString {
			for i < len(sourceLine) && tokenSeparators[sourceLine[i]] {
				if sourceLine[i] == ';' {
					tokens = append(tokens, SourceToken{
						Text:      ";",
						Kind:      SourceTokenSemicolon,
						Line:      lineNo,
						Column:    i,
						EndColumn: i + 1,
					})
				}
				i++
			}
			start = i
		}
	}

	// add rest of the string if any
	if start < len(sourceLine) {
		tokens = append(tokens, sourceValueToken(sourceLine, lineNo, start, i))
	}

	return tokens
}

func sourceValueToken(sourceLine string, lineNo, start, end int) SourceToken {
	return SourceToken{
		Text:      sourceLine[start:end],
		Kind:      SourceTokenValue,
		Line:      lineNo,
		Column:    start,
		EndColumn: end,
	}
}

func nonCommentSourceTokens(tokens []SourceToken) []SourceToken {
	for i, token := range tokens {
		if token.Kind == SourceTokenComment {
			return tokens[:i]
		}
	}
	return tokens
}

func sourceLineComment(tokens []SourceToken) *SourceToken {
	for _, token := range tokens {
		if token.Kind == SourceTokenComment {
			comment := token
			return &comment
		}
	}
	return nil
}

func sourceStatementsFromTokens(tokens []SourceToken, lineNo int) []SourceStatement {
	var statements []SourceStatement
	start := 0
	for i, token := range tokens {
		if token.Kind == SourceTokenSemicolon {
			statements = append(statements, sourceStatementFromTokens(tokens[start:i], lineNo, token.Column))
			start = i + 1
		}
	}
	statements = append(statements, sourceStatementFromTokens(tokens[start:], lineNo, lenFromTokens(tokens)))
	return statements
}

func sourceStatementFromTokens(tokens []SourceToken, lineNo, emptyColumn int) SourceStatement {
	if len(tokens) == 0 {
		return SourceStatement{
			Line:      lineNo,
			Column:    emptyColumn,
			EndColumn: emptyColumn,
		}
	}

	return SourceStatement{
		Tokens:    tokens,
		Line:      tokens[0].Line,
		Column:    tokens[0].Column,
		EndColumn: tokens[len(tokens)-1].EndColumn,
	}
}

func lenFromTokens(tokens []SourceToken) int {
	if len(tokens) == 0 {
		return 0
	}
	return tokens[len(tokens)-1].EndColumn
}
