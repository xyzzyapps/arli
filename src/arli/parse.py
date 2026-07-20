"""Arity-driven parser for arli.

The parser uses function arities to eliminate parentheses.

Core algorithm:
- When a symbol with KNOWN arity N is encountered OUTSIDE parentheses (or in
  argument position inside parens), it becomes the head of an S-expression
  and the next N expressions are consumed as its arguments.
- Inside a parenthesized list, the FIRST element (operator position) does NOT
  use arity -- the parens themselves provide the grouping.
- Special forms (defn, fn, defn-rec) have custom parsing that always
  registers the defined function's arity at parse time.
"""

from __future__ import annotations
from typing import Any, Optional

from .types import Symbol, nil
from .tokenize import (Token, TokenStream, TOKEN_OPEN, TOKEN_CLOSE,
                       TOKEN_NUMBER, TOKEN_STRING, TOKEN_SYMBOL,
                       TOKEN_QUOTE, TOKEN_QUASIQUOTE,
                       TOKEN_UNQUOTE, TOKEN_UNQUOTE_SPLICE)


# ---------------------------------------------------------------------------
# Arity registry
# ---------------------------------------------------------------------------

class ArityTable:
    """Maps symbol names to their arity.

    Arity -1 means 'unknown / variadic' - such symbols must use parens.
    """

    def __init__(self) -> None:
        self._table: dict[str, int] = {}

    def register(self, name: str, arity: int) -> None:
        self._table[name] = arity

    def get(self, name: str) -> Optional[int]:
        return self._table.get(name)

    def has(self, name: str) -> bool:
        return name in self._table

    def clone(self) -> ArityTable:
        t = ArityTable()
        t._table = dict(self._table)
        return t


# ---------------------------------------------------------------------------
# Parser
# ---------------------------------------------------------------------------

