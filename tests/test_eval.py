"""Python-specific evaluator tests — features not testable via common .arli tests.

These test Python backend internals and interop that the Go backend can't run.
"""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.arli.eval import Evaluator
from src.arli.types import Symbol, nil, Function, is_truthy, arli_repr


def test_truthy_falsy():
    """Test Python-level truthiness function (internal)."""
    assert is_truthy(True) is True
    assert is_truthy(42) is True
    assert is_truthy("hello") is True
    assert is_truthy(nil) is False
    assert is_truthy(False) is False
    print("  OK test_truthy_falsy")


def test_intern_stack_dup():
    """Verify dup pushes to the stack correctly."""
    ev = Evaluator()
    ev.exec("dup 5")
    assert len(ev.stack) >= 2
    assert ev.stack[-1] == 5
    assert ev.stack[-2] == 5
    print("  OK test_intern_stack_dup")


def test_python_import():
    """Python-specific: import a module."""
    ev = Evaluator()
    ev.exec("import os")
    m = ev.env.get("os")
    assert m is not None
    assert hasattr(m, "getcwd")
    print("  OK test_python_import")


def test_python_dot():
    """Python-specific: dot attribute access."""
    ev = Evaluator()
    ev.exec("import os")
    r = ev.exec(". os sep")
    assert isinstance(r, str)
    print("  OK test_python_dot")


def test_python_dot_chain():
    """Python-specific: chained dot with method call."""
    ev = Evaluator()
    ev.exec('import os')
    r = ev.exec('(. os path join "a" "b")')
    assert isinstance(r, str)
    assert "a" in r and "b" in r
    print("  OK test_python_dot_chain")


def test_python_eval():
    """Python-specific: python special form."""
    ev = Evaluator()
    r = ev.exec('python "repr(42)"')
    assert r == "42"
    print("  OK test_python_eval")


def test_python_eval_env():
    """Python-specific: python eval accesses env bindings."""
    ev = Evaluator()
    ev.exec("define x 42")
    r = ev.exec('python "x * 2"')
    assert r == 84
    print("  OK test_python_eval_env")


def test_arli_repr_nil():
    """Test representation of nil."""
    assert arli_repr(nil) == "nil"
    print("  OK test_arli_repr_nil")


def test_arli_repr_list():
    """Test representation of lists."""
    assert arli_repr([1, 2, 3]) == "(1 2 3)"
    print("  OK test_arli_repr_list")


def test_arli_repr_string():
    """Test representation of strings."""
    assert arli_repr("hello") == '"hello"'
    print("  OK test_arli_repr_string")


def test_arli_repr_symbol():
    """Test representation of symbols."""
    assert arli_repr(Symbol("foo")) == "foo"
    print("  OK test_arli_repr_symbol")


def test_env_scope():
    """Test let creates new scope, doesn't leak."""
    ev = Evaluator()
    ev.exec("define x 1")
    ev.exec("let ((x 2)) x")
    # After let, x should still be 1 in outer scope
    assert ev.env.get("x") == 1
    print("  OK test_env_scope")


def test_set_mutation():
    """Test set! mutates in correct scope."""
    ev = Evaluator()
    ev.exec("define x 1")
    ev.exec("let ((x 2)) set! x 99")
    # set! inside let should mutate the let's x, not outer x
    assert ev.env.get("x") == 1
    print("  OK test_set_mutation")


if __name__ == "__main__":
    tests = [
        test_truthy_falsy,
        test_intern_stack_dup,
        test_python_import,
        test_python_dot,
        test_python_dot_chain,
        test_python_eval,
        test_python_eval_env,
        test_arli_repr_nil,
        test_arli_repr_list,
        test_arli_repr_string,
        test_arli_repr_symbol,
        test_env_scope,
        test_set_mutation,
    ]
    failed = 0
    for t in tests:
        try:
            t()
        except Exception as e:
            print(f"  FAIL {t.__name__}: {e}")
            import traceback
            traceback.print_exc()
            failed += 1
    if failed:
        print(f"\n{len(tests) - failed}/{len(tests)} passed, {failed} failed")
    else:
        print(f"\nAll {len(tests)} tests passed!")
