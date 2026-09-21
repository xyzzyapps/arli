"""Self-contained FHRR Vector Symbolic Architecture engine for arli.

Everything here is implemented from scratch — a bit-identical xoshiro128**
generator, float32 phasor vectors, fractional power encoding, VSA pairs with
lazily cached vectors, and a per-evaluator cleanup memory. No third-party
dependencies and no evaluator globals: each `Evaluator` lazily owns one
`VSAEngine` (`evaluator.vsa_engine`), so memories never leak between programs.

Vectors are `array('f')` (re, im) pairs of unit-magnitude phasors. All
arithmetic runs in float64 and is rounded to float32 on store, and every sum
uses a plain left-to-right loop — no compensated summation — so the Python,
JavaScript and Go backends agree numerically.
"""

from __future__ import annotations

import math
from array import array
from typing import Any

from .types import Symbol, nil, Builtin, Function, VSAVec, VSAPair

# --- Constants --------------------------------------------------------------

DEFAULT_DIM = 4096
MIN_DIM = 4
MAX_DIM = 65536
CLEANUP_THRESHOLD = 0.3
FPE_PERIOD = 1000000.0
ROLE_SEED_CAR = 0x00C0FFEE
ROLE_SEED_CDR = 0x00CDCDCD
SEED_NIL = 0x00000A11
SEED_TRUE = 0x00000072
SEED_FALSE = 0x000000F0
SEED_FPE_BASIS = 0x00F9E001
DEFAULT_STREAM_SEED = 1

_MASK32 = 0xFFFFFFFF
_TWO_PI = 2.0 * math.pi


class VSAError(Exception):
    """A misused VSA primitive: bad operand, dimension, or argument count."""


# --- PRNG -------------------------------------------------------------------

def _splitmix32(x: int) -> tuple[int, int]:
    """One splitmix32 step: returns (new state, output), both uint32."""
    x = (x + 0x9E3779B9) & _MASK32
    z = x
    z = ((z ^ (z >> 16)) * 0x21F0AAAD) & _MASK32
    z = ((z ^ (z >> 15)) * 0x735A2D97) & _MASK32
    return x, (z ^ (z >> 15)) & _MASK32


def _rotl32(v: int, k: int) -> int:
    return ((v << k) | (v >> (32 - k))) & _MASK32


class Xoshiro128:
    """xoshiro128** over four uint32 lanes, seeded through splitmix32."""

    __slots__ = ("s0", "s1", "s2", "s3")

    def __init__(self, seed: int) -> None:
        x = seed & _MASK32
        state = [0, 0, 0, 0]
        for i in range(4):
            x, state[i] = _splitmix32(x)
        if (state[0] | state[1] | state[2] | state[3]) == 0:
            state[0] = 1
        self.s0, self.s1, self.s2, self.s3 = state

    def next_u32(self) -> int:
        s0, s1, s2, s3 = self.s0, self.s1, self.s2, self.s3
        result = (_rotl32((s1 * 5) & _MASK32, 7) * 9) & _MASK32
        t = (s1 << 9) & _MASK32
        s2 ^= s0
        s3 ^= s1
        s1 ^= s2
        s0 ^= s3
        s2 ^= t
        s3 = _rotl32(s3, 11)
        self.s0, self.s1, self.s2, self.s3 = s0, s1, s2, s3
        return result

    def next_float(self) -> float:
        """A float64 in [0, 1)."""
        return self.next_u32() / 4294967296.0


def fnv1a32_utf8(s: str) -> int:
    """32-bit FNV-1a over the UTF-8 bytes of `s`."""
    h = 0x811C9DC5
    for byte in s.encode("utf-8"):
        h = ((h ^ byte) * 0x01000193) & _MASK32
    return h


# --- Vector construction and math -------------------------------------------

def _zeros(n: int) -> tuple[array, array]:
    return array("f", [0.0]) * n, array("f", [0.0]) * n


def random_vec_from_seed(seed: int, dim: int) -> VSAVec:
    """Deterministic unit-phasor vector of length `dim` from a uint32 seed."""
    rng = Xoshiro128(seed)
    re, im = _zeros(dim)
    for k in range(dim):
        phase = _TWO_PI * rng.next_float()
        re[k] = math.cos(phase)
        im[k] = math.sin(phase)
    return VSAVec(re, im)


