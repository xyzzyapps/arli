package main

import (
	"strconv"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

type TokenType int

const (
	TK_OPEN  TokenType = iota // (
	TK_CLOSE                  // )
	TK_NUMBER                 // 42, 3.14
	TK_STRING                 // "hello"
	TK_SYMBOL                 // foo, +, define
	TK_QUOTE                  // '
	TK_EOF                    // end of input
)

type Token struct {
	Type  TokenType
	Value string
	Num   float64 // parsed number value (for TK_NUMBER)
	Str   string  // parsed string content (for TK_STRING)
}

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

func Tokenize(source string) []Token {
	var tokens []Token
	i := 0
	runes := []rune(source)
	n := len(runes)

	for i < n {
		ch := runes[i]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' {
			i++
			continue
		}

		// Comments
		if ch == ';' {
			for i < n && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			continue
		}

		// Parentheses
		if ch == '(' {
			tokens = append(tokens, Token{Type: TK_OPEN, Value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, Token{Type: TK_CLOSE, Value: ")"})
			i++
			continue
		}

		// Quote
		if ch == '\'' {
			tokens = append(tokens, Token{Type: TK_QUOTE, Value: "'"})
			i++
			continue
		}

		// Strings
		if ch == '"' {
			i++ // skip opening "
			var s []rune
			for i < n {
				c := runes[i]
				if c == '"' {
					i++
					break
				}
				if c == '\\' && i+1 < n {
					next := runes[i+1]
					switch next {
					case 'n':
						s = append(s, '\n')
					case 't':
						s = append(s, '\t')
					case 'r':
						s = append(s, '\r')
					case '"':
						s = append(s, '"')
					case '\\':
						s = append(s, '\\')
					default:
						s = append(s, c)
						i++
						continue
					}
					i += 2
					continue
				}
				s = append(s, c)
				i++
			}
			tokens = append(tokens, Token{Type: TK_STRING, Str: string(s)})
			continue
		}

		// Numbers and symbols
		start := i
		if ch == '-' && i+1 < n && isDigit(runes[i+1]) {
			i++
		} else if ch == '+' && i+1 < n && isDigit(runes[i+1]) {
			i++
		} else if isDigit(ch) {
			// start of number
		} else if isSymbolStart(ch) {
			// start of symbol
		} else {
			// Skip unknown
			i++
			continue
		}

		for i < n {
			c := runes[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
				c == '(' || c == ')' || c == '"' || c == '\'' || c == ';' {
				break
			}
			i++
		}
		raw := string(runes[start:i])

		// Try number
		if num, err := strconv.ParseFloat(raw, 64); err == nil {
			if strings.Contains(raw, ".") {
				tokens = append(tokens, Token{Type: TK_NUMBER, Value: raw, Num: num})
			} else {
				tokens = append(tokens, Token{Type: TK_NUMBER, Value: raw, Num: num})
			}
		} else {
			// Allow '-' as symbol
			if raw == "-" {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: raw})
			} else if raw == "+" {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: raw})
			} else {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: raw})
			}
		}
	}

	tokens = append(tokens, Token{Type: TK_EOF})
	return tokens
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isSymbolStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_' ||
		ch == '!' || ch == '$' || ch == '%' || ch == '&' ||
		ch == '*' || ch == '+' || ch == '-' || ch == '.' ||
		ch == '/' || ch == ':' || ch == '<' || ch == '=' ||
		ch == '>' || ch == '?' || ch == '@' || ch == '^' || ch == '~'
}

// ---------------------------------------------------------------------------
// TokenStream
// ---------------------------------------------------------------------------

type TokenStream struct {
	tokens []Token
	pos    int
}

func NewTokenStream(tokens []Token) *TokenStream {
	return &TokenStream{tokens: tokens, pos: 0}
}

func (ts *TokenStream) Peek() Token {
	return ts.tokens[ts.pos]
}

func (ts *TokenStream) Next() Token {
	tok := ts.tokens[ts.pos]
	ts.pos++
	return tok
}

func (ts *TokenStream) IsEOF() bool {
	return ts.tokens[ts.pos].Type == TK_EOF
}

func (ts *TokenStream) Expect(typ TokenType) Token {
	tok := ts.Next()
	if tok.Type != typ {
		panic("expected token type")
	}
	return tok
}
