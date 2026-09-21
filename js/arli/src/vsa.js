/**
 * VSA (Vector Symbolic Architecture) subsystem for arli (JavaScript port).
 *
 * Self-contained FHRR engine: unit-phase complex vectors of dimension D with
 * bind, unbind, bundle, permute and fractional power encoding. No external
 * dependencies — the PRNG is xoshiro128** seeded through splitmix32, vectors
 * are Float32Array pairs, and all arithmetic is carried out in float64 and
 * rounded to float32 on store.
 *
 * One engine instance per Evaluator, created lazily on first VSA use and held
 * in a module-level WeakMap (so `vsa-reset` can reinitialize it without ever
 * touching `eval.js`).
 *
 * Everything is additive: VSA values are reachable only through the `vsa-*`
 * primitives returned by `getVsaBuiltins()`.
 */

import { ArliSymbol, nil, Builtin, Function, VSAVec, VSAPair } from './types.js';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const DEFAULT_DIM = 4096;
const MIN_DIM = 4;
const MAX_DIM = 65536;
const CLEANUP_THRESHOLD = 0.3;
const FPE_PERIOD = 1000000.0;

const ROLE_SEED_CAR = 0x00c0ffee;
const ROLE_SEED_CDR = 0x00cdcdcd;
const SEED_NIL = 0x00000a11;
const SEED_TRUE = 0x00000072;
const SEED_FALSE = 0x000000f0;
const SEED_FPE_BASIS = 0x00f9e001;
const DEFAULT_STREAM_SEED = 1;

// ---------------------------------------------------------------------------
// PRNG — xoshiro128** over uint32 lanes, seeded through splitmix32
// ---------------------------------------------------------------------------

function rotl32(v, k) {
  return ((v << k) | (v >>> (32 - k))) >>> 0;
}

/** (new state, output) for a uint32 state. */
function splitmix32(x) {
  x = (x + 0x9e3779b9) >>> 0;
  let z = x;
  z = Math.imul(z ^ (z >>> 16), 0x21f0aaad) >>> 0;
  z = Math.imul(z ^ (z >>> 15), 0x735a2d97) >>> 0;
  return [x, (z ^ (z >>> 15)) >>> 0];
}

class Xoshiro128 {
  constructor(seed) {
    const s = [0, 0, 0, 0];
    let x = seed >>> 0;
    for (let i = 0; i < 4; i++) {
      const [next, out] = splitmix32(x);
      x = next;
      s[i] = out;
    }
    if ((s[0] | s[1] | s[2] | s[3]) === 0) s[0] = 1;
    this.s = s;
  }

  nextU32() {
    const s = this.s;
    const result = Math.imul(rotl32(Math.imul(s[1], 5), 7), 9) >>> 0;
    const t = (s[1] << 9) >>> 0;
    s[2] = (s[2] ^ s[0]) >>> 0;
    s[3] = (s[3] ^ s[1]) >>> 0;
    s[1] = (s[1] ^ s[2]) >>> 0;
    s[0] = (s[0] ^ s[3]) >>> 0;
    s[2] = (s[2] ^ t) >>> 0;
    s[3] = rotl32(s[3], 11);
    return result;
  }

  /** float64 in [0, 1) */
  nextFloat() {
    return this.nextU32() / 4294967296.0;
  }
}

const utf8Encoder = new TextEncoder();

/** 32-bit FNV-1a over the UTF-8 bytes of `s`. */
function fnv1a32Utf8(s) {
  const bytes = utf8Encoder.encode(s);
  let h = 0x811c9dc5;
  for (let i = 0; i < bytes.length; i++) {
    h = Math.imul(h ^ bytes[i], 0x01000193) >>> 0;
  }
  return h;
}

// ---------------------------------------------------------------------------
// Vector primitives — {re, im} Float32Array pairs of the same length
// ---------------------------------------------------------------------------

/** Unit-phase vector drawn from a freshly seeded stream. */
function randomVecFromSeed(seed, dim) {
  const rng = new Xoshiro128(seed);
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  for (let k = 0; k < dim; k++) {
    const phase = 2.0 * Math.PI * rng.nextFloat();
    re[k] = Math.cos(phase);
    im[k] = Math.sin(phase);
  }
  return { re, im };
}

