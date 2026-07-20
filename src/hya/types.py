"""Core data types for Hya.

Hya's type system is minimal:
- Numbers (int, float)
- Strings
- Symbols
- Lists (for parenthesized expressions, data, or function bodies)
- Nil (the empty list / false value)
- Builtin (Python functions wrapped with arity metadata)
- Function (user-defined closures)
"""

from __future__ import annotations
from typing import Any, Optional


class HyaType:
    """Base type for all Hya runtime objects."""
    pass


class Symbol(HyaType):
    """A named symbol, used for variable references and special forms."""

    def __init__(self, name: str) -> None:
        self.name = name

    def __repr__(self) -> str:
        return f"<Symbol {self.name}>"

    def __str__(self) -> str:
        return self.name

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Symbol):
            return self.name == other.name
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self.name)


# Sentinel for the empty list / false value
class NilType(HyaType):
    """Represents the empty list () and is the canonical false value."""

    _instance: Optional[NilType] = None

    def __new__(cls) -> NilType:
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __repr__(self) -> str:
        return "nil"

    def __bool__(self) -> bool:
        return False


nil = NilType()


class Builtin(HyaType):
    """A Python function wrapped with arity metadata.

    The arity tells the parser how many expressions to consume as arguments.
    Arity of -1 means 'unknown/variadic' and requires parentheses.
    """

    def __init__(self, name: str, fn: callable, arity: int,
                 doc: str = "") -> None:
        self.name = name
        self.fn = fn
        self.arity = arity  # -1 means variadic / unknown
        self.__doc__ = doc or fn.__doc__ or ""

    def __call__(self, *args: Any, evaluator=None) -> Any:
        return self.fn(*args, evaluator=evaluator)

    def __repr__(self) -> str:
        return f"<Builtin {self.name} arity={self.arity}>"


class Function(HyaType):
    """A user-defined closure with known arity."""

    def __init__(self, params: list[Symbol], body: Any,
                 env: dict, name: str = "") -> None:
        self.params = params
        self.body = body
        self.env = env  # closure environment
        self.name = name
        self.arity = len(params)

    def __repr__(self) -> str:
        return f"<Function {self.name or 'anon'} arity={self.arity}>"


def is_truthy(val: Any) -> bool:
    """Check truthiness. Only nil is false."""
    return val is not nil and val is not False


def hya_repr(val: Any) -> str:
    """Convert a Hya value to its string representation."""
    if val is nil:
        return "nil"
    if isinstance(val, Symbol):
        return val.name
    if isinstance(val, list):
        if not val:
            return "()"
        return "(" + " ".join(hya_repr(v) for v in val) + ")"
    if isinstance(val, bool):
        return "true" if val else "false"
    if isinstance(val, float):
        if val == int(val):
            return str(int(val))
        return str(val)
    if isinstance(val, int):
        return str(val)
    if isinstance(val, str):
        return f'"{val}"'
    if isinstance(val, Builtin):
        return f"<builtin {val.name}>"
    if isinstance(val, Function):
        return f"<fn {val.name or 'anon'}>"
    return str(val)
