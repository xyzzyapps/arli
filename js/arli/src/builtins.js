/**
 * Built-in functions for arli (JavaScript port).
 *
 * Each builtin has a known arity. Arity -1 means 'unknown/variadic' and
 * requires parentheses in source.
 *
 * All builtins receive (args, evaluator) where args is the array of evaluated
 * arguments and evaluator is the Evaluator instance (for stack access).
 */

import { ArliSymbol, nil, Builtin, Function, isTruthy, arliRepr } from './types.js';

export function getBuiltins() {
  const b = {};

  function reg(name, fn, arity, doc = '') { b[name] = new Builtin(name, fn, arity, doc); }

  // Arithmetic
  reg('+', (args, ev) => args[0] + args[1], 2);
  reg('-', (args, ev) => args[0] - args[1], 2);
  reg('*', (args, ev) => args[0] * args[1], 2);
  reg('/', (args, ev) => args[0] / args[1], 2);
  reg('//', (args, ev) => Math.floor(args[0] / args[1]), 2);
  reg('%', (args, ev) => args[0] % args[1], 2);
  reg('neg', (args, ev) => -args[0], 1);

  // Comparison
  reg('=', (args, ev) => args[0] === args[1], 2);
  reg('<', (args, ev) => args[0] < args[1], 2);
  reg('>', (args, ev) => args[0] > args[1], 2);
  reg('<=', (args, ev) => args[0] <= args[1], 2);
  reg('>=', (args, ev) => args[0] >= args[1], 2);
  reg('!=', (args, ev) => args[0] !== args[1], 2);

  // Logic
  reg('and', (args, ev) => {
    for (const arg of args) { if (!isTruthy(arg)) return arg; }
    return args.length > 0 ? args[args.length - 1] : true;
  }, -1);

  reg('or', (args, ev) => {
    for (const arg of args) { if (isTruthy(arg)) return arg; }
    return nil;
  }, -1);

  reg('not', (args, ev) => !isTruthy(args[0]), 1);

  // Stack operations (Forth-like)
  reg('dup', (args, ev) => { if (ev) ev.stack.push(args[0]); return args[0]; }, 1);
  reg('swap', (args, ev) => {
    if (ev) { ev.stack.pop(); ev.stack.pop(); ev.stack.push(args[1]); ev.stack.push(args[0]); }
    return args[1];
  }, 2);
  reg('drop', (args, ev) => nil, 1);
  reg('over', (args, ev) => { if (ev) ev.stack.push(args[0]); return args[0]; }, 2);
  reg('rot', (args, ev) => {
    if (ev) { ev.stack.pop(); ev.stack.pop(); ev.stack.pop(); ev.stack.push(args[1]); ev.stack.push(args[2]); ev.stack.push(args[0]); }
    return args[2];
  }, 3);
  reg('nip', (args, ev) => { if (ev) { ev.stack.pop(); ev.stack.pop(); ev.stack.push(args[1]); } return args[1]; }, 2);
  reg('tuck', (args, ev) => {
    if (ev) { ev.stack.pop(); ev.stack.pop(); ev.stack.push(args[1]); ev.stack.push(args[0]); ev.stack.push(args[1]); }
    return args[1];
  }, 2);

  reg('pick', (args, ev) => {
    if (ev && typeof args[0] === 'number') {
      const n = Math.floor(args[0]);
      const stackLen = ev.stack.length;
      if (n >= 0 && n < stackLen) { const val = ev.stack[stackLen - 1 - n]; ev.stack.push(val); return val; }
    }
    return args[0];
  }, 1);

  reg('roll', (args, ev) => {
    if (ev && typeof args[0] === 'number') {
      const depth = Math.floor(args[0]);
      const stackLen = ev.stack.length;
      if (depth >= 0 && depth < stackLen) { const val = ev.stack.splice(stackLen - 1 - depth, 1)[0]; ev.stack.push(val); return val; }
    }
    return args[0];
  }, 1);

  // List operations
  reg('cons', (args, ev) => { const a = args[0], b = args[1]; if (b === nil) return [a]; if (Array.isArray(b)) return [a].concat(b); return [a, b]; }, 2);
  reg('car', (args, ev) => { const lst = args[0]; if (Array.isArray(lst) && lst.length > 0) return lst[0]; if (Array.isArray(lst)) return nil; throw new TypeError(`car: expected list, got ${typeof lst}`); }, 1);
  reg('cdr', (args, ev) => { const lst = args[0]; if (Array.isArray(lst) && lst.length > 1) return lst.slice(1); if (Array.isArray(lst)) return nil; throw new TypeError(`cdr: expected list, got ${typeof lst}`); }, 1);
  reg('list', (args, ev) => args.slice(), -1);
  reg('nil?', (args, ev) => args[0] === nil, 1);
  reg('list?', (args, ev) => Array.isArray(args[0]), 1);

  // I/O
  reg('print', (args, ev) => { console.log(arliRepr(args[0])); return args[0]; }, 1);
  reg('.', (args, ev) => {
    // Forth-style dot — print value (no newline). Works in Node and browser.
    if (typeof process !== 'undefined' && process.stdout) {
      process.stdout.write(arliRepr(args[0]));
    } else {
      console.log(arliRepr(args[0]));
    }
    return args[0];
  }, 1);
  reg('read', (args, ev) => nil, 0);

  // Type checking
  reg('number?', (args, ev) => typeof args[0] === 'number', 1);
  reg('string?', (args, ev) => typeof args[0] === 'string', 1);
  reg('symbol?', (args, ev) => args[0] instanceof ArliSymbol, 1);
  reg('fn?', (args, ev) => args[0] instanceof Builtin || args[0] instanceof Function, 1);

  // Sequence operations
  reg('map', (args, ev) => {
    const fn = args[0], lst = args[1];
    if (!Array.isArray(lst) || !ev) return lst;
    return lst.map(x => ev.apply(fn, [x]));
  }, 2);

  reg('filter', (args, ev) => {
    const fn = args[0], lst = args[1];
    if (!Array.isArray(lst) || !ev) return lst;
    return lst.filter(x => isTruthy(ev.apply(fn, [x])));
  }, 2);

  reg('reduce', (args, ev) => {
    const fn = args[0], init = args[1], lst = args[2];
    if (!Array.isArray(lst) || lst.length === 0 || !ev) return init;
    let acc = init;
    for (const x of lst) acc = ev.apply(fn, [acc, x]);
    return acc;
  }, 3);

  // Result type constructors
  reg('Ok', (args, ev) => [new ArliSymbol('Ok'), args[0]], 1);
  reg('Err', (args, ev) => [new ArliSymbol('Err'), args[0]], 1);

  reg('map-ok', (args, ev) => {
    if (!ev) return args[0];
    const result = args[0], fn = args[1];
    if (Array.isArray(result) && result.length === 2 && result[0] instanceof ArliSymbol && result[0].name === 'Ok') return [new ArliSymbol('Ok'), ev.apply(fn, [result[1]])];
    return result;
  }, 2);

  reg('and-then', (args, ev) => {
    if (!ev) return args[0];
    const result = args[0], fn = args[1];
    if (Array.isArray(result) && result.length === 2 && result[0] instanceof ArliSymbol && result[0].name === 'Ok') return ev.apply(fn, [result[1]]);
    return result;
  }, 2);

  reg('or-else', (args, ev) => {
    if (!ev) return args[0];
    const result = args[0], fn = args[1];
    if (Array.isArray(result) && result.length === 2 && result[0] instanceof ArliSymbol && result[0].name === 'Ok') return result;
    return ev.apply(fn, []);
  }, 2);

  // Hash-map
  reg('hash-map', (args, ev) => {
    const result = {};
    for (let i = 0; i < args.length - 1; i += 2) {
      const key = args[i], val = args[i + 1];
      result[key instanceof ArliSymbol ? key.name : String(key)] = val;
    }
    return result;
  }, -1);

  // Stack reflection
  reg('stack', (args, ev) => { if (!ev) return []; return ev.stack.slice(); }, 0);

  reg('stack!', (args, ev) => {
    if (!ev) return args[0];
    const newStack = args[0];
    if (Array.isArray(newStack)) ev.stack = newStack.slice();
    else if (newStack === nil) ev.stack = [];
    else ev.stack = [newStack];
    return null;
  }, 1);

  // Exec stack operations
  reg('exec-stack', (args, ev) => { if (!ev) return []; return ev.execStack.slice(); }, 0);

  reg('exec!', (args, ev) => {
    if (!ev) return args[0];
    const newStack = args[0];
    if (Array.isArray(newStack)) ev.execStack = newStack.slice();
    else if (newStack === nil) ev.execStack = [];
    else ev.execStack = [newStack];
    return null;
  }, 1);

  reg('exec-push', (args, ev) => { if (ev) ev.execStack.push(args[0]); return null; }, 1);
  reg('exec-pop', (args, ev) => { if (!ev || ev.execStack.length === 0) return nil; return ev.execStack.pop(); }, 0);
  reg('exec-depth', (args, ev) => { if (!ev) return 0; return ev.execStack.length; }, 0);
  reg('exec-step', (args, ev) => { if (!ev || ev.execStack.length === 0) return nil; return ev.evalExpr(ev.execStack.pop()); }, 0);

  // exec: Push interpreter (variadic, parens required)
  reg('exec', (args, ev) => {
    if (!ev) return nil;
    while (ev.execStack.length > 0) {
      const item = ev.execStack.pop();

      if (Array.isArray(item)) {
        const result = ev.evalExpr(item);
        if (result !== null && result !== undefined) ev.stack.push(result);
      } else if (item instanceof ArliSymbol) {
        if (item.name === '__restore_env__') {
          if (ev.envStack.length > 0) ev.env = ev.envStack.pop();
          continue;
        }
        if (item.name === 'if') {
          const condVal = ev.stack.length > 0 ? ev.stack.pop() : nil;
          const elseBranch = ev.execStack.length > 0 ? ev.execStack.pop() : nil;
          const thenBranch = ev.execStack.length > 0 ? ev.execStack.pop() : nil;
          ev.execStack.push(isTruthy(condVal) ? thenBranch : elseBranch);
          continue;
        }
        if (item.name === 'do') { continue; }
        if (item.name === 'quote') {
          const quoted = ev.execStack.length > 0 ? ev.execStack.pop() : nil;
          ev.stack.push(quoted);
          continue;
        }
        const fn = ev.env.get(item.name);
        if (fn instanceof Builtin) {
          const arity = fn.arity >= 0 ? fn.arity : 0;
          const fnArgs = [];
          for (let i = 0; i < arity; i++) { if (ev.stack.length > 0) fnArgs.unshift(ev.stack.pop()); }
          const result = fn.call(fnArgs, ev);
          if (result !== null && result !== undefined) ev.stack.push(result);
        } else if (fn instanceof Function) {
          const fnArgs = [];
          for (let i = 0; i < fn.params.length; i++) { if (ev.stack.length > 0) fnArgs.unshift(ev.stack.pop()); }
          ev.envStack.push(ev.env);
          const callEnv = fn.env.extend('call');
          for (let i = 0; i < fn.params.length; i++) { if (i < fnArgs.length) callEnv.define(fn.params[i].name, fnArgs[i]); }
          ev.execStack.push(new ArliSymbol('__restore_env__'));
          const body = Array.isArray(fn.body) ? fn.body : [fn.body];
          for (let i = body.length - 1; i >= 0; i--) ev.execStack.push(body[i]);
          ev.env = callEnv;
        } else {
          ev.stack.push(fn);
        }
      } else {
        ev.stack.push(item);
      }
    }
    return null;
  }, -1);

  // Evaluation control
  reg('eval', (args, ev) => { if (!ev) return args[0]; return ev.evalForm(args[0]); }, 1);

  return b;
}