function bindVec(a, b, dim) {
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  for (let k = 0; k < dim; k++) {
    const ar = a.re[k], ai = a.im[k], br = b.re[k], bi = b.im[k];
    re[k] = ar * br - ai * bi;
    im[k] = ar * bi + ai * br;
  }
  return { re, im };
}

/** a * conj(b) */
function unbindVec(a, b, dim) {
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  for (let k = 0; k < dim; k++) {
    const ar = a.re[k], ai = a.im[k], br = b.re[k], bi = b.im[k];
    re[k] = ar * br + ai * bi;
    im[k] = ai * br - ar * bi;
  }
  return { re, im };
}

/** Per-component unit normalization (FHRR centroid; `majority` is the same). */
function bundleVecs(vs, dim) {
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  if (vs.length === 0) return { re, im };
  const sre = new Float64Array(dim);
  const sim = new Float64Array(dim);
  for (let i = 0; i < vs.length; i++) {
    const v = vs[i];
    for (let k = 0; k < dim; k++) {
      sre[k] += v.re[k];
      sim[k] += v.im[k];
    }
  }
  for (let k = 0; k < dim; k++) {
    const m = Math.sqrt(sre[k] * sre[k] + sim[k] * sim[k]);
    if (m < 1e-12) continue; // zero component
    re[k] = sre[k] / m;
    im[k] = sim[k] / m;
  }
  return { re, im };
}

/** Clamped cosine similarity in [-1, 1] (float64). */
function similarityVec(a, b, dim) {
  let dot = 0;
  for (let k = 0; k < dim; k++) dot += a.re[k] * b.re[k] + a.im[k] * b.im[k];
  let s = dot / dim;
  if (s > 1) s = 1;
  else if (s < -1) s = -1;
  return s;
}

/** out[(k + n) mod D] = v[k]; n may be negative. */
function permuteVec(v, n, dim) {
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  let shift = n % dim;
  if (shift < 0) shift += dim;
  for (let k = 0; k < dim; k++) {
    const t = (k + shift) % dim;
    re[t] = v.re[k];
    im[t] = v.im[k];
  }
  return { re, im };
}

/** Fractional power encoding of a number. */
function fpeVec(x, dim, freq) {
  const re = new Float32Array(dim);
  const im = new Float32Array(dim);
  for (let k = 0; k < dim; k++) {
    const y = x * freq[k] / FPE_PERIOD;
    const phase = 2.0 * Math.PI * (y - Math.floor(y));
    re[k] = Math.cos(phase);
    im[k] = Math.sin(phase);
  }
  return { re, im };
}

function wrapVec(v) {
  return v instanceof VSAVec ? v : new VSAVec(v.re, v.im);
}

/** Integer argument or null when it is not a finite number. */
function toInt(x) {
  if (typeof x !== 'number' || !Number.isFinite(x)) return null;
  return Math.trunc(x);
}

// ---------------------------------------------------------------------------
// Engine — roles, encodings, cleanup memory
// ---------------------------------------------------------------------------

class VsaEngine {
  constructor(dim = DEFAULT_DIM) {
    this.dim = dim;
    this.streamSeed = DEFAULT_STREAM_SEED;
    this.rng = new Xoshiro128(this.streamSeed);
    this.cache = new Map();  // 'y:'+name / 's:'+text -> vector
    this.memory = [];        // ordered [{vec, value}]
    this._buildRoles();
  }

  _buildRoles() {
    const dim = this.dim;
    this.CAR_ROLE = randomVecFromSeed(ROLE_SEED_CAR, dim);
    this.CDR_ROLE = randomVecFromSeed(ROLE_SEED_CDR, dim);
    this.NIL_VEC = randomVecFromSeed(SEED_NIL, dim);
    this.TRUE_VEC = randomVecFromSeed(SEED_TRUE, dim);
    this.FALSE_VEC = randomVecFromSeed(SEED_FALSE, dim);
    const basis = new Xoshiro128(SEED_FPE_BASIS);
    const half = Math.floor(dim / 2);
    this.FPE_FREQ = new Int32Array(dim);
    for (let k = 0; k < dim; k++) {
      this.FPE_FREQ[k] = 1 + Math.floor(basis.nextFloat() * half);
    }
  }

