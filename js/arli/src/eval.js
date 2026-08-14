/**
 * Stack-based evaluator for arli (JavaScript port).
 */

import { ArliSymbol, nil, Builtin, Function, isTruthy, arliRepr, arliEqual } from './types.js';
import { Environment } from './env.js';
import { ArityTable, Parser } from './parse.js';
import { tokenize, TokenStream } from './tokenize.js';
import { getBuiltins } from './builtins.js';

let nodeFs = null;
let nodeRequire = null;
if (typeof process !== 'undefined' && process.versions && process.versions.node) {
  try {
    const { createRequire } = await import('node:module');
    nodeRequire = createRequire(import.meta.url);
    nodeFs = nodeRequire('node:fs');
  } catch (e) {}
}

export class Evaluator {
  constructor(debug = false) {
    this.stack = [];
    this.execStack = [];
    this.envStack = [];
    this.globalEnv = new Environment(null, 'global');
    this.env = this.globalEnv;
    this.arityTable = new ArityTable();
    this.debug = debug;
    this._parser = null;
    this._loadBuiltins();
  }

  get parser() {
    if (this._parser === null) this._parser = new Parser(this.arityTable);
    return this._parser;
  }

  _loadBuiltins() {
    for (const [name, builtin] of Object.entries(getBuiltins())) {
      this.globalEnv.define(name, builtin);
      if (builtin.arity >= 0) this.arityTable.register(name, builtin.arity);
    }
    this.arityTable.register('define', 2);
    this.arityTable.register('quote', 1);
    this.arityTable.register('do', -1);
    this.arityTable.register('set!', 2);
    this.arityTable.register('let', 2);
    this.arityTable.register('if', 3);
    this.arityTable.register('while', 2);
    this.arityTable.register('for', 3);
    this.arityTable.register('cond', 1);
    this.arityTable.register('defn', 3);
    this.arityTable.register('defn-rec', 3);
    this.arityTable.register('fn', 2);
    this.arityTable.register('import', 1);
    this.arityTable.register('import!', 2);
    this.arityTable.register('.', 2);
    this.arityTable.register('host', 1);
    this.arityTable.register('assert', 2);
    this.arityTable.register('doc', 1);
    this.arityTable.register('doc!', 2);
    this.arityTable.register('match', -1);
    this.arityTable.register('import-module', 1);
    this.arityTable.register('defn-fexpr', 3);
    this.arityTable.register('eval', 1);
  }

  evalForm(form) { return this._evalExpr(form); }

  eval(expr) {
    const result = this._evalExpr(expr);
    if (result !== null && result !== undefined) this.stack.push(result);
    return result;
  }

