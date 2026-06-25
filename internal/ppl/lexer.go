// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.1  2026-06-24  Initial implementation
// ============================================================================

package ppl

import (
	"strings"
	"unicode"
)

// TokenKind identifies the type of a lexed token.
type TokenKind int

const (
	TokEOF TokenKind = iota
	TokNewline
	TokIdent    // identifiers and keywords
	TokInteger  // 42
	TokReal     // 3.14
	TokString   // "hello"
	TokPlus     // +
	TokMinus    // -
	TokStar     // *
	TokSlash    // /
	TokCaret    // ^
	TokLParen   // (
	TokRParen   // )
	TokComma    // ,
	TokSemicolon // ;
	TokColon    // :
	TokEq       // =  (assignment and comparison)
	TokNeq      // <>
	TokLt       // <
	TokLte      // <=
	TokGt       // >
	TokGte      // >=
	TokAmpAmp   // && (logical AND)
	TokPipePipe // || (logical OR)
	TokBang     // ! (logical NOT)
	TokAmp      // & (bitwise AND)
	TokPipe     // | (bitwise OR)
	TokPercent  // % (MOD)
	TokDot      // .
	TokHash     // # (label prefix)
)

// Token is a single lexed unit.
type Token struct {
	Kind    TokenKind
	Text    string
	Line    int
}

// Keywords (case-insensitive in PPL)
var keywords = map[string]bool{
	"IF": true, "THEN": true, "ELSE": true, "ELSEIF": true, "ENDIF": true,
	"WHILE": true, "WEND": true,
	"FOR": true, "TO": true, "STEP": true, "NEXT": true,
	"GOTO": true, "GOSUB": true, "RETURN": true,
	"END": true, "QUIT": true, "STOP": true,
	"LET": true,
	"BOOLEAN": true, "INTEGER": true, "STRING": true, "REAL": true,
	"DATE": true, "TIME": true, "MONEY": true, "BIGSTR": true,
	"DIM": true,
	"FUNCTION": true, "PROCEDURE": true, "ENDFUNC": true, "ENDPROC": true,
	"LOCAL": true, "GLOBAL": true,
	"AND": true, "OR": true, "NOT": true, "MOD": true,
	"TRUE": true, "FALSE": true,
}

// Lexer tokenises a PPL source string.
type Lexer struct {
	src  []rune
	pos  int
	line int
}

func NewLexer(src string) *Lexer {
	return &Lexer{src: []rune(src), line: 1}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) next() rune {
	r := l.src[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
	}
	return r
}

func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.scan()
		tokens = append(tokens, tok)
		if tok.Kind == TokEOF {
			break
		}
	}
	return tokens
}

func (l *Lexer) scan() Token {
	// Skip spaces (but not newlines)
	for l.pos < len(l.src) && l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r' {
		if l.peek() == '\r' {
			l.pos++
			continue
		}
		l.pos++
	}

	if l.pos >= len(l.src) {
		return Token{Kind: TokEOF, Line: l.line}
	}

	line := l.line
	ch := l.peek()

	// Comment: ; or ' starts a line comment
	if ch == ';' || ch == '\'' {
		for l.pos < len(l.src) && l.peek() != '\n' {
			l.pos++
		}
		return l.scan()
	}

	// Newline
	if ch == '\n' {
		l.next()
		return Token{Kind: TokNewline, Text: "\n", Line: line}
	}

	// String literal
	if ch == '"' {
		l.next()
		var sb strings.Builder
		for l.pos < len(l.src) && l.peek() != '"' {
			c := l.next()
			if c == '\\' && l.pos < len(l.src) {
				switch l.next() {
				case 'n':
					sb.WriteByte('\n')
				case 'r':
					sb.WriteByte('\r')
				case 't':
					sb.WriteByte('\t')
				case '"':
					sb.WriteByte('"')
				default:
					sb.WriteByte('\\')
				}
			} else {
				sb.WriteRune(c)
			}
		}
		if l.pos < len(l.src) {
			l.next() // closing "
		}
		return Token{Kind: TokString, Text: sb.String(), Line: line}
	}

	// Number
	if unicode.IsDigit(ch) || (ch == '.' && l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1])) {
		return l.scanNumber(line)
	}

	// Identifier or keyword
	if unicode.IsLetter(ch) || ch == '_' || ch == '@' {
		return l.scanIdent(line)
	}

	// Label: #identifier
	if ch == '#' {
		l.next()
		tok := l.scanIdent(line)
		tok.Kind = TokHash
		tok.Text = "#" + tok.Text
		return tok
	}

	l.next()
	switch ch {
	case '+':
		return Token{Kind: TokPlus, Text: "+", Line: line}
	case '-':
		return Token{Kind: TokMinus, Text: "-", Line: line}
	case '*':
		return Token{Kind: TokStar, Text: "*", Line: line}
	case '/':
		return Token{Kind: TokSlash, Text: "/", Line: line}
	case '^':
		return Token{Kind: TokCaret, Text: "^", Line: line}
	case '(':
		return Token{Kind: TokLParen, Text: "(", Line: line}
	case ')':
		return Token{Kind: TokRParen, Text: ")", Line: line}
	case ',':
		return Token{Kind: TokComma, Text: ",", Line: line}
	case ':':
		return Token{Kind: TokColon, Text: ":", Line: line}
	case '%':
		return Token{Kind: TokPercent, Text: "%", Line: line}
	case '&':
		if l.peek() == '&' {
			l.next()
			return Token{Kind: TokAmpAmp, Text: "&&", Line: line}
		}
		return Token{Kind: TokAmp, Text: "&", Line: line}
	case '|':
		if l.peek() == '|' {
			l.next()
			return Token{Kind: TokPipePipe, Text: "||", Line: line}
		}
		return Token{Kind: TokPipe, Text: "|", Line: line}
	case '!':
		if l.peek() == '=' {
			l.next()
			return Token{Kind: TokNeq, Text: "!=", Line: line}
		}
		return Token{Kind: TokBang, Text: "!", Line: line}
	case '=':
		return Token{Kind: TokEq, Text: "=", Line: line}
	case '<':
		if l.peek() == '>' {
			l.next()
			return Token{Kind: TokNeq, Text: "<>", Line: line}
		}
		if l.peek() == '=' {
			l.next()
			return Token{Kind: TokLte, Text: "<=", Line: line}
		}
		return Token{Kind: TokLt, Text: "<", Line: line}
	case '>':
		if l.peek() == '=' {
			l.next()
			return Token{Kind: TokGte, Text: ">=", Line: line}
		}
		return Token{Kind: TokGt, Text: ">", Line: line}
	}

	// Unknown — skip
	return l.scan()
}

func (l *Lexer) scanNumber(line int) Token {
	start := l.pos
	isReal := false
	for l.pos < len(l.src) && (unicode.IsDigit(l.peek()) || l.peek() == '.') {
		if l.peek() == '.' {
			if isReal {
				break
			}
			isReal = true
		}
		l.pos++
	}
	text := string(l.src[start:l.pos])
	if isReal {
		return Token{Kind: TokReal, Text: text, Line: line}
	}
	return Token{Kind: TokInteger, Text: text, Line: line}
}

func (l *Lexer) scanIdent(line int) Token {
	start := l.pos
	for l.pos < len(l.src) && (unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_') {
		l.pos++
	}
	text := string(l.src[start:l.pos])
	upper := strings.ToUpper(text)
	return Token{Kind: TokIdent, Text: upper, Line: line}
}
