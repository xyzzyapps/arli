"""Tests for the arli tokenizer."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.arli.tokenize import tokenize, TOKEN_NUMBER, TOKEN_SYMBOL, TOKEN_STRING, TOKEN_OPEN, TOKEN_CLOSE
from src.arli.tokenize import TOKEN_QUOTE, TOKEN_QUASIQUOTE, TOKEN_UNQUOTE, TOKEN_UNQUOTE_SPLICE


def test_numbers():
    tokens = tokenize("42 -17 3.14 +5")
    assert len(tokens) == 4
    assert tokens[0] == (TOKEN_NUMBER, 42)
    assert tokens[1] == (TOKEN_NUMBER, -17)
    assert tokens[2] == (TOKEN_NUMBER, 3.14)
    assert tokens[3] == (TOKEN_NUMBER, 5)


def test_symbols():
    tokens = tokenize("foo bar->baz hello-world")
    assert len(tokens) == 3
    assert tokens[0] == (TOKEN_SYMBOL, "foo")
    assert tokens[1] == (TOKEN_SYMBOL, "bar->baz")
    assert tokens[2] == (TOKEN_SYMBOL, "hello-world")


def test_strings():
    tokens = tokenize('"hello" "world\\n"')
    assert len(tokens) == 2
    assert tokens[0] == (TOKEN_STRING, "hello")
    assert tokens[1] == (TOKEN_STRING, "world\n")


def test_parens():
    tokens = tokenize("(+ 1 2)")
    assert len(tokens) == 5
    assert tokens[0] == (TOKEN_OPEN, "(")
    assert tokens[1] == (TOKEN_SYMBOL, "+")
    assert tokens[2] == (TOKEN_NUMBER, 1)
    assert tokens[3] == (TOKEN_NUMBER, 2)
    assert tokens[4] == (TOKEN_CLOSE, ")")


def test_quotes():
    tokens = tokenize("'x `y ,z ,@w")
    assert len(tokens) == 8
    assert tokens[0] == (TOKEN_QUOTE, "'")
    assert tokens[1] == (TOKEN_SYMBOL, "x")
    assert tokens[2] == (TOKEN_QUASIQUOTE, "`")
    assert tokens[3] == (TOKEN_SYMBOL, "y")
    assert tokens[4] == (TOKEN_UNQUOTE, ",")
    assert tokens[5] == (TOKEN_SYMBOL, "z")
    assert tokens[6] == (TOKEN_UNQUOTE_SPLICE, ",@")
    assert tokens[7] == (TOKEN_SYMBOL, "w")


def test_comment():
    tokens = tokenize("+ 1 2 ; this is a comment\n+ 3 4")
    assert len(tokens) == 6


def test_empty():
    tokens = tokenize("")
    assert tokens == []


if __name__ == "__main__":
    test_numbers()
    test_symbols()
    test_strings()
    test_parens()
    test_quotes()
    test_comment()
    test_empty()
    print("All tokenizer tests passed!")
