/**
 * Browser entry point for arli (JavaScript port).
 *
 * Exports everything needed to use arli in a browser environment.
 * Nothing in this file depends on Node.js APIs.
 *
 * Usage (browser via ES module):
 *   import { Evaluator, arliRepr } from './arli-browser.js';
 *   const ev = new Evaluator();
 *   ev.exec('+ 1 2');
 *   console.log(arliRepr(ev.stack.pop()));
 *
 * Browser limitations vs Node:
 *   - import/import! fire dynamic import() in the background, return a loading
 *     marker {__module__: name, __loading__: true}. The real module replaces
 *     it in the environment when the import resolves.
 *   - import-module fetches the .arli file via fetch() and executes it
 *     asynchronously; returns nil immediately (results not available yet).
 *   - read builtin returns nil (no stdin in browser).
 *   - . (print) falls back to console.log instead of process.stdout.write.
 */

export { ArliSymbol, nil, Builtin, Function, isTruthy, arliRepr, arliEqual } from './types.js';
export { Environment } from './env.js';
export { ArityTable, Parser, parseSource } from './parse.js';
export { tokenize, TokenStream, TOKEN_OPEN, TOKEN_CLOSE, TOKEN_VECTOR_OPEN, TOKEN_VECTOR_CLOSE, TOKEN_MAP_OPEN, TOKEN_MAP_CLOSE, TOKEN_STRING, TOKEN_NUMBER, TOKEN_SYMBOL, TOKEN_QUOTE, TOKEN_QUASIQUOTE, TOKEN_UNQUOTE, TOKEN_UNQUOTE_SPLICE, TOKEN_KEYWORD } from './tokenize.js';
export { Evaluator } from './eval.js';
export { getBuiltins } from './builtins.js';
