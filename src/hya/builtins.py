"""Built-in functions for Hya.

Each builtin has a known arity. Arity -1 means 'unknown/variadic' and
requires parentheses in source.

This module defines:
- Arithmetic: + - * / // %
- Comparison: = < > <= >= !=
- Logic: and or not
- Stack operations (Forth-like): dup swap drop over rot nip tuck
- List operations: cons car cdr list
- I/O: print
- Type checking: number? string? list? symbol? fn?
"""

from __future__ import annotations
from typing import Any, Optional

from .types import (Symbol, nil, Builtin, Function, is_truthy,
                    nil, hya_repr)


# ---------------------------------------------------------------------------
# Arithmetic (all arity 2)
# ---------------------------------------------------------------------------

def _add(a, b, evaluator=None):
    return a + b


def _sub(a, b, evaluator=None):
    return a - b


def _mul(a, b, evaluator=None):
    return a * b


def _div(a, b, evaluator=None):
    return a / b


def _floordiv(a, b, evaluator=None):
    return a // b


def _mod(a, b, evaluator=None):
    return a % b


def _neg(a, evaluator=None):
    return -a


# ---------------------------------------------------------------------------
# Comparison (all arity 2)
# ---------------------------------------------------------------------------

def _eq(a, b, evaluator=None):
    return a == b


def _lt(a, b, evaluator=None):
    return a < b


def _gt(a, b, evaluator=None):
    return a > b


def _le(a, b, evaluator=None):
    return a <= b


def _ge(a, b, evaluator=None):
    return a >= b


def _ne(a, b, evaluator=None):
    return a != b


# ---------------------------------------------------------------------------
# Logic
# ---------------------------------------------------------------------------

def _and(*args, evaluator=None):
    """Short-circuit AND. Variadic — must use parens."""
    for arg in args:
        if not is_truthy(arg):
            return arg
    return args[-1] if args else True


def _or(*args, evaluator=None):
    """Short-circuit OR. Variadic — must use parens."""
    for arg in args:
        if is_truthy(arg):
            return arg
    return nil


def _not(a, evaluator=None):
    return not is_truthy(a)


# ---------------------------------------------------------------------------
# Stack operations (Forth-like)
#
# These manipulate the evaluator's data stack directly.
# They have known arities so don't need parens.
# ---------------------------------------------------------------------------

def _dup(a, evaluator=None):
    """Duplicate top of stack."""
    if evaluator is not None:
        evaluator.stack.append(a)
    return a


def _swap(a, b, evaluator=None):
    """Swap top two items on stack — returns (b, a)."""
    if evaluator is not None:
        evaluator.stack.pop()  # remove b
        evaluator.stack.pop()  # remove a
        evaluator.stack.append(b)
        evaluator.stack.append(a)
    return b, a


def _drop(a, evaluator=None):
    """Drop top of stack. Returns nil."""
    return nil


def _over(a, b, evaluator=None):
    """Duplicate second item on stack: (a b) -> (a b a)."""
    if evaluator is not None:
        evaluator.stack.append(a)
    return a


def _rot(a, b, c, evaluator=None):
    """Rotate top three: (a b c) -> (b c a)."""
    if evaluator is not None:
        evaluator.stack.pop()
        evaluator.stack.pop()
        evaluator.stack.pop()
        evaluator.stack.append(b)
        evaluator.stack.append(c)
        evaluator.stack.append(a)
    return b, c, a


def _nip(a, b, evaluator=None):
    """Drop second item: (a b) -> (b)."""
    if evaluator is not None:
        evaluator.stack.pop()
        evaluator.stack.pop()
        evaluator.stack.append(b)
    return b


def _tuck(a, b, evaluator=None):
    """Duplicate top under second: (a b) -> (b a b)."""
    if evaluator is not None:
        evaluator.stack.pop()
        evaluator.stack.pop()
        evaluator.stack.append(b)
        evaluator.stack.append(a)
        evaluator.stack.append(b)
    return b, a, b