def bind(a: VSAVec, b: VSAVec) -> VSAVec:
    """Binding: the element-wise complex product of two vectors."""
    ar, ai, br, bi = a.re, a.im, b.re, b.im
    n = len(ar)
    re, im = _zeros(n)
    for k in range(n):
        x, y = ar[k], ai[k]
        u, v = br[k], bi[k]
        re[k] = x * u - y * v
        im[k] = x * v + y * u
    return VSAVec(re, im)


def unbind(a: VSAVec, b: VSAVec) -> VSAVec:
    """Unbinding: `a` multiplied by the conjugate of `b` — inverse of bind."""
    ar, ai, br, bi = a.re, a.im, b.re, b.im
    n = len(ar)
    re, im = _zeros(n)
    for k in range(n):
        x, y = ar[k], ai[k]
        u, v = br[k], bi[k]
        re[k] = x * u + y * v
        im[k] = y * u - x * v
    return VSAVec(re, im)


def bundle(vs: list[VSAVec], dim: int) -> VSAVec:
    """Superposition: the component-wise sum of `vs`, renormalized per component.

    Every component of the sum is divided by its own magnitude, so a bundle is
    again a unit-phasor vector — the FHRR representation invariant that
    `random` emits and `bind` preserves. An `n`-way bundle therefore keeps
    similarity ~2/pi with each of its parts. A component whose magnitude
    underflows is zeroed. An empty list yields the zero vector at `dim`.
    """
    if not vs:
        re, im = _zeros(dim)
        return VSAVec(re, im)
    n = len(vs[0].re)
    sre = array("d", [0.0]) * n
    sim = array("d", [0.0]) * n
    for v in vs:
        vr, vi = v.re, v.im
        for k in range(n):
            sre[k] += vr[k]
            sim[k] += vi[k]
    re, im = _zeros(n)
    for k in range(n):
        magnitude = math.sqrt(sre[k] * sre[k] + sim[k] * sim[k])
        if magnitude >= 1e-12:
            re[k] = sre[k] / magnitude
            im[k] = sim[k] / magnitude
    return VSAVec(re, im)


def similarity(a: VSAVec, b: VSAVec) -> float:
    """Mean real part of the inner product, clamped to [-1, 1]."""
    ar, ai, br, bi = a.re, a.im, b.re, b.im
    n = len(ar)
    total = 0.0
    for k in range(n):
        total += ar[k] * br[k] + ai[k] * bi[k]
    value = total / n
    if value < -1.0:
        return -1.0
    if value > 1.0:
        return 1.0
    return value


def permute(v: VSAVec, n: int) -> VSAVec:
    """Cyclic shift: component k moves to (k + n) mod D; `n` may be negative."""
    d = len(v.re)
    re, im = _zeros(d)
    for k in range(d):
        j = (k + n) % d
        re[j] = v.re[k]
        im[j] = v.im[k]
    return VSAVec(re, im)


def _fpe_freqs(dim: int) -> list[int]:
    """FPE frequencies in [1, dim // 2], drawn from the basis stream."""
    rng = Xoshiro128(SEED_FPE_BASIS)
    half = dim // 2
    return [1 + int(math.floor(rng.next_float() * half)) for _ in range(dim)]


# --- Engine -----------------------------------------------------------------