  /**
   * Reinitialize at `dim`: clears the cleanup memory and every cache and
   * restarts the random stream from the seed most recently set by `vsa-seed`
   * (or DEFAULT_STREAM_SEED).
   */
  reset(dim) {
    this.dim = dim;
    this.cache = new Map();
    this.memory = [];
    this.rng = new Xoshiro128(this.streamSeed);
    this._buildRoles();
  }

  reseed(seed) {
    this.streamSeed = seed >>> 0;
    this.rng = new Xoshiro128(this.streamSeed);
  }

  /** A vector drawn from the persistent stream. */
  randomVec() {
    const dim = this.dim;
    const re = new Float32Array(dim);
    const im = new Float32Array(dim);
    for (let k = 0; k < dim; k++) {
      const phase = 2.0 * Math.PI * this.rng.nextFloat();
      re[k] = Math.cos(phase);
      im[k] = Math.sin(phase);
    }
    return new VSAVec(re, im);
  }

  /** Memoized deterministic vector for a keyed symbol name / string text. */
  _cached(kind, key) {
    const full = kind + ':' + key;
    let vec = this.cache.get(full);
    if (vec === undefined) {
      vec = randomVecFromSeed(fnv1a32Utf8(full), this.dim);
      this.cache.set(full, vec);
    }
    return vec;
  }

  /**
   * Encode an arli value to a vector. Returns the value's own VSAVec for a
   * `VSAVec` argument (no copy); otherwise a fresh {re, im} pair.
   */
  encode(value) {
    if (value instanceof VSAVec) return value;
    if (value instanceof VSAPair) {
      if (value.vec === null) {
        const carVec = this.encode(value.car);
        const cdrVec = this.encode(value.cdr);
        value.vec = bundleVecs([
          bindVec(this.CAR_ROLE, carVec, this.dim),
          bindVec(this.CDR_ROLE, cdrVec, this.dim),
        ], this.dim);
      }
      return value.vec;
    }
    if (value === nil) return this.NIL_VEC;
    if (value === true) return this.TRUE_VEC;
    if (value === false) return this.FALSE_VEC;
    if (typeof value === 'number') return fpeVec(value, this.dim, this.FPE_FREQ);
    if (typeof value === 'string') return this._cached('s', value);
    if (value instanceof ArliSymbol) return this._cached('y', value.name);
    if (Array.isArray(value)) {
      // Proper arli list: nil-terminated chain built from the last element back.
      let chain = nil;
      for (let i = value.length - 1; i >= 0; i--) chain = new VSAPair(value[i], chain);
      return this.encode(chain);
    }
    if (value instanceof Builtin) throw new Error('vsa: cannot encode builtin');
    if (value instanceof Function) throw new Error('vsa: cannot encode fn');
    if (typeof value === 'object' && value !== null) throw new Error('vsa: cannot encode map');
    throw new Error('vsa: cannot encode unknown');
  }

  /** Register a value in cleanup memory; returns the value. */
  remember(value) {
    const vec = this.encode(value);
    for (let i = 0; i < this.memory.length; i++) {
      if (this.memory[i].value === value) {
        this.memory[i].vec = vec;
        return value;
      }
    }
    this.memory.push({ vec, value });
    return value;
  }

  /** Highest-similarity registered value at or above the cleanup threshold. */
  lookup(vec) {
    let bestSim = -Infinity;
    let best = nil;
    for (const entry of this.memory) {
      const sim = similarityVec(vec, entry.vec, this.dim);
      if (sim > bestSim) {
        bestSim = sim;
        best = entry.value;
      }
    }
    if (bestSim >= CLEANUP_THRESHOLD) return best;
    return nil;
  }

  clearMemory() {
    this.memory = [];
  }
}

/** One engine per Evaluator, created on first use. */
const VSA_ENGINES = new WeakMap();

function engineFor(ev) {
  if (ev === null || ev === undefined || (typeof ev !== 'object' && typeof ev !== 'function')) {
    throw new Error('vsa: no evaluator context');
  }
  let engine = VSA_ENGINES.get(ev);
  if (engine === undefined) {
    engine = new VsaEngine(DEFAULT_DIM);
    VSA_ENGINES.set(ev, engine);
  }
  return engine;
}

