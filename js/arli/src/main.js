#!/usr/bin/env node

/**
 * CLI entry point for arli (JavaScript port).
 */

import { Evaluator } from './eval.js';
import { REPL } from './repl.js';
import { arliRepr } from './types.js';

function main() {
  const args = process.argv.slice(2);
  let debug = false;
  let evalExpr = null;
  const files = [];

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '-d' || args[i] === '--debug') debug = true;
    else if (args[i] === '-e' || args[i] === '--eval') { i++; if (i < args.length) evalExpr = args[i]; }
    else if (args[i].startsWith('-')) { console.error(`Unknown option: ${args[i]}`); process.exit(1); }
    else files.push(args[i]);
  }

  if (evalExpr !== null) {
    const ev = new Evaluator(debug);
    console.log(arliRepr(ev.exec(evalExpr)));
    return;
  }

  if (files.length > 0) {
    const ev = new Evaluator(debug);
    for (const filepath of files) {
      try { ev.execFile(filepath); }
      catch (e) { console.error(`Error in ${filepath}: ${e.message}`); if (debug) console.error(e.stack); process.exit(1); }
    }
    return;
  }

  const repl = new REPL(null, debug);
  repl.run();
}

main();