class VSAEngine:
    """Per-evaluator VSA state: dimension, roles, stream, cleanup memory."""

    def __init__(self, dim: int = DEFAULT_DIM) -> None:
        self.stream_seed = DEFAULT_STREAM_SEED
        self._rebuild(dim)

    def _rebuild(self, dim: int) -> None:
        """Reinitialize everything at `dim`, clearing memory and caches."""
        self.dim = dim
        self.rng = Xoshiro128(self.stream_seed)
        self.car_role = random_vec_from_seed(ROLE_SEED_CAR, dim)
        self.cdr_role = random_vec_from_seed(ROLE_SEED_CDR, dim)
        self.nil_vec = random_vec_from_seed(SEED_NIL, dim)
        self.true_vec = random_vec_from_seed(SEED_TRUE, dim)
        self.false_vec = random_vec_from_seed(SEED_FALSE, dim)
        self.fpe_freq = _fpe_freqs(dim)
        self.memory: list[tuple[VSAVec, Any]] = []
        self._symbol_cache: dict[str, VSAVec] = {}
        self._string_cache: dict[str, VSAVec] = {}

    # -- configuration -------------------------------------------------------

    def reset(self, dim: int) -> int:
        """Reinitialize at a new dimension: memory and caches are cleared and
        the random stream restarts from the currently configured seed."""
        if dim < MIN_DIM or dim > MAX_DIM:
            raise VSAError("vsa-reset: dimension out of range")
        self._rebuild(dim)
        return dim

    def reseed(self, seed: int) -> None:
        """Restart the random stream with a uint32 seed."""
        self.stream_seed = seed & _MASK32
        self.rng = Xoshiro128(self.stream_seed)

    # -- vectors -------------------------------------------------------------

    def random_vec(self) -> VSAVec:
        """A vector drawn from the (seeded) stream."""
        dim = self.dim
        rng = self.rng
        re, im = _zeros(dim)
        for k in range(dim):
            phase = _TWO_PI * rng.next_float()
            re[k] = math.cos(phase)
            im[k] = math.sin(phase)
        return VSAVec(re, im)

    def _encode_number(self, x: float) -> VSAVec:
        """Fractional power encoding: close numbers stay similar."""
        dim = self.dim
        freqs = self.fpe_freq
        re, im = _zeros(dim)
        for k in range(dim):
            y = (x * freqs[k]) / FPE_PERIOD
            phase = _TWO_PI * (y - math.floor(y))
            re[k] = math.cos(phase)
            im[k] = math.sin(phase)
        return VSAVec(re, im)

    def encode(self, value: Any) -> VSAVec:
        """Encode any encodable arli value to a vector."""
        if isinstance(value, VSAVec):
            return value
        if isinstance(value, VSAPair):
            vec = value.vec
            if vec is None:
                vec = bundle([bind(self.car_role, self.encode(value.car)),
                              bind(self.cdr_role, self.encode(value.cdr))],
                             self.dim)
                value.vec = vec
            return vec
        if value is nil:
            return self.nil_vec
        if isinstance(value, bool):
            return self.true_vec if value else self.false_vec
        if isinstance(value, (int, float)):
            return self._encode_number(float(value))
        if isinstance(value, str):
            vec = self._string_cache.get(value)
            if vec is None:
                vec = random_vec_from_seed(fnv1a32_utf8("s:" + value), self.dim)
                self._string_cache[value] = vec
            return vec
        if isinstance(value, Symbol):
            name = value.name
            vec = self._symbol_cache.get(name)
            if vec is None:
                vec = random_vec_from_seed(fnv1a32_utf8("y:" + name), self.dim)
                self._symbol_cache[name] = vec
            return vec
        if isinstance(value, list):
            chain = nil
            for item in reversed(value):
                chain = VSAPair(item, chain)
            return self.encode(chain)
        if isinstance(value, dict):
            raise VSAError("vsa: cannot encode map")
        if isinstance(value, Builtin):
            raise VSAError("vsa: cannot encode builtin")
        if isinstance(value, Function):
            raise VSAError("vsa: cannot encode fn")
        raise VSAError("vsa: cannot encode unknown")

    # -- cleanup memory ------------------------------------------------------

    def remember(self, value: Any) -> Any:
        """Store `value` and its vector; re-registering replaces in place."""
        vec = self.encode(value)
        for i, (_, existing) in enumerate(self.memory):
            if existing is value:
                self.memory[i] = (vec, value)
                return value
        self.memory.append((vec, value))
        return value

    def lookup(self, vec: VSAVec) -> Any:
        """Nearest registered value above the cleanup threshold, else nil."""
        best_value = nil
        best_score = None
        for entry_vec, value in self.memory:
            score = similarity(vec, entry_vec)
            if best_score is None or score > best_score:
                best_score = score
                best_value = value
        if best_score is None or best_score < CLEANUP_THRESHOLD:
            return nil
        return best_value

    def nearest(self, vec: VSAVec, candidates: list) -> Any:
        """Highest-similarity element of `candidates`, with no threshold."""
        best = nil
        best_score = None
        for candidate in candidates:
            score = similarity(vec, self.encode(candidate))
            if best_score is None or score > best_score:
                best_score = score
                best = candidate
        return best

    # -- pattern matching ----------------------------------------------------

    def _walk(self, node: Any, steps: list[str]) -> Any:
        """Resolve a `car`/`cdr` path without cleaning up intermediate nodes.

        Only the final leaf, if it is a vector, goes through cleanup memory:
        cleaning up mid-path would replace an intermediate pair vector with
        whichever atom happens to be nearest and lose the rest of the path.
        """
        for step in steps:
            if isinstance(node, VSAPair):
                node = node.car if step == "car" else node.cdr
            elif isinstance(node, list):
                if step == "car":
                    node = node[0] if node else nil
                else:
                    node = node[1:] if len(node) > 1 else nil
            elif isinstance(node, VSAVec):
                role = self.car_role if step == "car" else self.cdr_role
                node = unbind(node, role)
            elif node is nil:
                node = nil
            else:
                node = nil  # an atom: cannot descend any further
        if isinstance(node, VSAVec):
            node = self.lookup(node)
        return node

    def match_pattern(self, pattern: Any, value: Any) -> list:
        """Bind `?vars` of `pattern` against `value`; returns [var, value]s."""
        bindings: list = []
        self._match(pattern, value, bindings)
        return bindings

    def _match(self, pattern: Any, value: Any, bindings: list) -> None:
        if isinstance(pattern, Symbol):
            name = pattern.name
            if name == "_":
                return
            if name.startswith("?"):
                bindings.append([pattern, value])
            return  # any other atom is ignored — no equality test
        if isinstance(pattern, list):
            for i, sub in enumerate(pattern):
                node = self._walk(value, ["cdr"] * i + ["car"])
                self._match(sub, node, bindings)
        # any other atom is ignored