// ---------------------------------------------------------------------------
// car / cdr / ->list over VSAPair, VSAVec and plain lists
// ---------------------------------------------------------------------------

function vsaCar(engine, value) {
  if (value instanceof VSAPair) return value.car;
  if (value instanceof VSAVec) {
    return engine.lookup(unbindVec(value, engine.CAR_ROLE, engine.dim));
  }
  if (Array.isArray(value)) return value.length > 0 ? value[0] : nil;
  if (value === nil) return nil;
  throw new Error('vsa-car: expected vsa-pair, vsa-vec, or list');
}

function vsaCdr(engine, value) {
  if (value instanceof VSAPair) return value.cdr;
  if (value instanceof VSAVec) {
    return engine.lookup(unbindVec(value, engine.CDR_ROLE, engine.dim));
  }
  if (Array.isArray(value)) return value.length > 1 ? value.slice(1) : nil;
  if (value === nil) return nil;
  throw new Error('vsa-cdr: expected vsa-pair, vsa-vec, or list');
}

function vsaToList(value) {
  if (value instanceof VSAPair) {
    const out = [];
    let node = value;
    while (node instanceof VSAPair) {
      out.push(node.car);
      node = node.cdr;
    }
    if (node !== nil) throw new Error('vsa->list: improper list');
    return out;
  }
  if (Array.isArray(value)) return value.slice();
  if (value === nil) return [];
  throw new Error('vsa->list: expected vsa-pair or list');
}

// ---------------------------------------------------------------------------
// vsa-match — pattern paths and their extraction
// ---------------------------------------------------------------------------

/**
 * Collect `?var` -> accessor path in pattern order. Element `i` of a list
 * pattern is reached with `['cdr'] * i + ['car']`.
 */
function collectPaths(pattern, path, out) {
  if (pattern instanceof ArliSymbol) {
    if (pattern.name.startsWith('?')) out.push({ sym: pattern, path: path.slice() });
    return;
  }
  if (!Array.isArray(pattern)) return;
  for (let i = 0; i < pattern.length; i++) {
    const sub = path.concat(new Array(i).fill('cdr'), ['car']);
    collectPaths(pattern[i], sub, out);
  }
}

/**
 * Walk an accessor path for `vsa-match` (§11). Unlike a direct vsa-car/vsa-cdr
 * call, this resolves each step without consulting cleanup memory: a VSAVec is
 * unbound by the role and the RAW vector is kept, so nested structure survives
 * the walk. Exactly one cleanup happens, at the leaf. An atom cannot descend
 * any further and yields nil.
 */
function walkPath(engine, value, path) {
  let node = value;
  for (const op of path) {
    if (node instanceof VSAPair) {
      node = op === 'car' ? node.car : node.cdr;
    } else if (Array.isArray(node)) {
      node = op === 'car'
        ? (node.length > 0 ? node[0] : nil)
        : (node.length > 1 ? node.slice(1) : nil);
    } else if (node === nil) {
      node = nil;
    } else if (node instanceof VSAVec) {
      node = wrapVec(unbindVec(node, op === 'car' ? engine.CAR_ROLE : engine.CDR_ROLE, engine.dim));
    } else {
      node = nil;
    }
  }
  if (node instanceof VSAVec) node = engine.lookup(node);
  return node;
}

// ---------------------------------------------------------------------------
// vsa-type
// ---------------------------------------------------------------------------

function vsaTypeOf(value) {
  if (value === nil) return 'nil';
  if (typeof value === 'boolean') return 'bool';
  if (typeof value === 'number') return 'number';
  if (typeof value === 'string') return 'string';
  if (value instanceof ArliSymbol) return 'symbol';
  if (Array.isArray(value)) return 'list';
  if (value instanceof Builtin) return 'builtin';
  if (value instanceof Function) return 'fn';
  if (value instanceof VSAVec) return 'vsa-vec';
  if (value instanceof VSAPair) return 'pair';
  if (typeof value === 'object' && value !== null) return 'map';
  return 'unknown';
}

// ---------------------------------------------------------------------------
// Builtins
// ---------------------------------------------------------------------------

