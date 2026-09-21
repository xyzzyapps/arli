"""Python VSA backend tests — contract details the shared corpus cannot express.

The .arli corpus proves the user-visible VSA semantics; these tests cover the
pinned error messages, range validation, cleanup-threshold behavior, the raw
float representation and the numeric guarantees that need floats.
"""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.arli.eval import Evaluator
from src.arli.types import Symbol, nil, VSAVec, VSAPair, arli_repr
from src.arli.vsa import (CLEANUP_THRESHOLD, DEFAULT_DIM, VSAError, bind,
                          get_vsa_builtins, similarity, unbind)


def expect_error(ev, source, message):
    """Run `source`, requiring a VSAError carrying exactly `message`."""
    try:
        ev.exec(source)
    except VSAError as err:
        assert str(err) == message, f"{source}: got {str(err)!r}"
        return
    raise AssertionError(f"{source}: expected VSAError {message!r}")


def test_encode_rejects_unencodable_values():
    ev = Evaluator()
    expect_error(ev, "vsa-encode {:a 1}", "vsa: cannot encode map")
    expect_error(ev, "vsa-encode list", "vsa: cannot encode builtin")
    expect_error(ev, "vsa-encode fn (x) x", "vsa: cannot encode fn")
    expect_error(ev, "vsa-encode import os", "vsa: cannot encode unknown")


def test_pinned_error_messages():
    ev = Evaluator()
    expect_error(ev, "(vsa-bundle)", "vsa-bundle: needs at least 1 argument")
    expect_error(ev, "(vsa-majority)", "vsa-majority: needs at least 1 argument")
    expect_error(ev, "vsa-car 'alpha",
                 "vsa-car: expected vsa-pair, vsa-vec, or list")
    expect_error(ev, "vsa-cdr 'alpha",
                 "vsa-cdr: expected vsa-pair, vsa-vec, or list")
    expect_error(ev, "vsa->list 'alpha", "vsa->list: expected vsa-pair or list")
    expect_error(ev, "vsa-seed \"x\"", "vsa-seed: expected integer")
    expect_error(ev, "vsa-permute 'alpha 1.5",
                 "vsa-permute: expected integer shift")
    expect_error(ev, "(vsa-factorize 'alpha)",
                 "vsa-factorize: needs at least 2 arguments")
    expect_error(ev, "(vsa-factorize 'alpha 'nope)",
                 "vsa-factorize: expected codebook list")
    expect_error(ev, "(vsa-factorize 'alpha (list))",
                 "vsa-factorize: empty codebook")


def test_vsa_reset_validates_dimension():
    ev = Evaluator()
    for bad in ("0", "3", "65537", "3.5", "true", "nil", '"64"'):
        expect_error(ev, f"vsa-reset {bad}",
                     "vsa-reset: dimension out of range")
    # A rejected dimension leaves the engine untouched.
    assert ev.exec("vsa-dim") == DEFAULT_DIM
    # Integral floats are fine.
    assert ev.exec("vsa-reset 64.0") == 64
    assert ev.exec("vsa-dim") == 64


def test_reset_resizes_vectors_and_clears_memory():
    ev = Evaluator()
    ev.exec("vsa-register 'alpha")
    assert ev.exec("vsa-reset 128") == 128
    assert ev.exec("vsa-dim") == 128
    # Vectors are rebuilt at the new dimension...
    assert len(ev.exec("vsa->floats 'alpha")) == 256
    assert ev.exec("vsa-similarity (floats->vsa (vsa->floats 'alpha)) 'alpha") > 0.999
    # ...and the cleanup memory was cleared.
    assert ev.exec("vsa-cleanup vsa-encode 'alpha") is nil


def test_cleanup_uses_threshold_and_per_evaluator_memory():
    ev = Evaluator()
    ev.exec("vsa-register 'alpha")
    assert arli_repr(ev.exec("vsa-cleanup vsa-encode 'alpha")) == "alpha"
    # A vector whose best score is under the threshold cleans up to nil.
    engine = ev.vsa_engine
    score = similarity(engine.encode(Symbol("alpha")),
                       engine.encode(Symbol("never-registered")))
    assert abs(score) < CLEANUP_THRESHOLD
    assert ev.exec("vsa-cleanup 'never-registered") is nil
    assert ev.exec("vsa-cleanup vsa-random") is nil
    # Memory belongs to one evaluator: a fresh one starts empty.
    assert Evaluator().exec("vsa-cleanup vsa-encode 'alpha") is nil