# --- Builtins ---------------------------------------------------------------

def _engine(evaluator: Any) -> VSAEngine:
    if evaluator is None:
        raise VSAError("vsa: primitives need an evaluator")
    return evaluator.vsa_engine


def _int_arg(value: Any, message: str) -> int:
    """Accept ints and integral floats; anything else raises `message`."""
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise VSAError(message)
    if isinstance(value, float):
        if not value.is_integer():
            raise VSAError(message)
        return int(value)
    return value


def _vsa_dim(evaluator=None):
    return _engine(evaluator).dim


def _vsa_reset(n, evaluator=None):
    return _engine(evaluator).reset(
        _int_arg(n, "vsa-reset: dimension out of range"))


def _vsa_seed(n, evaluator=None):
    seed = _int_arg(n, "vsa-seed: expected integer")
    _engine(evaluator).reseed(seed)
    return seed


def _vsa_random(evaluator=None):
    return _engine(evaluator).random_vec()


def _vsa_bind(a, b, evaluator=None):
    engine = _engine(evaluator)
    return bind(engine.encode(a), engine.encode(b))


def _vsa_unbind(a, b, evaluator=None):
    engine = _engine(evaluator)
    return unbind(engine.encode(a), engine.encode(b))


def _vsa_bundle(*args, evaluator=None):
    engine = _engine(evaluator)
    if not args:
        raise VSAError("vsa-bundle: needs at least 1 argument")
    return bundle([engine.encode(a) for a in args], engine.dim)


def _vsa_majority(*args, evaluator=None):
    engine = _engine(evaluator)
    if not args:
        raise VSAError("vsa-majority: needs at least 1 argument")
    return bundle([engine.encode(a) for a in args], engine.dim)


def _vsa_similarity(a, b, evaluator=None):
    engine = _engine(evaluator)
    return similarity(engine.encode(a), engine.encode(b))


def _vsa_permute(value, n, evaluator=None):
    shift = _int_arg(n, "vsa-permute: expected integer shift")
    return permute(_engine(evaluator).encode(value), shift)


