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
	TK_EOF              TokenType = iota
	TK_OPEN                       // (
	TK_CLOSE                      // )
	TK_VECTOR_OPEN                // [
	TK_VECTOR_CLOSE               // ]
	TK_MAP_OPEN                   // {
	TK_MAP_CLOSE                  // }
	TK_NUMBER                     // 42, 3.14, 0xFF, 0o77, 0b1010
	TK_STRING                     // "hello"
	TK_SYMBOL                     // foo, +, define
	TK_KEYWORD                    // :keyword
	TK_QUOTE                      // '
	TK_QUASIQUOTE                 // `
	TK_UNQUOTE                    // ,
	TK_UNQUOTE_SPLICE             // ,@
)

type Token struct {
	Type  TokenType
	Value string
	Num   float64   // parsed number value (for TK_NUMBER)
	Str   string    // parsed string content (for TK_STRING)
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

		// Brackets
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
		if ch == '[' {
			tokens = append(tokens, Token{Type: TK_VECTOR_OPEN, Value: "["})
			i++
			continue
		}
		if ch == ']' {
			tokens = append(tokens, Token{Type: TK_VECTOR_CLOSE, Value: "]"})
			i++
			continue
		}
		if ch == '{' {
			tokens = append(tokens, Token{Type: TK_MAP_OPEN, Value: "{"})
			i++
			continue
		}
		if ch == '}' {
			tokens = append(tokens, Token{Type: TK_MAP_CLOSE, Value: "}"})
			i++
			continue
		}

		// Quote shorthands
		if ch == '\'' {
			tokens = append(tokens, Token{Type: TK_QUOTE, Value: "'"})
			i++
			continue
		}
		if ch == '`' {
			tokens = append(tokens, Token{Type: TK_QUASIQUOTE, Value: "`"})
			i++
			continue
		}
		if ch == ',' {
			if i+1 < n && runes[i+1] == '@' {
				tokens = append(tokens, Token{Type: TK_UNQUOTE_SPLICE, Value: ",@"})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TK_UNQUOTE, Value: ","})
				i++
			}
			continue
		}

		// Strings (regular and triple-quoted)
		if ch == '"' {
			// Check for triple-quoted string """
			if i+2 < n && runes[i+1] == '"' && runes[i+2] == '"' {
				i += 3 // skip opening """
				var s []rune
				for i < n {
					if i+2 < n && runes[i] == '"' && runes[i+1] == '"' && runes[i+2] == '"' {
						i += 3 // skip closing """
						break
					}
					if runes[i] == '\\' && i+1 < n {
						next := runes[i+1]
						switch next {
						case 'n':
							s = append(s, '\n'); i += 2
						case 't':
							s = append(s, '\t'); i += 2
						case 'r':
							s = append(s, '\r'); i += 2
						case '"':
							s = append(s, '"'); i += 2
						case '\\':
							s = append(s, '\\'); i += 2
						default:
							s = append(s, runes[i]); i++
						}
					} else {
						s = append(s, runes[i])
						i++
					}
				}
				tokens = append(tokens, Token{Type: TK_STRING, Str: string(s)})
			} else {
				// Regular string
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
			}
			continue
		}

		// Keywords (:keyword)
		if ch == ':' {
			start := i
			i++
			if i < n && (unicode.IsLetter(runes[i]) || runes[i] == '_' ||
				runes[i] == '!' || runes[i] == '$' || runes[i] == '%' ||
				runes[i] == '&' || runes[i] == '*' || runes[i] == '.' ||
				runes[i] == '/' || runes[i] == '<' || runes[i] == '=' ||
				runes[i] == '>' || runes[i] == '?' || runes[i] == '@' ||
				runes[i] == '^' || runes[i] == '~') {
				for i < n {
					c := runes[i]
					if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
						c == '(' || c == ')' || c == '[' || c == ']' ||
						c == '{' || c == '}' || c == '"' || c == '\'' ||
						c == '`' || c == ';' || c == ',' {
						break
					}
					i++
				}
				tokens = append(tokens, Token{Type: TK_KEYWORD, Value: string(runes[start:i])})
			} else {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: ":"})
			}
			continue
		}

		// Numbers and symbols
		start := i
		if ch == '-' && i+1 < n && isDigit(runes[i+1]) {
			i++
		} else if ch == '+' && i+1 < n && isDigit(runes[i+1]) {
			i++
		} else if isDigit(ch) || ch == '0' {
			// start of number (including hex/octal/bin)
		} else if isSymbolStart(ch) {
			// start of symbol
		} else {
			i++
			continue
		}

		for i < n {
			c := runes[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
				c == '(' || c == ')' || c == '[' || c == ']' ||
				c == '{' || c == '}' || c == '"' || c == '\'' ||
				c == '`' || c == ';' || c == ',' {
				break
			}
			i++
		}
		raw := string(runes[start:i])

		// Try hex/octal/binary numbers
		if num := parseSpecialNumber(raw); num != nil {
			tokens = append(tokens, Token{Type: TK_NUMBER, Value: raw, Num: *num})
		} else if num, err := strconv.ParseFloat(raw, 64); err == nil {
			tokens = append(tokens, Token{Type: TK_NUMBER, Value: raw, Num: num})
		} else {
			// Allow '-' and '+' as symbols
			if raw == "-" || raw == "+" {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: raw})
			} else {
				tokens = append(tokens, Token{Type: TK_SYMBOL, Value: raw})
			}
		}
	}

	tokens = append(tokens, Token{Type: TK_EOF})
	return tokens
}

func parseSpecialNumber(s string) *float64 {
	var base int
	var val string
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		val = s[2:]
	} else if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		base = 8
		val = s[2:]
	} else if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		base = 2
		val = s[2:]
	} else {
		return nil
	}
	n, err := strconv.ParseInt(val, base, 64)
	if err != nil {
		return nil
	}
	f := float64(n)
	return &f
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isSymbolStart(ch rune) bool {
	// Accept Unicode letters and symbols (Greek, Cyrillic, Chinese, math, etc.)
	if unicode.IsLetter(ch) || unicode.IsSymbol(ch) {
		return true
	}
	// ASCII symbol characters
	return ch == '_' || ch == '!' || ch == '$' || ch == '%' || ch == '&' ||
		ch == '*' || ch == '+' || ch == '-' || ch == '.' ||
		ch == '/' || ch == '<' || ch == '=' ||
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
		panic("expected token type " + strconv.Itoa(int(typ)) + " got " + strconv.Itoa(int(tok.Type)))
	}
	return tok
}
