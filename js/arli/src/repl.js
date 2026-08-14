/**
 * Interactive REPL for arli (JavaScript port).
 */

import readline from 'readline';
import { Evaluator } from './eval.js';
import { arliRepr, nil } from './types.js';

export class REPL {
  constructor(evaluator = null, debug = false) {
    this.evaluator = evaluator || new Evaluator(debug);
    this.history = [];
  }

  run() {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: 'arli> ' });

    console.log('arli v1.0.0 (JavaScript port)');
    console.log('Arity-driven Lisp with Forth-like stack operations');
    console.log("Type 'help' for commands, 'exit' or Ctrl+C to quit\n");

    let currentInput = '';
    rl.prompt();

    rl.on('line', (line) => {
      line = line.trim();
      if (line === '' && currentInput === '') { rl.prompt(); return; }
      if (line === '') { this._evalLine(currentInput); currentInput = ''; rl.prompt(); return; }
      if (line === 'exit' || line === 'quit') { rl.close(); return; }
      if (line === 'help') { this._showHelp(); rl.prompt(); return; }
      if (line.startsWith('/')) { this._handleCommand(line.slice(1)); rl.prompt(); return; }

      currentInput = currentInput === '' ? line : currentInput + '\n' + line;
      if (!this._needsMoreInput(currentInput)) { this._evalLine(currentInput); currentInput = ''; rl.prompt(); }
      else { rl.setPrompt('   .. '); rl.prompt(); }
    });

    rl.on('close', () => { console.log(); process.exit(0); });
    rl.on('SIGINT', () => { console.log('^C'); currentInput = ''; rl.setPrompt('arli> '); rl.prompt(); });
  }

  _needsMoreInput(text) {
    const stripped = text.trim();
    if (!stripped) return false;
    let opens = 0, closes = 0;
    for (const ch of stripped) { if (ch === '(') opens++; if (ch === ')') closes++; }
    if (opens > closes) return true;
    let inString = false;
    for (let i = 0; i < stripped.length; i++) {
      if (stripped[i] === '"') inString = !inString;
      else if (stripped[i] === '\\' && i + 1 < stripped.length) i++;
    }
    return inString;
  }

  _evalLine(line) {
    this.history.push(line);
    try {
      const result = this.evaluator.exec(line);
      if (result !== null && result !== undefined && result !== nil) console.log(arliRepr(result));
    } catch (e) { console.error(`Error: ${e.message}`); }
  }

  _handleCommand(cmd) {
    const parts = cmd.trim().split(/\s+/);
    if (parts.length === 0) return;
    switch (parts[0]) {
      case 'stack': case 's': this._showStack(); break;
      case 'env': case 'e': this._showEnv(); break;
      case 'arity': case 'a': this._showArity(); break;
      case 'clear': case 'c': this.evaluator.stack = []; console.log('Stack cleared.'); break;
      case 'debug': case 'd': this.evaluator.debug = !this.evaluator.debug; console.log(`Debug mode: ${this.evaluator.debug}`); break;
      case 'reset': this.evaluator = new Evaluator(this.evaluator.debug); console.log('Evaluator reset.'); break;
      default: console.log(`Unknown command: ${parts[0]}\nCommands: /stack, /env, /arity, /clear, /debug, /reset`);
    }
  }

  _showStack() {
    const stack = this.evaluator.stack;
    if (stack.length === 0) { console.log('Stack is empty.'); return; }
    console.log(`Stack (${stack.length} items):`);
    for (let i = 0; i < stack.length; i++) console.log(`  ${i}: ${arliRepr(stack[i])}`);
  }

  _showEnv() {
    const bindings = {};
    let e = this.evaluator.env;
    while (e) { for (const [k, v] of Object.entries(e._bindings)) { if (!(k in bindings)) bindings[k] = v; } e = e.parent; }
    const keys = Object.keys(bindings);
    if (keys.length === 0) { console.log('Environment is empty.'); return; }
    console.log(`Environment (${keys.length} bindings):`);
    for (const name of keys.sort()) console.log(`  ${name}: ${arliRepr(bindings[name])}`);
  }

  _showArity() {
    const table = this.evaluator.arityTable._table;
    const keys = Object.keys(table);
    if (keys.length === 0) { console.log('No arity registrations.'); return; }
    console.log('Registered arities:');
    for (const name of keys.sort()) console.log(`  ${name}: ${table[name] >= 0 ? table[name] : 'variadic'}`);
  }

  _showHelp() {
    console.log('arli — Arity-driven Forth-like Lisp\n');
    console.log('Basic syntax:');
    console.log('  + 1 2          ; arity-driven, no parens needed');
    console.log('  * + 1 2 3      ; (= (* (+ 1 2) 3))');
    console.log('  define x + 1 2 ; define with arity 2');
    console.log('  (if cond a b)  ; if uses parens since it is a special form');
    console.log('  (defn add (x y) (+ x y))\n');
    console.log('Stack operations (known arity, no parens):');
    console.log('  dup, swap, drop, over, rot, nip, tuck\n');
    console.log('Commands:');
    console.log('  /stack  - show data stack');
    console.log('  /env    - show environment');
    console.log('  /arity  - show arity table');
    console.log('  /clear  - clear stack');
    console.log('  /debug  - toggle debug mode');
    console.log('  /reset  - reset evaluator');
    console.log('  help    - show this help');
    console.log('  exit    - exit REPL');
  }
}