  _evalExpr(expr) {
    if (this.debug) {
      const stackPreview = this.stack.slice(-3).map(arliRepr).join(',');
      console.log(`  EVAL: ${arliRepr(expr)}  stack=[${stackPreview}]`);
    }

    if (typeof expr === 'number' || typeof expr === 'string') return expr;
    if (expr === nil) return nil;
    if (typeof expr === 'boolean') return expr;

    if (expr instanceof ArliSymbol) {
      const name = expr.name;
      if (name.startsWith(':')) return expr;
      if (name === 'nil') return nil;
      if (name === 'true') return true;
      if (name === 'false') return false;
      const val = this.env.get(name);
      if (val === null || val === undefined) throw new ReferenceError(`Undefined symbol: ${name}`);
      return val;
    }

    if (Array.isArray(expr)) {
      if (expr.length === 0) return nil;
      const head = expr[0];

      // Special forms
      if (head instanceof ArliSymbol && head.name === 'quote') {
        if (expr.length < 2) throw new SyntaxError('quote expects 1 argument');
        return expr[1];
      }

      if (head instanceof ArliSymbol && head.name === 'define') {
        if (expr.length < 3) throw new SyntaxError('define expects (define name value)');
        const nameExpr = expr[1], valueExpr = expr[2];
        if (nameExpr instanceof ArliSymbol) {
          const name = nameExpr.name;
          const value = this._evalExpr(valueExpr);
          this.env.define(name, value);
          if (value instanceof Function) this.arityTable.register(name, value.arity);
          return value;
        }
        throw new SyntaxError(`define expects a symbol name, got ${nameExpr}`);
      }

      if (head instanceof ArliSymbol && head.name === 'defn') {
        if (expr.length < 4) throw new SyntaxError('defn expects (defn name (params) body...)');
        const nameSym = expr[1], params = expr[2], body = expr.slice(3);
        if (!(nameSym instanceof ArliSymbol)) throw new SyntaxError(`defn expects a symbol name, got ${nameSym}`);
        if (!Array.isArray(params)) throw new SyntaxError('defn expects a parameter list');
        const fn = new Function(params, body, this.env, nameSym.name);
        this.env.define(nameSym.name, fn);
        this.arityTable.register(nameSym.name, params.length);
        return fn;
      }

      if (head instanceof ArliSymbol && head.name === 'if') {
        if (expr.length < 4) throw new SyntaxError('if expects (if cond then else)');
        const cond = this._evalExpr(expr[1]);
        return this._evalExpr(isTruthy(cond) ? expr[2] : expr[3]);
      }

      if (head instanceof ArliSymbol && head.name === 'do') {
        let result = nil;
        for (const sub of expr.slice(1)) result = this._evalExpr(sub);
        return result;
      }

      if (head instanceof ArliSymbol && head.name === 'fn') {
        if (expr.length < 3) throw new SyntaxError('fn expects (fn (params) body...)');
        const params = expr[1], body = expr.slice(2);
        if (!Array.isArray(params)) throw new SyntaxError('fn expects a parameter list');
        return new Function(params, body, this.env);
      }

      if (head instanceof ArliSymbol && head.name === 'while') {
        if (expr.length < 3) throw new SyntaxError('while expects (while cond body)');
        let result = nil;
        while (isTruthy(this._evalExpr(expr[1]))) { for (const sub of expr.slice(2)) result = this._evalExpr(sub); }
        return result;
      }

      if (head instanceof ArliSymbol && head.name === 'for') {
        if (expr.length < 4) throw new SyntaxError('for expects (for var list body)');
        const varExpr = expr[1];
        const listVal = this._evalExpr(expr[2]);
        const bodyExprs = expr.slice(3);
        if (!(varExpr instanceof ArliSymbol)) throw new SyntaxError(`for expects a symbol as variable, got ${varExpr}`);
        if (!Array.isArray(listVal)) throw new TypeError(`for expects a list, got ${typeof listVal}`);
        let result = nil;
        for (const item of listVal) { this.env.define(varExpr.name, item); for (const sub of bodyExprs) result = this._evalExpr(sub); }
        return result;
      }

      if (head instanceof ArliSymbol && head.name === 'cond') {
        if (expr.length < 2) throw new SyntaxError('cond expects (cond clause...)');
        const clauses = expr[1];
        if (Array.isArray(clauses)) {
          let i = 0;
          while (i < clauses.length - 1) {
            if (isTruthy(this._evalExpr(clauses[i]))) return this._evalExpr(clauses[i + 1]);
            i += 2;
          }
          if (i < clauses.length && isTruthy(this._evalExpr(clauses[i]))) return this._evalExpr(clauses[i]);
        }
        return nil;
      }

      if (head instanceof ArliSymbol && head.name === 'set!') {
        if (expr.length < 3) throw new SyntaxError('set! expects (set! target value)');
        const nameExpr = expr[1];
        const value = this._evalExpr(expr[expr.length - 1]);
        if (nameExpr instanceof ArliSymbol && expr.length === 3) { this.env.set(nameExpr.name, value); return value; }
        if (expr.length >= 4) {
          const obj = this._evalExpr(expr[1]), key = expr[2];
          if (typeof obj === 'object' && obj !== null && !Array.isArray(obj)) {
            if (key instanceof ArliSymbol && key.name.startsWith(':')) obj[key.name] = value;
            else if (typeof key === 'string') obj[key] = value;
            else obj[String(key)] = value;
            return value;
          }
          if (Array.isArray(obj) && typeof key === 'number') { const idx = Math.floor(key); if (idx >= 0 && idx < obj.length) { obj[idx] = value; return value; } }
        }
        throw new SyntaxError(`set! cannot set on target: ${arliRepr(nameExpr)}`);
      }

      if (head instanceof ArliSymbol && head.name === 'let') {
        if (expr.length < 3) throw new SyntaxError('let expects (let ((name val)...) body...)');
        const bindings = expr[1], body = expr.slice(2);
        const letEnv = this.env.extend('let');
        const oldEnv = this.env;
        this.env = letEnv;
        try {
          if (Array.isArray(bindings)) {
            for (const binding of bindings) {
              if (Array.isArray(binding) && binding.length >= 2) {
                const bname = binding[0], bval = this._evalExpr(binding[1]);
                if (bname instanceof ArliSymbol) this.env.define(bname.name, bval);
              }
            }
          }
          let result = nil;
          for (const sub of body) result = this._evalExpr(sub);
          return result;
        } finally { this.env = oldEnv; }
      }

      if (head instanceof ArliSymbol && (head.name === 'import' || head.name === 'import!')) {
        if (expr.length < 2) throw new SyntaxError('import expects (import module-name)');
        let moduleName = '';
        const nameTok = expr[1];
        if (nameTok instanceof ArliSymbol) moduleName = nameTok.name;
        else if (typeof nameTok === 'string') moduleName = nameTok;
        else throw new TypeError(`import expects a symbol or string, got ${nameTok}`);
        // Node: sync require(). Browser: fire dynamic import(), module available when resolved.
        try {
          let name = moduleName;
          if (head.name === 'import!' && expr.length >= 3 && expr[2] instanceof ArliSymbol) name = expr[2].name;
          let mod;
          if (nodeRequire) {
            mod = nodeRequire(moduleName);
            this.env.define(name, mod);
            return mod;
          } else {
            // Browser ESM: dynamic import() — returns a Promise.
            // We return a loading marker; the real module replaces it when resolved.
            const marker = { __module__: moduleName, __loading__: true };
            this.env.define(name, marker);
            import(moduleName)
              .then(m => { this.env.define(name, m); })
              .catch(() => { /* import failed, marker remains */ });
            return marker;
          }
        } catch (e) {
          const marker = { __module__: moduleName, __error__: e.message };
          let name = moduleName;
          if (head.name === 'import!' && expr.length >= 3 && expr[2] instanceof ArliSymbol) name = expr[2].name;
          this.env.define(name, marker);
          return marker;
        }
      }

      if (head instanceof ArliSymbol && head.name === '.') {
        if (expr.length < 3) throw new SyntaxError('. expects (. obj attr [attr...] [args...])');
        let obj = this._evalExpr(expr[1]);
        let i = 2;
        while (i < expr.length) {
          const item = expr[i];
          if (item instanceof ArliSymbol) { obj = obj[item.name]; i++; }
          else break;
        }
        if (i < expr.length) {
          const callArgs = [];
          for (let j = i; j < expr.length; j++) callArgs.push(this._evalExpr(expr[j]));
          if (typeof obj === 'function') return obj(...callArgs);
          throw new TypeError(`Cannot call non-callable: ${obj}`);
        }
        return obj;
      }

      if (head instanceof ArliSymbol && head.name === 'host') {
        if (expr.length < 2) throw new SyntaxError('host expects (host "code")');
        const code = expr[1];
        if (typeof code === 'string') {
          try {
            const fn = new Function(...Object.keys(this.globalEnv._bindings), `return ${code}`);
            return fn(...Object.values(this.globalEnv._bindings));
          } catch (e) { throw new Error(`Host eval error: ${e.message}`); }
        }
        throw new TypeError(`host expects a string, got ${typeof code}`);
      }

      if (head instanceof ArliSymbol && head.name === 'assert') {
        if (expr.length < 2) throw new SyntaxError('assert expects (assert expr message)');
        const val = this._evalExpr(expr[1]);
        if (!isTruthy(val)) {
          let msg = '';
          if (expr.length >= 3) msg = arliRepr(this._evalExpr(expr[2]));
          throw new Error(`Assertion failed: ${msg}`);
        }
        return val;
      }

      if (head instanceof ArliSymbol && head.name === 'doc') {
        if (expr.length >= 2 && expr[1] instanceof ArliSymbol) {
          const docVal = this.env.lookup(`__doc_${expr[1].name}`);
          if (docVal !== null && docVal !== undefined) return docVal;
        }
        return nil;
      }

      if (head instanceof ArliSymbol && head.name === 'doc!') {
        if (expr.length >= 3 && expr[1] instanceof ArliSymbol) {
          const docText = this._evalExpr(expr[2]);
          this.env.define(`__doc_${expr[1].name}`, docText);
          return docText;
        }
        return nil;
      }

      if (head instanceof ArliSymbol && head.name === 'match') {
        if (expr.length < 2) throw new SyntaxError('match expects (match expr clause...)');
        const matchVal = this._evalExpr(expr[1]);
        for (const clause of expr.slice(2)) {
          if (Array.isArray(clause) && clause.length >= 2) {
            if (this._matchPattern(clause[0], matchVal)) return this._evalExpr(clause[1]);
          }
        }
        return nil;
      }

      if (head instanceof ArliSymbol && head.name === 'import-module') {
        if (expr.length < 2) throw new SyntaxError('import-module expects (import-module "path")');
        const path = this._evalExpr(expr[1]);
        if (typeof path === 'string') {
          return this.execFile(path);
        }
        throw new TypeError(`import-module expects a string path`);
      }

      if (head instanceof ArliSymbol && head.name === 'defn-rec') {
        if (expr.length < 4) throw new SyntaxError('defn-rec expects (defn-rec name (params) body...)');
        const nameSym = expr[1], params = expr[2], body = expr.slice(3);
        if (!(nameSym instanceof ArliSymbol)) throw new SyntaxError(`defn-rec expects a symbol name, got ${nameSym}`);
        if (!Array.isArray(params)) throw new SyntaxError('defn-rec expects a parameter list');
        const fn = new Function(params, body, this.env, nameSym.name);
        this.env.define(nameSym.name, fn);
        this.arityTable.register(nameSym.name, params.length);
        return fn;
      }

      if (head instanceof ArliSymbol && head.name === 'defn-fexpr') {
        if (expr.length < 4) throw new SyntaxError('defn-fexpr expects (defn-fexpr name (params) body...)');
        const nameSym = expr[1], params = expr[2], body = expr.slice(3);
        if (!(nameSym instanceof ArliSymbol)) throw new SyntaxError(`defn-fexpr expects a symbol name, got ${nameSym}`);
        if (!Array.isArray(params)) throw new SyntaxError('defn-fexpr expects a parameter list');
        const fn = new Function(params, body, this.env, nameSym.name, true);
        this.env.define(nameSym.name, fn);
        this.arityTable.register(nameSym.name, params.length);
        return fn;
      }

      // Generic function call
      let fnVal = this._evalExpr(head);
      while (fnVal instanceof ArliSymbol) {
        fnVal = this.env.lookup(fnVal.name);
      }
      if (fnVal instanceof Function && fnVal.isFexpr) return this._applyFunction(fnVal, expr.slice(1));
      const args = expr.slice(1).map(arg => this._evalExpr(arg));
      if (fnVal instanceof Builtin) return fnVal.call(args, this);
      if (fnVal instanceof Function) return this._applyFunction(fnVal, args);
      if (typeof fnVal === 'function') return fnVal(...args);
      throw new TypeError(`Cannot call non-function: ${arliRepr(fnVal)}`);
    }
    throw new TypeError(`Unknown expression type: ${typeof expr}: ${expr}`);
  }