def _vsa_encode(value, evaluator=None):
    return _engine(evaluator).encode(value)


def _vsa_cons(a, b, evaluator=None):
    return VSAPair(a, b)


def _vsa_car(value, evaluator=None):
    engine = _engine(evaluator)
    if isinstance(value, VSAPair):
        return value.car
    if isinstance(value, VSAVec):
        return engine.lookup(unbind(value, engine.car_role))
    if isinstance(value, list):
        return value[0] if value else nil
    if value is nil:
        return nil
    raise VSAError("vsa-car: expected vsa-pair, vsa-vec, or list")


def _vsa_cdr(value, evaluator=None):
    engine = _engine(evaluator)
    if isinstance(value, VSAPair):
        return value.cdr
    if isinstance(value, VSAVec):
        return engine.lookup(unbind(value, engine.cdr_role))
    if isinstance(value, list):
        return value[1:] if len(value) > 1 else nil
    if value is nil:
        return nil
    raise VSAError("vsa-cdr: expected vsa-pair, vsa-vec, or list")


def _vsa_list(*args, evaluator=None):
    chain = nil
    for item in reversed(args):
        chain = VSAPair(item, chain)
    return chain


def _vsa_to_list(value, evaluator=None):
    if isinstance(value, VSAPair):
        out = []
        node = value
        while isinstance(node, VSAPair):
            out.append(node.car)
            node = node.cdr
        if node is not nil:
            raise VSAError("vsa->list: improper list")
        return out
    if isinstance(value, list):
        return list(value)
    if value is nil:
        return []
    raise VSAError("vsa->list: expected vsa-pair or list")


def _vsa_pair_q(value, evaluator=None):
    return isinstance(value, VSAPair)


def _vsa_vec_q(value, evaluator=None):
    return isinstance(value, VSAVec)


def _vsa_type(value, evaluator=None):
    if isinstance(value, VSAVec):
        return Symbol("vsa-vec")
    if isinstance(value, VSAPair):
        return Symbol("pair")
    if value is nil:
        return Symbol("nil")
    if isinstance(value, bool):
        return Symbol("bool")
    if isinstance(value, (int, float)):
        return Symbol("number")
    if isinstance(value, str):
        return Symbol("string")
    if isinstance(value, Symbol):
        return Symbol("symbol")
    if isinstance(value, list):
        return Symbol("list")
    if isinstance(value, dict):
        return Symbol("map")
    if isinstance(value, Builtin):
        return Symbol("builtin")
    if isinstance(value, Function):
        return Symbol("fn")
    return Symbol("unknown")


def _vsa_register(value, evaluator=None):
    _engine(evaluator).remember(value)
    return value


def _vsa_cleanup(value, evaluator=None):
    engine = _engine(evaluator)
    return engine.lookup(engine.encode(value))


def _vsa_query(value, k, evaluator=None):
    engine = _engine(evaluator)
    limit = _int_arg(k, "vsa-query: expected integer k")
    vec = engine.encode(value)
    scored = [(similarity(vec, entry_vec), entry)
              for entry_vec, entry in engine.memory]
    scored.sort(key=lambda pair: pair[0], reverse=True)
    if limit > 0:
        scored = scored[:limit]
    return [VSAPair(score, entry) for score, entry in scored]


def _vsa_clear(evaluator=None):
    _engine(evaluator).memory.clear()
    return nil


def _vsa_factorize(*args, evaluator=None):
    engine = _engine(evaluator)
    if len(args) < 2:
        raise VSAError("vsa-factorize: needs at least 2 arguments")
    target = engine.encode(args[0])
    codebooks = []
    for codebook in args[1:]:
        if not isinstance(codebook, list):
            raise VSAError("vsa-factorize: expected codebook list")
        if not codebook:
            raise VSAError("vsa-factorize: empty codebook")
        codebooks.append(codebook)
    # Superposition init, then cleanup-driven refinement.
    est = [bundle([engine.encode(item) for item in codebook], engine.dim)
           for codebook in codebooks]
    for _ in range(32):
        changed = False
        for i, codebook in enumerate(codebooks):
            others = bundle([engine.encode(est[j])
                             for j in range(len(est)) if j != i], engine.dim)
            candidate = unbind(target, others)
            new = engine.nearest(candidate, codebook)
            if new is not est[i]:
                est[i] = new
                changed = True
        if not changed:
            break
    return est