class Parser:
    """Arity-driven S-expression parser.

    Parsing uses an ArityTable to know which symbols have known arities.
    """

    def __init__(self, arity_table: ArityTable) -> None:
        self.arity_table = arity_table

    def parse(self, source: str) -> list[Any]:
        """Parse source string into a list of top-level expressions."""
        from .tokenize import tokenize
        tokens = tokenize(source)
        stream = TokenStream(tokens)
        exprs: list[Any] = []
        while not stream.is_eof:
            expr = self._parse_expr(stream, allow_arity=True)
            if expr is not None:
                exprs.append(expr)
        return exprs

    # ------------------------------------------------------------------
    # Main parse methods
    # ------------------------------------------------------------------

    def _parse_expr(self, stream: TokenStream,
                    allow_arity: bool = True) -> Any:
        """Parse a single expression.

        Args:
            stream: Token stream.
            allow_arity: If True, known-arity symbols consume their args.
                         If False, symbols are returned as-is (used for the
                         first element inside parenthesized lists).

        Returns:
            A parsed expression (literal, Symbol, or list S-expression).
        """
        tok = stream.peek()
        if tok is None:
            return None

        # Parenthesized list
        if tok[0] == TOKEN_OPEN:
            return self._parse_paren_list(stream)

        # Quote shorthands
        if tok[0] == TOKEN_QUOTE:
            stream.next()  # consume '
            expr = self._parse_expr(stream, allow_arity=True)
            return [Symbol("quote"), expr]

        if tok[0] == TOKEN_QUASIQUOTE:
            stream.next()
            expr = self._parse_expr(stream, allow_arity=True)
            return [Symbol("quasiquote"), expr]

        if tok[0] == TOKEN_UNQUOTE:
            stream.next()
            expr = self._parse_expr(stream, allow_arity=True)
            return [Symbol("unquote"), expr]

        if tok[0] == TOKEN_UNQUOTE_SPLICE:
            stream.next()
            expr = self._parse_expr(stream, allow_arity=True)
            return [Symbol("unquote-splicing"), expr]

        # Literal atoms
        tok = stream.next()

        if tok[0] == TOKEN_NUMBER:
            return tok[1]

        if tok[0] == TOKEN_STRING:
            return tok[1]

        if tok[0] == TOKEN_SYMBOL:
            name = tok[1]

            # defn-rec uses special handler regardless of position because
            # it must register arity BEFORE parsing the body (for recursion)
            if name == "defn-rec":
                return self._parse_defn_rec(stream)

            if not allow_arity:
                # Inside parens as first element (operator position):
                # return symbol without consuming args or calling
                # special handlers. The paren list provides grouping.
                return Symbol(name)

            # defn/fn use arity-driven parsing (arity 3 and 2 respectively)

            # Check arity table
            arity = self.arity_table.get(name)
            if arity is not None and arity >= 0:
                # Known arity: consume N args
                args = []
                for _ in range(arity):
                    arg = self._parse_expr(stream, allow_arity=True)
                    if arg is None:
                        raise SyntaxError(
                            f"Expected {arity} args for {name}, "
                            f"got {len(args)}")
                    args.append(arg)
                return [Symbol(name)] + args

            # Unknown or variadic arity: return the symbol
            return Symbol(name)

        raise SyntaxError(f"Unexpected token: {tok}")

    # ------------------------------------------------------------------
    # Parenthesized list
    # ------------------------------------------------------------------

    def _parse_paren_list(self, stream: TokenStream) -> list[Any]:
        """Parse ( ... ) into a flat list.

        Inside parens:
        - The FIRST element is the operator and does NOT use arity
          (the parens define the grouping).
        - All SUBSEQUENT elements use arity-driven parsing.
        - Nested parens recurse.
        """
        stream.expect(TOKEN_OPEN)  # consume (
        items: list[Any] = []
        is_first = True

        while True:
            tok = stream.peek()
            if tok is None:
                raise SyntaxError("Unclosed parenthesis")
            if tok[0] == TOKEN_CLOSE:
                stream.next()  # consume )
                break

            # First element: allow_arity=False (operator position)
            # Subsequent elements: allow_arity=True (argument position)
            expr = self._parse_expr(stream, allow_arity=not is_first)
            if expr is not None:
                # If the first element is a defn-rec special form that
                # consumed all remaining tokens, don't double-wrap it
                if (is_first and isinstance(expr, list) and len(expr) > 0
                        and isinstance(expr[0], Symbol)
                        and expr[0].name == 'defn'
                        and stream.peek() is not None
                        and stream.peek()[0] == TOKEN_CLOSE):
                    stream.next()  # consume )
                    return expr
                items.append(expr)
            is_first = False

        return items

    # ------------------------------------------------------------------
    # Special form parsers
    # ------------------------------------------------------------------

    def _parse_name_sym(self, stream: TokenStream) -> Symbol:
        """Read the next token as a symbol name."""
        tok = stream.peek()
        if tok is None or tok[0] != TOKEN_SYMBOL:
            raise SyntaxError("Expected a symbol name")
        return Symbol(stream.next()[1])

    def _parse_params_list(self, stream: TokenStream) -> list[Symbol]:
        """Read a parenthesized parameter list into a list of Symbols."""
        params = self._parse_expr(stream, allow_arity=False)
        if not isinstance(params, list):
            raise SyntaxError("Expected a parameter list in parens")
        param_syms = []
        for p in params:
            if isinstance(p, Symbol):
                param_syms.append(p)
            else:
                raise SyntaxError(
                    f"Expected symbol in param list, got {p}")
        return param_syms

    def _parse_defn(self, stream: TokenStream) -> list[Any]:
        """Parse 'defn name (params) body...'

        Returns: [Symbol('defn'), name, params_list, *body_exprs]
        Registers arity for the defined name.
        """
        name_sym = self._parse_name_sym(stream)
        param_syms = self._parse_params_list(stream)

        # Register arity at parse time so subsequent uses work
        self.arity_table.register(name_sym.name, len(param_syms))

        # Parse body expressions
        body: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None or tok[0] == TOKEN_CLOSE:
                break
            expr = self._parse_expr(stream, allow_arity=True)
            if expr is not None:
                body.append(expr)

        return [Symbol("defn"), name_sym, param_syms] + (body or [nil])

    def _parse_fn(self, stream: TokenStream) -> list[Any]:
        """Parse 'fn (params) body...'

        Returns: [Symbol('fn'), params_list, *body_exprs]
        """
        param_syms = self._parse_params_list(stream)

        body: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None or tok[0] == TOKEN_CLOSE:
                break
            expr = self._parse_expr(stream, allow_arity=True)
            if expr is not None:
                body.append(expr)

        return [Symbol("fn"), param_syms] + (body or [nil])

    def _parse_defn_rec(self, stream: TokenStream) -> list[Any]:
        """Parse 'defn-rec name (params) body...' for recursive functions."""
        name_sym = self._parse_name_sym(stream)
        param_syms = self._parse_params_list(stream)

        # Register arity BEFORE body parsing for self-reference
        self.arity_table.register(name_sym.name, len(param_syms))

        body: list[Any] = []
        while True:
            tok = stream.peek()
            if tok is None or tok[0] == TOKEN_CLOSE:
                break
            expr = self._parse_expr(stream, allow_arity=True)
            if expr is not None:
                body.append(expr)

        return [Symbol("defn"), name_sym, param_syms] + (body or [nil])


# ---------------------------------------------------------------------------
# Convenience
# ---------------------------------------------------------------------------

def parse_source(source: str,
                 arity_table: Optional[ArityTable] = None
                 ) -> list[Any]:
    """Parse arli source code.

    Args:
        source: arli source string.
        arity_table: Optional custom arity table.

    Returns:
        List of parsed S-expressions (nested Python lists).
    """
    if arity_table is None:
        arity_table = ArityTable()
    parser = Parser(arity_table)
    return parser.parse(source)