  apply(fn, args) {
    while (fn instanceof ArliSymbol) {
      fn = this.env.lookup(fn.name);
    }
    if (fn instanceof Builtin) return fn.call(args, this);
    if (fn instanceof Function) return this._applyFunction(fn, args);
    if (typeof fn === 'function') return fn(...args);
    throw new TypeError(`Cannot call non-function: ${arliRepr(fn)}`);
  }

  _applyFunction(fn, args) {
    if (args.length !== fn.params.length) throw new TypeError(`Function expected ${fn.params.length} args, got ${args.length}`);
    const callEnv = fn.env.extend(`call(${fn.name || 'anon'})`);
    const oldEnv = this.env;
    this.env = callEnv;
    try {
      for (let i = 0; i < fn.params.length; i++) this.env.define(fn.params[i].name, args[i]);
      const body = Array.isArray(fn.body) ? fn.body : [fn.body];
      let result = nil;
      for (const sub of body) result = this._evalExpr(sub);
      return result;
    } finally { this.env = oldEnv; }
  }

  _matchPattern(pattern, value) {
    if (pattern instanceof ArliSymbol) {
      if (pattern.name === '_') return true;
      if (value instanceof ArliSymbol) return pattern.name === value.name;
      return false;
    }
    if (Array.isArray(pattern) && Array.isArray(value)) {
      if (pattern.length !== value.length) return false;
      for (let i = 0; i < pattern.length; i++) { if (!this._matchPattern(pattern[i], value[i])) return false; }
      return true;
    }
    return arliEqual(pattern, value);
  }

  exec(source) {
    const tokens = tokenize(source);
    const stream = new TokenStream(tokens);
    let result = nil;
    while (!stream.isEOF) {
      const expr = this.parser._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) result = this.eval(expr);
    }
    return result;
  }

  execFile(path) {
    // Node: sync readFile. Browser: fetch + fire exec async, return nil (result not available yet).
    if (nodeFs) {
      return this.exec(nodeFs.readFileSync(path, 'utf-8'));
    }
    // Browser: async fetch, execute when available
    if (typeof fetch !== 'undefined') {
      fetch(path)
        .then(r => { if (!r.ok) throw new Error(`fetch ${path}: ${r.status}`); return r.text(); })
        .then(source => this.exec(source))
        .catch(e => { console.error(`import-module error: ${e.message}`); });
      return nil;
    }
    return nil;
  }
}
