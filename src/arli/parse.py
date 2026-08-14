"""Arity-driven parser for arli."""

from __future__ import annotations
from typing import Any, Optional

from .types import Symbol, nil
from .tokenize import (Token, TokenStream, TOKEN_OPEN, TOKEN_CLOSE,
                       TOKEN_VECTOR_OPEN, TOKEN_VECTOR_CLOSE,
                       TOKEN_MAP_OPEN, TOKEN_MAP_CLOSE,
                       TOKEN_NUMBER, TOKEN_STRING, TOKEN_SYMBOL,
                       TOKEN_QUOTE, TOKEN_QUASIQUOTE,
                       TOKEN_UNQUOTE, TOKEN_UNQUOTE_SPLICE,
                       TOKEN_KEYWORD)


class ArityTable:
    """Maps symbol names to their arity. -1 = variadic."""
    def __init__(self) -> None:
        self._table: dict[str, int] = {}
    def register(self, name: str, arity: int) -> None:
        self._table[name] = arity
    def get(self, name: str) -> Optional[int]:
        return self._table.get(name)
    def has(self, name: str) -> bool:
        return name in self._table
    def clone(self) -> ArityTable:
        t = ArityTable(); t._table = dict(self._table); return t


class Parser:
    def __init__(self, arity_table: ArityTable) -> None:
        self.arity_table = arity_table

    def parse(self, source: str) -> list[Any]:
        from .tokenize import tokenize
        tokens = tokenize(source)
        stream = TokenStream(tokens)
        exprs: list[Any] = []
        while not stream.is_eof:
            expr = self._parse_expr(stream, allow_arity=True)
            if expr is not None:
                exprs.append(expr)
        return exprs

    def _parse_expr(self, stream: TokenStream, allow_arity: bool = True) -> Any:
        tok = stream.peek()
        if tok is None:
            return None

        tt = tok[0]
        if tt == TOKEN_OPEN:
            return self._parse_paren_list(stream)
        if tt == TOKEN_VECTOR_OPEN:
            return self._parse_bracket_list(stream)
        if tt == TOKEN_MAP_OPEN:
            return self._parse_map_literal(stream)
        if tt == TOKEN_QUOTE:
            stream.next(); expr = self._parse_expr(stream, False)
            return [Symbol("quote"), expr]
        if tt == TOKEN_QUASIQUOTE:
            stream.next(); expr = self._parse_expr(stream, True)
            return [Symbol("quasiquote"), expr]
        if tt == TOKEN_UNQUOTE:
            stream.next(); expr = self._parse_expr(stream, True)
            return [Symbol("unquote"), expr]
        if tt == TOKEN_UNQUOTE_SPLICE:
            stream.next(); expr = self._parse_expr(stream, True)
            return [Symbol("unquote-splicing"), expr]

        tok = stream.next()
        if tt == TOKEN_NUMBER:
            return tok[1]
        if tt == TOKEN_STRING:
            return tok[1]
        if tt == TOKEN_KEYWORD:
            return Symbol(tok[1])
        if tt == TOKEN_SYMBOL:
            name = tok[1]
            if name in ("defn", "defn-rec"):
                return self._parse_defn_rec(stream)
            if name == "defn-fexpr":
                return self._parse_defn_fexpr(stream)
            if not allow_arity:
                return Symbol(name)
            arity = self.arity_table.get(name)
            if arity is not None and arity >= 0:
                args = []
                for _ in range(arity):
                    arg = self._parse_expr(stream, True)
                    if arg is None:
                        raise SyntaxError(f"Expected {arity} args for {name}, got {len(args)}")
                    args.append(arg)
                return [Symbol(name)] + args
            return Symbol(name)

        raise SyntaxError(f"Unexpected token: {tok}")

    def _parse_paren_list(self, stream: TokenStream) -> list[Any]:
        stream.expect(TOKEN_OPEN)
        items: list[Any] = []
        is_first = True
        # When the first element is quote, disable arity expansion for all
        # subsequent elements (so '(+ 1 2) parses without + consuming args)
        quoted_form = False
        while True:
            tok = stream.peek()
            if tok is None:
                raise SyntaxError("Unclosed parenthesis")
            if tok[0] == TOKEN_CLOSE:
                stream.next(); break
            allow_arity = not is_first and not quoted_form
            expr = self._parse_expr(stream, allow_arity)
            if expr is not None:
                # After parsing the first element, check if it's 'quote'
                if is_first and isinstance(expr, Symbol) and expr.name == 'quote':
                    quoted_form = True
                if (is_first and isinstance(expr, list) and len(expr) > 0
                        and isinstance(expr[0], Symbol) and expr[0].name in ('defn', 'defn-rec', 'defn-fexpr')
                        and stream.peek() is not None and stream.peek()[0] == TOKEN_CLOSE):
                    stream.next(); return expr
                items.append(expr)
            is_first = False
        return items

    def _parse_bracket_list(self, stream: TokenStream) -> list[Any]:
        """Parse [a b c] -> [Symbol('list'), a, b, c]"""
        stream.expect(TOKEN_VECTOR_OPEN)
        items: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None:
                raise SyntaxError("Unclosed bracket")
            if tok[0] == TOKEN_VECTOR_CLOSE:
                stream.next(); break
            expr = self._parse_expr(stream, True)
            if expr is not None:
                items.append(expr)
        return [Symbol("list")] + items

    def _parse_map_literal(self, stream: TokenStream) -> list[Any]:
        """Parse {:a 1 :b 2} -> [Symbol('hash-map'), :a, 1, :b, 2]"""
        stream.expect(TOKEN_MAP_OPEN)
        items: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None:
                raise SyntaxError("Unclosed map brace")
            if tok[0] == TOKEN_MAP_CLOSE:
                stream.next(); break
            expr = self._parse_expr(stream, True)
            if expr is not None:
                items.append(expr)
        return [Symbol("hash-map")] + items

    def _parse_name_sym(self, stream: TokenStream) -> Symbol:
        tok = stream.peek()
        if tok is None or tok[0] != TOKEN_SYMBOL:
            raise SyntaxError("Expected a symbol name")
        return Symbol(stream.next()[1])

    def _parse_params_list(self, stream: TokenStream) -> list[Symbol]:
        params = self._parse_expr(stream, False)
        if not isinstance(params, list):
            raise SyntaxError("Expected a parameter list in parens")
        param_syms = []
        for p in params:
            if isinstance(p, Symbol):
                param_syms.append(p)
            else:
                raise SyntaxError(f"Expected symbol in param list, got {p}")
        return param_syms

    def _parse_defn(self, stream: TokenStream) -> list[Any]:
        return self._parse_defn_rec(stream)

    def _parse_fn(self, stream: TokenStream) -> list[Any]:
        param_syms = self._parse_params_list(stream)
        body: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None or tok[0] in (TOKEN_CLOSE, TOKEN_VECTOR_CLOSE, TOKEN_MAP_CLOSE):
                break
            expr = self._parse_expr(stream, True)
            if expr is not None:
                body.append(expr)
        return [Symbol("fn"), param_syms] + (body or [nil])

    def _parse_defn_rec(self, stream: TokenStream) -> list[Any]:
        name_sym = self._parse_name_sym(stream)
        param_syms = self._parse_params_list(stream)
        self.arity_table.register(name_sym.name, len(param_syms))
        # Parse one body expression (like defn arity 3). Multiple expressions
        # use (do ...) — consistent with defn at top level.
        body: list[Any] = []
        tok = stream.peek()
        if tok is not None and tok[0] not in (TOKEN_CLOSE, TOKEN_VECTOR_CLOSE, TOKEN_MAP_CLOSE):
            expr = self._parse_expr(stream, True)
            if expr is not None:
                body.append(expr)
        return [Symbol("defn"), name_sym, param_syms] + (body or [nil])

    def _parse_defn_fexpr(self, stream: TokenStream) -> list[Any]:
        name_sym = self._parse_name_sym(stream)
        param_syms = self._parse_params_list(stream)
        self.arity_table.register(name_sym.name, len(param_syms))
        body: list[Any] = []
        tok = stream.peek()
        if tok is not None and tok[0] not in (TOKEN_CLOSE, TOKEN_VECTOR_CLOSE, TOKEN_MAP_CLOSE):
            expr = self._parse_expr(stream, True)
            if expr is not None:
                body.append(expr)
        return [Symbol("defn-fexpr"), name_sym, param_syms] + (body or [nil])


def parse_source(source: str, arity_table: Optional[ArityTable] = None) -> list[Any]:
    if arity_table is None:
        arity_table = ArityTable()
    return Parser(arity_table).parse(source)
