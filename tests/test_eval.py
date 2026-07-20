"""Tests for the Hya evaluator."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.hya.eval import Evaluator
from src.hya.types import Symbol, nil, Function, hya_repr


def check(name, actual, expected):
    if actual != expected:
        print(f"  FAIL {name}: expected {hya_repr(expected)}, got {hya_repr(actual)}")
        return False
    print(f"  OK {name}")
    return True


def test_literal():
    ev = Evaluator()
    assert ev.exec("42") == 42
    assert ev.exec("3.14") == 3.14
    assert ev.exec('"hello"') == "hello"
    print("  OK test_literal")


def test_arity_arithmetic():
    ev = Evaluator()
    result = ev.exec("+ 1 2")
    assert result == 3
    print("  OK test_arity_arithmetic")


def test_nested_arity():
    ev = Evaluator()
    result = ev.exec("* + 2 3 4")  # (* (+ 2 3) 4) = 20
    assert result == 20
    print("  OK test_nested_arity")


def test_define():
    ev = Evaluator()
    ev.exec("define x 42")
    assert ev.env.get("x") == 42
    print("  OK test_define")


def test_define_with_expr():
    ev = Evaluator()
    ev.exec("define x + 1 2")
    assert ev.env.get("x") == 3
    print("  OK test_define_with_expr")


def test_if_true():
    ev = Evaluator()
    result = ev.exec('(if true "yes" "no")')
    assert result == "yes"
    print("  OK test_if_true")


def test_if_false():
    ev = Evaluator()
    result = ev.exec('(if nil "yes" "no")')
    assert result == "no"
    print("  OK test_if_false")


def test_if_cond_expr():
    """Test if with arity-driven condition expression."""
    ev = Evaluator()
    result = ev.exec("+ (if true 1 2) (if false 10 20)")
    assert result == 21  # 1 + 20
    print("  OK test_if_cond_expr")


def test_quote():
    ev = Evaluator()
    result = ev.exec("quote + 1 2")  # should return unevaluated [+, 1, 2]
    assert isinstance(result, list)
    assert len(result) == 3
    assert isinstance(result[0], Symbol)
    assert result[0].name == "+"
    print("  OK test_quote")


def test_defn_and_call():
    ev = Evaluator()
    ev.exec("(defn add (x y) (+ x y))")
    result = ev.exec("add 1 2")
    assert result == 3
    print("  OK test_defn_and_call")


def test_fn_lambda():
    ev = Evaluator()
    ev.exec("define double (fn (x) (* x 2))")
    result = ev.exec("double 5")
    assert result == 10
    print("  OK test_fn_lambda")


def test_do_sequence():
    ev = Evaluator()
    ev.exec("define x 1")
    result = ev.exec("(do (define x 2) (+ x 10))")
    assert result == 12
    assert ev.env.get("x") == 2
    print("  OK test_do_sequence")


def test_let():
    ev = Evaluator()
    result = ev.exec("(let ((x 5) (y 3)) (+ x y))")
    assert result == 8
    print("  OK test_let")


def test_set():
    ev = Evaluator()
    ev.exec("define x 1")
    ev.exec("(set! x 42)")
    assert ev.env.get("x") == 42
    print("  OK test_set")


def test_cons_car_cdr():
    ev = Evaluator()
    result = ev.exec("cons 1 cons 2 nil")
    # (cons 1 (cons 2 nil)) -> list [1, 2]
    assert isinstance(result, list)
    assert result[0] == 1
    assert result[1] == 2

    ev2 = Evaluator()
    ev2.exec("define lst cons 1 cons 2 nil")
    r1 = ev2.exec("car lst")
    assert r1 == 1
    r2 = ev2.exec("cdr lst")
    assert r2 == [2]
    print("  OK test_cons_car_cdr")


def test_recursive_function():
    """Test recursive function using defn-rec."""
    ev = Evaluator()
    ev.exec("""
        (defn-rec fact (n)
            (if (= n 0)
                1
                (* n fact (- n 1))))
    """)
    result = ev.exec("fact 5")
    assert result == 120
    print("  OK test_recursive_function")


def test_complex_arity_chain():
    """Test chained arity-driven operations."""
    ev = Evaluator()
    result = ev.exec("/ - 10 2 3")  # (/ (- 10 2) 3) = 8/3 ~= 2.666...
    assert abs(result - 8/3) < 0.0001
    print("  OK test_complex_arity_chain")


def test_comparison():
    ev = Evaluator()
    assert ev.exec("= 1 1") is True
    assert ev.exec("< 1 2") is True
    assert ev.exec("> 2 1") is True
    assert ev.exec("> 1 2") is False
    print("  OK test_comparison")


def test_nil_constant():
    ev = Evaluator()
    assert ev.exec("nil") is nil
    print("  OK test_nil_constant")


def test_list_variadic():
    """Variadic list requires parens."""
    ev = Evaluator()
    result = ev.exec("(list 1 2 3)")
    assert result == [1, 2, 3]
    print("  OK test_list_variadic")


def test_while_loop():
    """Test while loop."""
    ev = Evaluator()
    ev.exec("""
        (do
            (define i 0)
            (define sum 0)
            (while (< i 5)
                (do
                    (set! sum + sum i)
                    (set! i + i 1)))
            sum)
    """)
    # Exec returns the last expression which is `sum` after the while loop
    # Actually the do returns sum, but we need to get sum's value
    result = ev.env.get("sum")
    assert result == 10  # 0 + 1 + 2 + 3 + 4 = 10
    print("  OK test_while_loop")


def test_neg():
    ev = Evaluator()
    result = ev.exec("neg 42")
    assert result == -42
    print("  OK test_neg")


def test_multi_expr_file():
    """Multiple expressions in a file."""
    ev = Evaluator()
    ev.exec("""
        define x 10
        define y 20
        define z + x y
    """)
    assert ev.env.get("z") == 30
    print("  OK test_multi_expr_file")


def test_truthy_falsy():
    ev = Evaluator()
    assert is_truthy(ev.exec("true")) is True
    assert is_truthy(ev.exec("42")) is True
    assert is_truthy(ev.exec('"hello"')) is True
    assert is_truthy(nil) is False
    print("  OK test_truthy_falsy")


# Import here to avoid circular import
from src.hya.types import is_truthy


def test_for_loop():
    """Test for loop with arity 3: for var list body."""
    ev = Evaluator()
    ev.exec("define result nil")
    ev.exec('for x (list 1 2 3) do print x set! result x')
    # After loop, result should be 3 (last value)
    assert ev.env.get("result") == 3
    print("  OK test_for_loop")


def test_cond_simple():
    """Test cond with clauses list."""
    ev = Evaluator()
    ev.exec("define x 5")
    result = ev.exec('cond ((> x 10) "big" (< x 3) "small" true "medium")')
    assert result == "medium"
    print("  OK test_cond_simple")


def test_cond_first_match():
    """Test cond returns first matching clause."""
    ev = Evaluator()
    ev.exec("define x 15")
    result = ev.exec('cond ((> x 10) "big" true "fallback")')
    assert result == "big"
    print("  OK test_cond_first_match")


def test_cond_fallthrough():
    """Test cond with no matches returns nil."""
    ev = Evaluator()
    result = ev.exec("cond (false 1 nil 2)")
    # false is falsy, nil is falsy, so nothing matches
    assert result is nil
    print("  OK test_cond_fallthrough")


def test_cond_nested():
    """Test cond with arity-driven expressions inside clauses."""
    ev = Evaluator()
    ev.exec("define x 7")
    result = ev.exec("cond ((< x 5) 0 (= x 7) (+ 10 20) true 99)")
    assert result == 30  # 10 + 20 = 30
    print("  OK test_cond_nested")


def test_for_no_body():
    """Test for with empty body."""
    ev = Evaluator()
    result = ev.exec("for x (list 1 2 3) nil")
    assert result is nil
    print("  OK test_for_no_body")


def test_for_arity_driven():
    """Test for WITHOUT parens — uses arity 3."""
    ev = Evaluator()
    ev.exec("define sum 0")
    ev.exec("for x (list 1 2 3 4 5) set! sum + sum x")
    assert ev.env.get("sum") == 15  # 1+2+3+4+5 = 15
    print("  OK test_for_arity_driven")


def test_if_without_parens():
    """Test if cond then else WITHOUT parens — uses arity 3."""
    ev = Evaluator()
    result = ev.exec('if true "yes" "no"')
    assert result == "yes"
    result = ev.exec('if nil "yes" "no"')
    assert result == "no"
    print("  OK test_if_without_parens")


def test_let_without_parens():
    """Test let bindings body WITHOUT parens — uses arity 2."""
    ev = Evaluator()
    result = ev.exec('let ((x 5) (y 3)) + x y')
    assert result == 8
    print("  OK test_let_without_parens")


def test_while_without_parens():
    """Test while cond body WITHOUT parens — uses arity 2."""
    ev = Evaluator()
    ev.exec("define i 0")
    ev.exec("while (< i 5) do print i set! i + i 1")
    assert ev.env.get("i") == 5
    print("  OK test_while_without_parens")


if __name__ == "__main__":
    tests = [
        test_literal,
        test_arity_arithmetic,
        test_nested_arity,
        test_define,
        test_define_with_expr,
        test_if_true,
        test_if_false,
        test_if_cond_expr,
        test_quote,
        test_defn_and_call,
        test_fn_lambda,
        test_do_sequence,
        test_let,
        test_set,
        test_cons_car_cdr,
        test_recursive_function,
        test_complex_arity_chain,
        test_comparison,
        test_nil_constant,
        test_list_variadic,
        test_while_loop,
        test_neg,
        test_multi_expr_file,
        test_truthy_falsy,
        test_for_loop,
        test_cond_simple,
        test_cond_first_match,
        test_cond_fallthrough,
        test_cond_nested,
        test_for_no_body,
        test_for_arity_driven,
        test_if_without_parens,
        test_let_without_parens,
        test_while_without_parens,
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
