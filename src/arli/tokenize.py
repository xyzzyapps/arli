"""Tokenizer for arli.

Converts source text into a flat list of tokens.
Tokens are: numbers, strings, symbols, and punctuation ()[]{}.
"""

from __future__ import annotations
from typing import Optional
import unicodedata

# Token types
TOKEN_OPEN = "("
TOKEN_CLOSE = ")"
TOKEN_VECTOR_OPEN = "["
TOKEN_VECTOR_CLOSE = "]"
TOKEN_MAP_OPEN = "{"
TOKEN_MAP_CLOSE = "}"
TOKEN_STRING = "STRING"
TOKEN_NUMBER = "NUMBER"
TOKEN_SYMBOL = "SYMBOL"
TOKEN_QUOTE = "'"
TOKEN_QUASIQUOTE = "`"
TOKEN_UNQUOTE = ","
TOKEN_UNQUOTE_SPLICE = ",@"
TOKEN_KEYWORD = "KEYWORD"

Token = tuple[str, str | int | float]


def _is_symbol_start(ch: str) -> bool:
    """Check if a character can start a symbol name."""
    if ch.isalpha() or ch.isidentifier():
        return True
    try:
        cat = unicodedata.category(ch)
        if cat.startswith('S'):
            return True
    except ValueError:
        pass
    return ch in '!$%&*./:<=>?@^_~'


def tokenize(source: str) -> list[Token]:
    tokens: list[Token] = []
    i = 0
    length = len(source)

    while i < length:
        ch = source[i]

        # Whitespace
        if ch in ' \t\n\r\f':
            i += 1
            continue

        # Comments
        if ch == ';':
            while i < length and source[i] not in '\n\r':
                i += 1
            continue

        # Brackets
        if ch == '(':
            tokens.append((TOKEN_OPEN, '('))
            i += 1
            continue
        if ch == ')':
            tokens.append((TOKEN_CLOSE, ')'))
            i += 1
            continue
        if ch == '[':
            tokens.append((TOKEN_VECTOR_OPEN, '['))
            i += 1
            continue
        if ch == ']':
            tokens.append((TOKEN_VECTOR_CLOSE, ']'))
            i += 1
            continue
        if ch == '{':
            tokens.append((TOKEN_MAP_OPEN, '{'))
            i += 1
            continue
        if ch == '}':
            tokens.append((TOKEN_MAP_CLOSE, '}'))
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

        # Strings (regular and triple-quoted)
        if ch == '"':
            # Check for triple-quoted string """
            if i + 2 < length and source[i+1] == '"' and source[i+2] == '"':
                i += 3  # skip opening """
                s: list[str] = []
                while i < length:
                    if i + 2 < length and source[i] == '"' and source[i+1] == '"' and source[i+2] == '"':
                        i += 3  # skip closing """
                        break
                    if source[i] == '\\' and i + 1 < length:
                        esc = source[i + 1]
                        if esc == 'n':
                            s.append('\n'); i += 2
                        elif esc == 't':
                            s.append('\t'); i += 2
                        elif esc == 'r':
                            s.append('\r'); i += 2
                        elif esc == '"':
                            s.append('"'); i += 2
                        elif esc == '\\':
                            s.append('\\'); i += 2
                        else:
                            s.append(source[i]); i += 1
                    else:
                        s.append(source[i])
                        i += 1
                tokens.append((TOKEN_STRING, ''.join(s)))
            else:
                # Regular string
                i += 1
                s = []
                while i < length:
                    c = source[i]
                    if c == '"':
                        i += 1
                        break
                    if c == '\\' and i + 1 < length:
                        esc = source[i + 1]
                        if esc == 'n':
                            s.append('\n'); i += 2
                        elif esc == 't':
                            s.append('\t'); i += 2
                        elif esc == 'r':
                            s.append('\r'); i += 2
                        elif esc == '"':
                            s.append('"'); i += 2
                        elif esc == '\\':
                            s.append('\\'); i += 2
                        else:
                            s.append(c); i += 1
                    else:
                        s.append(c)
                        i += 1
                tokens.append((TOKEN_STRING, ''.join(s)))
            continue

        # Keywords (:keyword)
        if ch == ':':
            start = i
            i += 1
            if i < length and (source[i].isalpha() or source[i] in '_!$%&*./<=>?@^~'):
                while i < length:
                    c = source[i]
                    if c in ' \t\n\r()[]{}"\'`;,':
                        break
                    i += 1
                tokens.append((TOKEN_KEYWORD, source[start:i]))
            else:
                tokens.append((TOKEN_SYMBOL, ':'))
            continue

        # Numbers and symbols
        start = i
        if ch == '-':
            if i + 1 < length and source[i + 1].isdigit():
                i += 1
            else:
                pass
        elif ch == '+':
            if i + 1 < length and source[i + 1].isdigit():
                i += 1
        elif ch.isdigit():
            pass
        elif _is_symbol_start(ch):
            pass
        else:
            i += 1
            continue

        while i < length:
            c = source[i]
            if c in ' \t\n\r()[]{}"\'`;,':
                break
            i += 1
        token_str = source[start:i]

        if token_str.startswith('-') and len(token_str) == 1:
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
    try:
        if s.startswith('0x') or s.startswith('0X'):
            return int(s, 16)
        if s.startswith('0o') or s.startswith('0O'):
            return int(s, 8)
        if s.startswith('0b') or s.startswith('0B'):
            return int(s, 2)
        if '.' in s:
            return float(s)
        return int(s)
    except (ValueError, TypeError):
        return None


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