# ---------------------------------------------------------------------------
# List operations
# ---------------------------------------------------------------------------

def _cons(a, b, evaluator=None):
    """Prepend a to list b (nil terminates like empty list)."""
    if b is nil:
        return [a]
    if isinstance(b, list):
        return [a] + b
    return [a, b]


def _car(lst, evaluator=None):
    """First element of list."""
    if isinstance(lst, list) and lst:
        return lst[0]
    if isinstance(lst, list):
        return nil
    raise TypeError(f"car: expected list, got {type(lst)}")


def _cdr(lst, evaluator=None):
    """Rest of list (all but first element)."""
    if isinstance(lst, list) and len(lst) > 1:
        return lst[1:]
    if isinstance(lst, list):
        return nil
    raise TypeError(f"cdr: expected list, got {type(lst)}")


def _list(*args, evaluator=None):
    """Create a list. Variadic — requires parens."""
    return list(args)


def _is_nil(val, evaluator=None):
    return val is nil


def _is_list(val, evaluator=None):
    return isinstance(val, list)


# ---------------------------------------------------------------------------
# I/O
# ---------------------------------------------------------------------------

def _print(val, evaluator=None):
    """Print a value followed by newline."""
    print(hya_repr(val))
    return val


def _pr(val, evaluator=None):
    """Print a value (like Forth .)."""
    print(hya_repr(val), end="")
    return val


def _read(evaluator=None):
    """Read a line from stdin."""
    try:
        import sys
        line = sys.stdin.readline()
        return line.rstrip('\n')
    except EOFError:
        return nil


# ---------------------------------------------------------------------------
# Type checking
# ---------------------------------------------------------------------------

def _is_number(val, evaluator=None):
    return isinstance(val, (int, float))


def _is_string(val, evaluator=None):
    return isinstance(val, str)


def _is_symbol(val, evaluator=None):
    return isinstance(val, Symbol)


def _is_fn(val, evaluator=None):
    return isinstance(val, (Builtin, Function))


# ---------------------------------------------------------------------------
# Builtin registry
# ---------------------------------------------------------------------------

def get_builtins() -> dict[str, Builtin]:
    """Return a dict of all built-in functions keyed by symbol name."""
    b = {}

    def reg(name: str, fn: callable, arity: int, doc: str = ""):
        b[name] = Builtin(name, fn, arity, doc)

    # Arithmetic
    reg("+", _add, 2)
    reg("-", _sub, 2)
    reg("*", _mul, 2)
    reg("/", _div, 2)
    reg("//", _floordiv, 2)
    reg("%", _mod, 2)
    reg("neg", _neg, 1)

    # Comparison
    reg("=", _eq, 2)
    reg("<", _lt, 2)
    reg(">", _gt, 2)
    reg("<=", _le, 2)
    reg(">=", _ge, 2)
    reg("!=", _ne, 2)

    # Logic
    reg("and", _and, -1)    # variadic
    reg("or", _or, -1)      # variadic
    reg("not", _not, 1)

    # Stack operations (Forth-like)
    reg("dup", _dup, 1)
    reg("swap", _swap, 2)
    reg("drop", _drop, 1)
    reg("over", _over, 2)
    reg("rot", _rot, 3)
    reg("nip", _nip, 2)
    reg("tuck", _tuck, 2)

    # List operations
    reg("cons", _cons, 2)
    reg("car", _car, 1)
    reg("cdr", _cdr, 1)
    reg("list", _list, -1)  # variadic
    reg("nil?", _is_nil, 1)
    reg("list?", _is_list, 1)

    # I/O
    reg("print", _print, 1)
    reg(".", _pr, 1)
    reg("read", _read, 0)

    # Type checking
    reg("number?", _is_number, 1)
    reg("string?", _is_string, 1)
    reg("symbol?", _is_symbol, 1)
    reg("fn?", _is_fn, 1)

    return b