def test_vsa_to_list_rejects_improper_chain():
    ev = Evaluator()
    improper = ev.exec("vsa-cons 1 vsa-cons 2 'tail")
    assert arli_repr(improper) == "(1 2 . tail)"
    expect_error(ev, "vsa->list vsa-cons 1 vsa-cons 2 'tail",
                 "vsa->list: improper list")
    assert ev.exec("vsa->list (vsa-list 1 2 3)") == [1, 2, 3]
    assert ev.exec("vsa->list nil") == []
    assert ev.exec("(vsa-list)") is nil
    assert arli_repr(ev.exec("(vsa-list)")) == "nil"


def test_registering_the_same_value_twice_keeps_one_entry():
    ev = Evaluator()
    symbol = Symbol("alpha")
    ev.vsa_engine.remember(symbol)
    ev.vsa_engine.remember(symbol)
    assert len(ev.exec("vsa-query vsa-encode 'alpha 0")) == 1


def test_vsa_type_reports_unknown_for_host_objects():
    ev = Evaluator()
    assert ev.exec("vsa-type import os") == Symbol("unknown")


def test_floats_to_vsa_validates_length():
    ev = Evaluator()
    floats = ev.exec("vsa->floats 'alpha")
    assert len(floats) == 2 * DEFAULT_DIM
    expect_error(ev, "floats->vsa (list 1 2 3)",
                 "floats->vsa: expected 2*D floats")
    # One float too many is rejected as well.
    build = get_vsa_builtins()["floats->vsa"]
    try:
        build(floats + [0.0], evaluator=ev)
    except VSAError as err:
        assert str(err) == "floats->vsa: expected 2*D floats"
    else:
        raise AssertionError("floats->vsa accepted 2*D + 1 floats")


def test_bind_unbind_round_trip():
    ev = Evaluator()
    engine = ev.vsa_engine
    alpha = engine.encode(Symbol("alpha"))
    beta = engine.encode(Symbol("beta"))
    bound = bind(alpha, beta)
    # Numerically the pair recovers both components (bind is commutative).
    assert similarity(unbind(bound, alpha), beta) > 0.999
    assert similarity(unbind(bound, beta), alpha) > 0.999
    # ...and unbinding with an unrelated key does not.
    assert abs(similarity(unbind(bound, engine.encode(Symbol("gamma"))), beta)) < 0.1
    # The same round trip through the interpreter.
    assert ev.exec("vsa-similarity (vsa-unbind (vsa-bind 42 7) 42) 7") > 0.999


def test_vsa_equality_is_structural_and_exact():
    ev = Evaluator()
    assert ev.exec("= vsa-cons 1 2 vsa-cons 1 2") is True
    assert ev.exec("= vsa-cons 1 2 vsa-cons 1 3") is False
    assert ev.exec("= vsa-encode 'alpha vsa-encode 'alpha") is True
    assert ev.exec("= vsa-encode 'alpha 'alpha") is False
    assert ev.exec("= vsa-cons 1 2 5") is False


def test_vsa_values_are_truthy_but_not_lists():
    ev = Evaluator()
    pair = ev.exec("vsa-cons 1 2")
    vec = ev.exec("vsa-encode 'alpha")
    assert isinstance(pair, VSAPair) and isinstance(vec, VSAVec)
    assert ev.exec("if vsa-cons 1 2 'yes 'no") == Symbol("yes")
    assert ev.exec("if vsa-encode 'alpha 'yes 'no") == Symbol("yes")
    assert ev.exec("list? vsa-cons 1 2") is False
    assert ev.exec("list? vsa-encode 'alpha") is False
    assert ev.exec("nil? vsa-cons 1 2") is False
    assert ev.exec("car cons 1 nil") == 1


def test_query_returns_pairs_nearest_first():
    ev = Evaluator()
    ev.exec("vsa-register 'alpha")
    ev.exec("vsa-register 'beta")
    hits = ev.exec("vsa-query vsa-encode 'alpha 2")
    assert [arli_repr(hit.cdr) for hit in hits] == ["alpha", "beta"]
    assert hits[0].car > hits[1].car
    assert ev.exec("vsa-type vsa-car vsa-query vsa-encode 'alpha 0") == Symbol("pair")


if __name__ == "__main__":
    tests = [value for name, value in sorted(globals().items())
             if name.startswith("test_") and callable(value)]
    failed = 0
    for test in tests:
        try:
            test()
            print(f"  OK {test.__name__}")
        except Exception as exc:
            failed += 1
            print(f"  FAIL {test.__name__}: {exc}")
            import traceback
            traceback.print_exc()
    print(f"\n{len(tests) - failed}/{len(tests)} passed")
