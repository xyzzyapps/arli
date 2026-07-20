"""Tokenizer for arli.

Converts source text into a flat list of tokens.
Tokens are: numbers, strings, symbols, and punctuation ().
"""

from __future__ import annotations
from typing import Optional

# ---------------------------------------------------------------------------
# Token types
# ---------------------------------------------------------------------------

TOKEN_OPEN = "("
TOKEN_CLOSE = ")"
TOKEN_STRING = "STRING"
TOKEN_NUMBER = "NUMBER"
TOKEN_SYMBOL = "SYMBOL"
TOKEN_QUOTE = "'"
TOKEN_QUASIQUOTE = "`"
TOKEN_UNQUOTE = ","
TOKEN_UNQUOTE_SPLICE = ",@"

Token = tuple[str, str | int | float]  # (type, value)


# ---------------------------------------------------------------------------
# Tokenizer
# ---------------------------------------------------------------------------

def tokenize(source: str) -> list[Token]:
    """Tokenize arli source code into a list of tokens.

    Handles:
    - integers and floats (including negative via '-')
    - double-quoted strings with escape sequences
    - symbols, including '->', '...', etc.
    - parentheses () for grouping unknown-arity expressions
    - line comments starting with ';'
    - quote shorthand 'x for (quote x)
    """
    tokens: list[Token] = []
    i = 0
    length = len(source)

    while i < length:
        ch = source[i]

        # Whitespace
        if ch in ' \t\n\r\f':
            i += 1
            continue

        # Line comments
        if ch == ';':
            while i < length and source[i] not in '\n\r':
                i += 1
            continue

        # Parentheses
        if ch == '(':
            tokens.append((TOKEN_OPEN, '('))
            i += 1
            continue
        if ch == ')':
            tokens.append((TOKEN_CLOSE, ')'))
            i += 1
            continue

        # Quote shorthands
        if ch == "'":
            tokens.append((TOKEN_QUOTE, "'"))
            i += 1
            continue
        if ch == '`':
            tokens.append((TOKEN_QUASIQUOTE, "`"))
            i += 1
            continue
        if ch == ',':
            if i + 1 < length and source[i + 1] == '@':
                tokens.append((TOKEN_UNQUOTE_SPLICE, ",@"))
                i += 2
            else:
                tokens.append((TOKEN_UNQUOTE, ","))
                i += 1
            continue

        # Strings
        if ch == '"':
            i += 1
            s: list[str] = []
            while i < length:
                c = source[i]
                if c == '"':
                    i += 1
                    break
                if c == '\\' and i + 1 < length:
                    esc = source[i + 1]
                    if esc == 'n':
                        s.append('\n')
                        i += 2
                    elif esc == 't':
                        s.append('\t')
                        i += 2
                    elif esc == 'r':
                        s.append('\r')
                        i += 2
                    elif esc == '"':
                        s.append('"')
                        i += 2
                    elif esc == '\\':
                        s.append('\\')
                        i += 2
                    else:
                        s.append(c)
                        i += 1
                else:
                    s.append(c)
                    i += 1
            tokens.append((TOKEN_STRING, ''.join(s)))
            continue

        # Numbers and symbols
        start = i
        if ch == '-':
            # Check if next char is a digit — negative number
            if i + 1 < length and source[i + 1].isdigit():
                i += 1
            else:
                # It's a symbol starting with '-' (like '->')
                pass
        elif ch == '+':
            if i + 1 < length and source[i + 1].isdigit():
                i += 1
        elif ch.isdigit():
            pass
        elif ch.isalpha() or ch in '!$%&*./:<=>?@^_~':
            pass
        else:
            # Skip unknown character
            i += 1
            continue

        # Read the full token
        while i < length:
            c = source[i]
            if c in ' \t\n\r()"\'`;,':
                break
            i += 1
        token_str = source[start:i]

        # Try to parse as number
        if token_str.startswith('-') and len(token_str) == 1:
            # Just '-', treat as symbol
            tokens.append((TOKEN_SYMBOL, '-'))
        elif token_str.startswith('+') and len(token_str) == 1:
            tokens.append((TOKEN_SYMBOL, '+'))
        else:
            num = _parse_number(token_str)
            if num is not None:
                tokens.append((TOKEN_NUMBER, num))
            else:
                tokens.append((TOKEN_SYMBOL, token_str))

    return tokens


def _parse_number(s: str) -> Optional[int | float]:
    """Try to parse a number. Returns None if not a valid number."""
    try:
        if '.' in s:
            return float(s)
        return int(s)
    except (ValueError, TypeError):
        return None


# ---------------------------------------------------------------------------
# Token stream helper
# ---------------------------------------------------------------------------

class TokenStream:
    """A stream of tokens with position tracking."""

    def __init__(self, tokens: list[Token]) -> None:
        self.tokens = tokens
        self.pos = 0

    @property
    def is_eof(self) -> bool:
        return self.pos >= len(self.tokens)

    def peek(self) -> Optional[Token]:
        if self.is_eof:
            return None
        return self.tokens[self.pos]

    def next(self) -> Optional[Token]:
        if self.is_eof:
            return None
        tok = self.tokens[self.pos]
        self.pos += 1
        return tok

    def expect(self, expected_type: str) -> Token:
        tok = self.next()
        if tok is None:
            raise SyntaxError(f"Expected {expected_type}, got end of input")
        if tok[0] != expected_type:
            raise SyntaxError(f"Expected {expected_type}, got {tok}")
        return tok

    def __len__(self) -> int:
        return len(self.tokens)