/**
 * Build the VSA primitive table. Same shape as `getBuiltins()`: a plain object
 * of `{name: Builtin}`, each builtin receiving `(args, evaluator)`.
 */
export function getVsaBuiltins() {
  const b = {};

  function reg(name, fn, arity, doc = '') { b[name] = new Builtin(name, fn, arity, doc); }

  // Engine state
  reg('vsa-dim', (args, ev) => engineFor(ev).dim, 0,
    'Current VSA dimension.');

  reg('vsa-reset', (args, ev) => {
    const n = args[0];
    if (!Number.isInteger(n) || n < MIN_DIM || n > MAX_DIM) throw new Error('vsa-reset: dimension out of range');
    engineFor(ev).reset(n);
    return n;
  }, 1, 'Reinitialize the engine at dimension n: clears memory and caches and restarts the stream from the last vsa-seed.');

  reg('vsa-seed', (args, ev) => {
    const n = args[0];
    if (!Number.isInteger(n)) throw new Error('vsa-seed: expected integer');
    engineFor(ev).reseed(n);
    return n;
  }, 1, 'Reseed the random vector stream; returns the seed.');

  reg('vsa-random', (args, ev) => engineFor(ev).randomVec(), 0,
    'A random unit-phase vector from the stream.');

  // Vector algebra
  reg('vsa-bind', (args, ev) => {
    const engine = engineFor(ev);
    return wrapVec(bindVec(engine.encode(args[0]), engine.encode(args[1]), engine.dim));
  }, 2, 'Circular convolution (binding) of two values.');

  const bundlePrim = (name, doc) => reg(name, (args, ev) => {
    if (args.length === 0) throw new Error(`${name}: needs at least 1 argument`);
    const engine = engineFor(ev);
    return wrapVec(bundleVecs(args.map(a => engine.encode(a)), engine.dim));
  }, -1, doc);

  bundlePrim('vsa-bundle', 'Normalized superposition of one or more values.');
  bundlePrim('vsa-majority', 'FHRR centroid — identical to vsa-bundle.');

  reg('vsa-unbind', (args, ev) => {
    const engine = engineFor(ev);
    return wrapVec(unbindVec(engine.encode(args[0]), engine.encode(args[1]), engine.dim));
  }, 2, 'Approximate inverse of vsa-bind.');

  reg('vsa-similarity', (args, ev) => {
    const engine = engineFor(ev);
    return similarityVec(engine.encode(args[0]), engine.encode(args[1]), engine.dim);
  }, 2, 'Cosine similarity in [-1, 1] between two encoded values.');

  reg('vsa-permute', (args, ev) => {
    const engine = engineFor(ev);
    const n = args[1];
    if (!Number.isInteger(n)) throw new Error('vsa-permute: expected integer shift');
    return wrapVec(permuteVec(engine.encode(args[0]), n, engine.dim));
  }, 2, 'Rotation of a vector by n positions (negative allowed).');

  reg('vsa-encode', (args, ev) => wrapVec(engineFor(ev).encode(args[0])), 1,
    'Encode any value to a VSA vector.');

  // Pairs and lists
  reg('vsa-cons', (args, ev) => new VSAPair(args[0], args[1]), 2,
    'A VSA pair (encoded lazily).');

  reg('vsa-car', (args, ev) => vsaCar(engineFor(ev), args[0]), 1,
    'Exact car of a VSA pair; approximate car of a vector or list.');

  reg('vsa-cdr', (args, ev) => vsaCdr(engineFor(ev), args[0]), 1,
    'Exact cdr of a VSA pair; approximate cdr of a vector or list.');

  reg('vsa-list', (args, ev) => {
    let chain = nil;
    for (let i = args.length - 1; i >= 0; i--) chain = new VSAPair(args[i], chain);
    return chain;
  }, -1, 'Build a nil-terminated chain of VSA pairs.');

  reg('vsa->list', (args, ev) => vsaToList(args[0]), 1,
    'Convert a VSA pair chain (or plain list) to a plain arli list.');

  reg('vsa-pair?', (args, ev) => args[0] instanceof VSAPair, 1, 'Is this a VSA pair?');
  reg('vsa-vec?', (args, ev) => args[0] instanceof VSAVec, 1, 'Is this a VSA vector?');

  reg('vsa-type', (args, ev) => new ArliSymbol(vsaTypeOf(args[0])), 1,
    'Type class of a value as a symbol.');

  // Cleanup (associative) memory
  reg('vsa-register', (args, ev) => engineFor(ev).remember(args[0]), 1,
    'Register a value in the cleanup memory.');

  reg('vsa-cleanup', (args, ev) => {
    const engine = engineFor(ev);
    return engine.lookup(engine.encode(args[0]));
  }, 1, 'Nearest registered value above the cleanup threshold, else nil.');

  reg('vsa-query', (args, ev) => {
    const engine = engineFor(ev);
    const vec = engine.encode(args[0]);
    const k = toInt(args[1]);
    const scored = engine.memory.map(entry => [similarityVec(vec, entry.vec, engine.dim), entry.value]);
    scored.sort((x, y) => y[0] - x[0]);
    const kept = k !== null && k > 0 ? scored.slice(0, k) : scored;
    return kept.map(pair => new VSAPair(pair[0], pair[1]));
  }, 2, 'All memory entries as VSA pairs (car similarity, cdr value), nearest first, capped at k.');

  reg('vsa-clear', (args, ev) => { engineFor(ev).clearMemory(); return nil; }, 0,
    'Empty the cleanup memory.');

  // Resonator factorization
  reg('vsa-factorize', (args, ev) => {
    if (args.length < 2) throw new Error('vsa-factorize: needs at least 2 arguments');
    const engine = engineFor(ev);
    const target = engine.encode(args[0]);
    const codebooks = args.slice(1);
    for (const codebook of codebooks) {
      if (!Array.isArray(codebook)) throw new Error('vsa-factorize: expected codebook list');
      if (codebook.length === 0) throw new Error('vsa-factorize: empty codebook');
    }
    const est = codebooks.map(codebook => wrapVec(bundleVecs(codebook.map(v => engine.encode(v)), engine.dim)));
    for (let round = 0; round < 32; round++) {
      let changed = false;
      for (let i = 0; i < est.length; i++) {
        const others = [];
        for (let j = 0; j < est.length; j++) { if (j !== i) others.push(engine.encode(est[j])); }
        const cand = unbindVec(target, bundleVecs(others, engine.dim), engine.dim);
        let best = codebooks[i][0];
        let bestSim = -Infinity;
        for (const item of codebooks[i]) {
          const sim = similarityVec(cand, engine.encode(item), engine.dim);
          if (sim > bestSim) {
            bestSim = sim;
            best = item;
          }
        }
        if (best !== est[i]) {
          est[i] = best;
          changed = true;
        }
      }
      if (!changed) break;
    }
    return est.slice();
  }, -1, 'Resonator factorization of a bundle against one codebook per factor.');

  // Pattern matching
  reg('vsa-match', (args, ev) => {
    const engine = engineFor(ev);
    const paths = [];
    collectPaths(args[0], [], paths);
    return paths.map(binding => [binding.sym, walkPath(engine, args[1], binding.path)]);
  }, 2, 'Bind ?vars in a list pattern against a value, in pattern order.');

  // Raw float representation
  reg('vsa->floats', (args, ev) => {
    const engine = engineFor(ev);
    const vec = engine.encode(args[0]);
    const out = new Array(engine.dim * 2);
    for (let k = 0; k < engine.dim; k++) {
      out[2 * k] = vec.re[k];
      out[2 * k + 1] = vec.im[k];
    }
    return out;
  }, 1, 'Interleaved [re0 im0 re1 im1 ...] floats of an encoded value.');

  reg('floats->vsa', (args, ev) => {
    const engine = engineFor(ev);
    const dim = engine.dim;
    const floats = args[0];
    if (!Array.isArray(floats) || floats.length !== dim * 2) {
      throw new Error('floats->vsa: expected 2*D floats');
    }
    const re = new Float32Array(dim);
    const im = new Float32Array(dim);
    for (let k = 0; k < dim; k++) {
      re[k] = floats[2 * k];
      im[k] = floats[2 * k + 1];
    }
    return new VSAVec(re, im);
  }, 1, 'Build a VSA vector from an interleaved float list.');

  return b;
}
