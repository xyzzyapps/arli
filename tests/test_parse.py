"""Tests for the arli arity-driven parser."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.arli.parse import Parser, ArityTable, parse_source
from src.arli.types import Symbol


def test_arity_known():
    """Known arity functions don't need parentheses."""
    table = ArityTable()
    table.register("+", 2)
    table.register("*", 2)
    p = Parser(table)

    result = p.parse("+ 1 2")
    assert len(result) == 1
    assert result[0] == [Symbol("+"), 1, 2]


def test_nested_arity():
    """Nested arity-driven expressions."""
    table = ArityTable()
    table.register("+", 2)
    table.register("*", 2)
    p = Parser(table)

    result = p.parse("* + 1 2 3")
    assert len(result) == 1
    # * has arity 2, consumes [+ 1 2] and 3
    assert result[0] == [Symbol("*"), [Symbol("+"), 1, 2], 3]


def test_paren_list():
    """Parenthesized lists for unknown/variadic arity."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("(if cond a b)")
    assert len(result) == 1
    assert result[0] == [Symbol("if"), Symbol("cond"),
                         Symbol("a"), Symbol("b")]


def test_nested_parens():
    """Nested parenthesized lists."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("(define (f x) (+ x 1))")
    assert len(result) == 1
    assert result[0][0] == Symbol("define")
    assert result[0][1] == [Symbol("f"), Symbol("x")]


def test_quote():
    """Quote shorthand."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("'x")
    assert result[0] == [Symbol("quote"), Symbol("x")]


def test_mixed():
    """Mixed arity-driven and parenthesized."""
    table = ArityTable()
    table.register("+", 2)
    table.register("print", 1)
    p = Parser(table)

    result = p.parse("print + 1 2")
    assert result[0] == [Symbol("print"), [Symbol("+"), 1, 2]]


def test_defn_parsed_as_arity3():
    """defn at top-level (no parens) uses arity 3 — produces flat list."""
    table = ArityTable()
    table.register("defn", 3)
    table.register("+", 2)
    p = Parser(table)

    result = p.parse("defn add (x y) + x y")
    assert len(result) == 1
    expr = result[0]
    assert len(expr) == 4  # [defn, add, [x, y], [+, x, y]]
    assert isinstance(expr[0], Symbol) and expr[0].name == "defn"
    assert isinstance(expr[3], list)  # body is [+, x, y]
    assert expr[3][0].name == "+"


def test_defn_inside_parens():
    """defn inside parens produces a flat list for the evaluator."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("(defn add (x y) (+ x y))")
    assert len(result) == 1
    # Flat list: [defn, add, [x, y], [+, x, y]]
    expr = result[0]
    assert len(expr) == 4
    assert expr[0] == Symbol("defn")
    assert expr[1] == Symbol("add")
    assert expr[2] == [Symbol("x"), Symbol("y")]


def test_literals():
    """Numbers and strings as atoms."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("42 -1 3.14 \"hello\"")
    assert len(result) == 4
    assert result[0] == 42
    assert result[1] == -1
    assert result[2] == 3.14
    assert result[3] == "hello"


def test_empty_parens():
    """Empty parentheses produce empty list."""
    table = ArityTable()
    p = Parser(table)

    result = p.parse("()")
    assert result[0] == []


if __name__ == "__main__":
    test_arity_known()
    test_nested_arity()
    test_paren_list()
    test_nested_parens()
    test_quote()
    test_mixed()
    test_defn_parsed_as_arity3()
    test_defn_inside_parens()
    test_literals()
    test_empty_parens()
    print("All parser tests passed!")
