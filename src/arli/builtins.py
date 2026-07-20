"""Built-in functions for arli.

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
from .env import Environment


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
    """Swap top two items on stack — returns b then a."""
    if evaluator is not None:
        evaluator.stack.pop()  # remove b
        evaluator.stack.pop()  # remove a
        evaluator.stack.append(b)
        evaluator.stack.append(a)
    return b  # return the new top


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
    return c  # new top of stack


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
    return b  # return the new top of stack


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

    # Stack shorthand: pick and roll
    def _pick(n, evaluator=None):
        """Copy nth element (0=top) to the top. pick 0 = dup, pick 1 = over."""
        if evaluator is not None and isinstance(n, (int, float)):
            idx = int(n)
            stack_len = len(evaluator.stack)
            if 0 <= idx < stack_len:
                source_idx = stack_len - 1 - idx
                val = evaluator.stack[source_idx]
                evaluator.stack.append(val)
                return val
        return n
    reg("pick", _pick, 1)

    def _roll(n, evaluator=None):
        """Rotate nth element (0=top) to the top. roll 1 = swap, roll 2 = rot."""
        if evaluator is not None and isinstance(n, (int, float)):
            depth = int(n)
            stack_len = len(evaluator.stack)
            if 0 <= depth < stack_len:
                roll_idx = stack_len - 1 - depth
                val = evaluator.stack.pop(roll_idx)
                evaluator.stack.append(val)
                return val
        return n
    reg("roll", _roll, 1)

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

    # Sequence operations (fixed arity: individual args)
    def _map(fn, lst, evaluator=None):
        if not isinstance(lst, list):
            return lst
        if evaluator is None:
            return lst
        result = []
        for x in lst:
            result.append(evaluator._eval_expr([Symbol("_"), fn, x]) if isinstance(fn, Symbol) else evaluator.apply(fn, [x]))
        return result
    def _filter(fn, lst, evaluator=None):
        if not isinstance(lst, list):
            return lst
        if evaluator is None:
            return lst
        result = []
        for x in lst:
            val = evaluator.apply(fn, [x])
            if is_truthy(val):
                result.append(x)
        return result
    def _reduce(fn, init, lst, evaluator=None):
        if not isinstance(lst, list) or not lst:
            return init
        if evaluator is None:
            return init
        acc = init
        for x in lst:
            acc = evaluator.apply(fn, [acc, x])
        return acc
    reg("map", _map, 2)
    reg("filter", _filter, 2)
    reg("reduce", _reduce, 3)

    # Result type constructors and operations
    reg("Ok", lambda val, ev=None, **kw: [Symbol("Ok"), val], 1)
    reg("Err", lambda val, ev=None, **kw: [Symbol("Err"), val], 1)
    # Result type operations
    def _map_ok(result, fn, evaluator=None):
        if evaluator is None:
            return result
        if isinstance(result, list) and len(result) == 2 and result[0] == Symbol("Ok"):
            return [Symbol("Ok"), evaluator.apply(fn, [result[1]])]
        return result
    def _and_then(result, fn, evaluator=None):
        if evaluator is None:
            return result
        if isinstance(result, list) and len(result) == 2 and result[0] == Symbol("Ok"):
            return evaluator.apply(fn, [result[1]])
        return result
    def _or_else(result, fn, evaluator=None):
        if evaluator is None:
            return result
        if isinstance(result, list) and len(result) == 2 and result[0] == Symbol("Ok"):
            return result
        return evaluator.apply(fn, [])
    reg("map-ok", _map_ok, 2)
    reg("and-then", _and_then, 2)
    reg("or-else", _or_else, 2)

    # Hash-map creation
    def make_hash_map(*pairs, evaluator=None):
        result = {}
        for i in range(0, len(pairs), 2):
            if i + 1 < len(pairs):
                key = pairs[i]
                val = pairs[i + 1]
                if isinstance(key, Symbol):
                    result[key.name] = val
                else:
                    result[str(key)] = val
        return result
    reg("hash-map", make_hash_map, -1)

    # Type checking
    reg("number?", _is_number, 1)
    reg("string?", _is_string, 1)
    reg("symbol?", _is_symbol, 1)
    reg("fn?", _is_fn, 1)

    # Stack reflection (self-modifying code support)
    def _stack(evaluator=None):
        """Push a copy of the current data stack as a list.
        
        Stack (arity 0) captures the evaluator's data stack so you
        can inspect, filter, or transform it with list operations,
        then restore it with stack!.
        
        Example:
            stack          ;; push (42 \"hello\" 3) — a copy of the stack
            filter number? ;; keep only numbers
            stack!         ;; replace the evaluator's stack
        """
        if evaluator is None:
            return []
        return list(evaluator.stack)
    reg("stack", _stack, 0)

    def _stack_set(new_stack, evaluator=None):
        """Replace the evaluator's data stack with a list.
        
        Stack! (arity 1) takes a list and replaces the evaluator's
        entire stack with its contents. Returns None (not nil!)
        so exec skips the extra push.
        
        Pairs with `stack`:
        
            (let ((s stack))
                (stack! (filter (fn (x) number? x) s)))
        
        If the argument is nil, the stack is cleared.
        """
        if evaluator is None:
            return new_stack
        if isinstance(new_stack, list):
            evaluator.stack = list(new_stack)
        elif new_stack is nil:
            evaluator.stack = []
        else:
            evaluator.stack = [new_stack]
        return None  # tells exec to skip the push
    reg("stack!", _stack_set, 1)

    # -----------------------------------------------------------------------
    # Exec stack operations (Push-like self-modifying code support)
    # -----------------------------------------------------------------------
    def _exec_stack(evaluator=None):
        """Push a copy of the exec stack to the data stack. (arity 0)"""
        if evaluator is None:
            return []
        return list(evaluator.exec_stack)
    reg("exec-stack", _exec_stack, 0)

    def _exec_set(new_stack, evaluator=None):
        """Replace the exec stack with a list. (arity 1)"""
        if evaluator is None:
            return new_stack
        if isinstance(new_stack, list):
            evaluator.exec_stack = list(new_stack)
        elif new_stack is nil:
            evaluator.exec_stack = []
        else:
            evaluator.exec_stack = [new_stack]
        return None  # skip the push
    reg("exec!", _exec_set, 1)

    def _exec_push(form, evaluator=None):
        """Push a form onto the exec stack. (arity 1)"""
        if evaluator is not None:
            evaluator.exec_stack.append(form)
        return None  # skip the push
    reg("exec-push", _exec_push, 1)

    def _exec_pop(evaluator=None):
        """Pop the top of the exec stack onto the data stack. (arity 0)"""
        if evaluator is None or not evaluator.exec_stack:
            return nil
        return evaluator.exec_stack.pop()
    reg("exec-pop", _exec_pop, 0)

    def _exec_depth(evaluator=None):
        """Push the depth of the exec stack to the data stack. (arity 0)"""
        if evaluator is None:
            return 0
        return len(evaluator.exec_stack)
    reg("exec-depth", _exec_depth, 0)

    def _exec_step(evaluator=None):
        """Pop and evaluate one form from the exec stack. (arity 0)
        
        Pops the top of the exec stack and evaluates it as a normal
        arli expression, pushing the result to the data stack.
        Returns the result (which exec also pushes).
        """
        if evaluator is None or not evaluator.exec_stack:
            return nil
        form = evaluator.exec_stack.pop()
        return evaluator._eval_expr(form)
    reg("exec-step", _exec_step, 0)

    # exec-all: process exec stack until empty — variadic (parens required)
    def _exec_all(*args, evaluator=None):
        """Process the entire exec stack until empty. (arity -1, parens)
        
        Each item popped from the exec stack:
        - If it's a list: each element is pushed back onto the exec stack
          (in reverse order, so the first element executes first)
        - If it's a Symbol: the function is looked up and called. It pops
          arguments from the DATA stack according to the function's arity.
        - If it's a literal: it's pushed to the data stack.
        
        Returns the last result pushed to the data stack.
        """
        if evaluator is None:
            return nil
        last = nil
        while evaluator.exec_stack:
            item = evaluator.exec_stack.pop()
            if isinstance(item, list):
                # Evaluate lists as normal arli expressions (atomic evaluation).
                # This preserves prefix semantics: (+ 1 2) is evaluated as a unit.
                result = evaluator._eval_expr(item)
                if result is not None:
                    evaluator.stack.append(result)
                    last = result
                else:
                    last = None
            elif isinstance(item, Symbol):
                # Internal sentinel: restore caller's environment
                if item.name == "__restore_env__":
                    if evaluator.env_stack:
                        evaluator.env = evaluator.env_stack.pop()
                    last = None
                    continue
                # Special forms for Push interpreter
                if item.name == "if":
                    # Push-style if: pop boolean from DATA stack,
                    # pop then/else from EXEC stack
                    cond_val = evaluator.stack.pop() if evaluator.stack else nil
                    else_branch = evaluator.exec_stack.pop() if evaluator.exec_stack else nil
                    then_branch = evaluator.exec_stack.pop() if evaluator.exec_stack else nil
                    if is_truthy(cond_val):
                        evaluator.exec_stack.append(then_branch)
                    else:
                        evaluator.exec_stack.append(else_branch)
                    last = None
                    continue
                if item.name == "do":
                    # do is a no-op in Push mode: items after it are already
                    # on the exec stack and will be processed sequentially
                    last = None
                    continue
                if item.name == "quote":
                    # Push the next item on exec stack as data (don't expand)
                    quoted = evaluator.exec_stack.pop() if evaluator.exec_stack else nil
                    evaluator.stack.append(quoted)
                    last = quoted
                    continue
                fn = evaluator.env.get(item.name)
                if isinstance(fn, Builtin):
                    args = []
                    for _ in range(fn.arity if fn.arity >= 0 else 0):
                        if evaluator.stack:
                            args.insert(0, evaluator.stack.pop())
                    result = fn(*args, evaluator=evaluator)
                    if result is not None:
                        evaluator.stack.append(result)
                        last = result
                elif isinstance(fn, Function):
                    # Bind params from data stack
                    fn_args = []
                    for _ in range(len(fn.params)):
                        if evaluator.stack:
                            fn_args.insert(0, evaluator.stack.pop())
                    # Save current env, create call env
                    evaluator.env_stack.append(evaluator.env)
                    call_env = Environment(parent=fn.env)
                    for param, val in zip(fn.params, fn_args):
                        call_env.define(param.name, val)
                    # Push env-restore marker FIRST (at bottom of frame)
                    evaluator.exec_stack.append(
                        Symbol("__restore_env__"))
                    # Push body on TOP (processed first by the while loop)
                    for expr in reversed(fn.body):
                        evaluator.exec_stack.append(expr)
                    evaluator.env = call_env
                    last = None
                elif callable(fn):
                    # Python callable — pop args? Use arity from signature
                    try:
                        import inspect
                        sig = inspect.signature(fn)
                        required = sum(1 for p in sig.parameters.values()
                                       if p.default is inspect.Parameter.empty
                                       and p.kind in (inspect.Parameter.POSITIONAL_ONLY,
                                                      inspect.Parameter.POSITIONAL_OR_KEYWORD))
                        args = []
                        for _ in range(required):
                            if evaluator.stack:
                                args.insert(0, evaluator.stack.pop())
                        result = fn(*args)
                        if result is not None:
                            evaluator.stack.append(result)
                            last = result
                    except (ValueError, TypeError):
                        result = fn()
                        if result is not None:
                            evaluator.stack.append(result)
                            last = result
                else:
                    evaluator.stack.append(fn)
                    last = fn
            else:
                # Literal: push to data stack
                evaluator.stack.append(item)
                last = item
        # Return None so exec() doesn't double-push the last result
        # (results are already pushed to the data stack by the loop)
        return None
    reg("exec", _exec_all, -1)

    # Evaluation control
    def _eval_form(form, evaluator=None):
        """Evaluate a form in the current environment.
        Used by f-expressions to selectively evaluate arguments."""
        if evaluator is None:
            return form
        return evaluator.eval_form(form)
    reg("eval", _eval_form, 1)

    return b