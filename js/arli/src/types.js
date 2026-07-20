/**
 * Core data types for arli (JavaScript port).
 *
 * arli's type system is minimal:
 * - Numbers (Number)
 * - Strings (String)
 * - Symbols (ArliSymbol class)
 * - Lists (Array) — for parenthesized expressions, data, or function bodies
 * - Nil (singleton) — the empty list / false value
 * - Builtin (JS functions wrapped with arity metadata)
 * - Function (user-defined closures)
 * - Boolean (JS boolean true/false, printed as "true"/"false")
 */

// ---------------------------------------------------------------------------
// ArliSymbol
// ---------------------------------------------------------------------------

export class ArliSymbol {
  constructor(name) {
    this.name = name;
  }

  toString() {
    return this.name;
  }

  equals(other) {
    if (other instanceof ArliSymbol) {
      return this.name === other.name;
    }
    return false;
  }
}

// ---------------------------------------------------------------------------
// Nil — singleton false value
// ---------------------------------------------------------------------------

class NilType {
  constructor() {
    // singleton
  }

  toString() {
    return 'nil';
  }

  get [Symbol.toStringTag]() {
    return 'nil';
  }
}

/** Singleton nil — the empty list / canonical false value. */
export const nil = new NilType();

// ---------------------------------------------------------------------------
// Builtin — JS function wrapped with arity metadata
// ---------------------------------------------------------------------------

export class Builtin {
  constructor(name, fn, arity, doc = '') {
    this.name = name;
    this.fn = fn;
    this.arity = arity; // -1 means variadic / unknown
    this.doc = doc;
  }

  call(args, evaluator) {
    return this.fn(args, evaluator);
  }

  toString() {
    return `<Builtin ${this.name} arity=${this.arity}>`;
  }
}

// ---------------------------------------------------------------------------
// Function — user-defined closure
// ---------------------------------------------------------------------------

export class Function {
  constructor(params, body, env, name = '', isFexpr = false) {
    this.params = params; // Array of ArliSymbol
    this.body = body;     // Array of AST nodes
    this.env = env;       // closure environment
    this.name = name;
    this.arity = params.length;
    this.isFexpr = isFexpr;
  }

  toString() {
    const kind = this.isFexpr ? 'fexpr' : 'fn';
    const n = this.name || 'anon';
    return `<${kind} ${n}>`;
  }
}

// ---------------------------------------------------------------------------
// Truthiness
// ---------------------------------------------------------------------------

/**
 * Check truthiness. Only nil is false.
 */
export function isTruthy(val) {
  return val !== nil && val !== false && !(val instanceof NilType);
}

// ---------------------------------------------------------------------------
// Arli repr — convert a value to its string representation
// ---------------------------------------------------------------------------

export function arliRepr(val) {
  if (val === nil || val instanceof NilType) {
    return 'nil';
  }
  if (val instanceof ArliSymbol) {
    return val.name;
  }
  if (Array.isArray(val)) {
    if (val.length === 0) {
      return '()';
    }
    return '(' + val.map(v => arliRepr(v)).join(' ') + ')';
  }
  if (typeof val === 'boolean') {
    return val ? 'true' : 'false';
  }
  if (typeof val === 'number') {
    if (Number.isInteger(val)) {
      return String(val);
    }
    return String(val);
  }
  if (typeof val === 'string') {
    return `"${val}"`;
  }
  if (val instanceof Builtin) {
    return `<builtin ${val.name}>`;
  }
  if (val instanceof Function) {
    return val.toString();
  }
  if (typeof val === 'object' && val !== null && !Array.isArray(val)) {
    // Plain object (hash-map)
    const keys = Object.keys(val);
    const parts = keys.map(k => {
      const keyStr = k.startsWith(':') ? `'${k}'` : `'${k}'`;
      return `${keyStr}: ${arliRepr(val[k])}`;
    });
    return '{' + parts.join(', ') + '}';
  }
  return String(val);
}

/**
 * Deep equality for arli values.
 */
export function arliEqual(a, b) {
  if (a === b) return true;
  if (a instanceof ArliSymbol && b instanceof ArliSymbol) {
    return a.name === b.name;
  }
  if (a instanceof NilType && b instanceof NilType) return true;
  if (a === nil && b === nil) return true;
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!arliEqual(a[i], b[i])) return false;
    }
    return true;
  }
  if (typeof a === 'number' && typeof b === 'number') {
    return a === b;
  }
  if (typeof a === 'string' && typeof b === 'string') {
    return a === b;
  }
  if (typeof a === 'boolean' && typeof b === 'boolean') {
    return a === b;
  }
  return false;
}