def _vsa_match(pattern, value, evaluator=None):
    return _engine(evaluator).match_pattern(pattern, value)


def _vsa_to_floats(value, evaluator=None):
    vec = _engine(evaluator).encode(value)
    re, im = vec.re, vec.im
    out = []
    for k in range(len(re)):
        out.append(re[k])
        out.append(im[k])
    return out


def _vsa_from_floats(lst, evaluator=None):
    engine = _engine(evaluator)
    dim = engine.dim
    if not isinstance(lst, list) or len(lst) != 2 * dim:
        raise VSAError("floats->vsa: expected 2*D floats")
    re, im = _zeros(dim)
    for k in range(dim):
        re[k] = lst[2 * k]
        im[k] = lst[2 * k + 1]
    return VSAVec(re, im)


def get_vsa_builtins() -> dict[str, Builtin]:
    """All VSA primitives, keyed by symbol name."""
    b: dict[str, Builtin] = {}

    def reg(name: str, fn, arity: int, doc: str = ""):
        b[name] = Builtin(name, fn, arity, doc)

    reg("vsa-dim", _vsa_dim, 0, "Current vector dimension (default 4096).")
    reg("vsa-reset", _vsa_reset, 1,
        "Reinitialize the engine at dimension n, clearing memory and caches.")
    reg("vsa-seed", _vsa_seed, 1, "Reseed the random stream with n.")
    reg("vsa-random", _vsa_random, 0, "New random vector from the stream.")
    reg("vsa-bind", _vsa_bind, 2, "Binding (element-wise complex product).")
    reg("vsa-bundle", _vsa_bundle, -1, "Bundling / superposition (variadic).")
    reg("vsa-majority", _vsa_majority, -1,
        "FHRR centroid; identical to vsa-bundle (variadic).")
    reg("vsa-unbind", _vsa_unbind, 2, "Unbinding (multiply by the conjugate).")
    reg("vsa-similarity", _vsa_similarity, 2, "Cosine similarity in [-1, 1].")
    reg("vsa-permute", _vsa_permute, 2, "Cyclic shift by n (negative shifts back).")
    reg("vsa-encode", _vsa_encode, 1, "Encode any encodable value to a vector.")
    reg("vsa-cons", _vsa_cons, 2, "Create a VSA pair.")
    reg("vsa-car", _vsa_car, 1, "First element of a VSA pair / vector / list.")
    reg("vsa-cdr", _vsa_cdr, 1, "Rest of a VSA pair / vector / list.")
    reg("vsa-list", _vsa_list, -1, "Nil-terminated VSA pair chain (variadic).")
    reg("vsa->list", _vsa_to_list, 1, "Convert a VSA pair chain to a plain list.")
    reg("vsa-pair?", _vsa_pair_q, 1, "Is this a VSA pair?")
    reg("vsa-vec?", _vsa_vec_q, 1, "Is this a VSA vector?")
    reg("vsa-type", _vsa_type, 1, "Type class as a symbol.")
    reg("vsa-register", _vsa_register, 1, "Register a value in cleanup memory.")
    reg("vsa-cleanup", _vsa_cleanup, 1, "Nearest registered value above 0.3, else nil.")
    reg("vsa-query", _vsa_query, 2, "k nearest entries as (similarity . value).")
    reg("vsa-clear", _vsa_clear, 0, "Empty the cleanup memory.")
    reg("vsa-factorize", _vsa_factorize, -1,
        "Resonator factorization: (vsa-factorize bundle cb1 cb2 ...).")
    reg("vsa-match", _vsa_match, 2, "Bind ?vars of a quoted pattern against a value.")
    reg("vsa->floats", _vsa_to_floats, 1, "Interleaved re, im floats of a value.")
    reg("floats->vsa", _vsa_from_floats, 1, "Wrap an interleaved float list.")
    return b
