/**
 * Arity-driven parser for arli (JavaScript port).
 */

import { ArliSymbol, nil } from './types.js';
import {
  TokenStream, tokenize, TOKEN_OPEN, TOKEN_CLOSE,
  TOKEN_VECTOR_OPEN, TOKEN_VECTOR_CLOSE,
  TOKEN_MAP_OPEN, TOKEN_MAP_CLOSE,
  TOKEN_NUMBER, TOKEN_STRING, TOKEN_SYMBOL,
  TOKEN_QUOTE, TOKEN_QUASIQUOTE,
  TOKEN_UNQUOTE, TOKEN_UNQUOTE_SPLICE,
  TOKEN_KEYWORD
} from './tokenize.js';

export class ArityTable {
  constructor() { this._table = {}; }
  register(name, arity) { this._table[name] = arity; }
  get(name) { const val = this._table[name]; return val !== undefined ? val : null; }
  has(name) { return Object.prototype.hasOwnProperty.call(this._table, name); }
  clone() { const t = new ArityTable(); t._table = { ...this._table }; return t; }
}

export class Parser {
  constructor(arityTable) { this.arityTable = arityTable; }

  parse(source) {
    const tokens = tokenize(source);
    const stream = new TokenStream(tokens);
    const exprs = [];
    while (!stream.isEOF) {
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) exprs.push(expr);
    }
    return exprs;
  }

  _parseExpr(stream, allowArity = true) {
    const tok = stream.peek();
    if (tok === null) return null;
    const tt = tok.type;

    if (tt === TOKEN_OPEN) return this._parseParenList(stream);
    if (tt === TOKEN_VECTOR_OPEN) return this._parseBracketList(stream);
    if (tt === TOKEN_MAP_OPEN) return this._parseMapLiteral(stream);
    if (tt === TOKEN_QUOTE) { stream.next(); const expr = this._parseExpr(stream, false); return [new ArliSymbol('quote'), expr]; }
    if (tt === TOKEN_QUASIQUOTE) { stream.next(); const expr = this._parseExpr(stream, true); return [new ArliSymbol('quasiquote'), expr]; }
    if (tt === TOKEN_UNQUOTE) { stream.next(); const expr = this._parseExpr(stream, true); return [new ArliSymbol('unquote'), expr]; }
    if (tt === TOKEN_UNQUOTE_SPLICE) { stream.next(); const expr = this._parseExpr(stream, true); return [new ArliSymbol('unquote-splicing'), expr]; }

    stream.next();
    if (tt === TOKEN_NUMBER) return tok.value;
    if (tt === TOKEN_STRING) return tok.value;
    if (tt === TOKEN_KEYWORD) return new ArliSymbol(tok.value);
    if (tt === TOKEN_SYMBOL) {
      const name = tok.value;
      if (name === 'defn-rec') return this._parseDefnRec(stream);
      if (!allowArity) return new ArliSymbol(name);
      const arity = this.arityTable.get(name);
      if (arity !== null && arity >= 0) {
        const args = [];
        for (let i = 0; i < arity; i++) {
          const arg = this._parseExpr(stream, true);
          if (arg === null || arg === undefined) throw new SyntaxError(`Expected ${arity} args for ${name}, got ${args.length}`);
          args.push(arg);
        }
        return [new ArliSymbol(name)].concat(args);
      }
      return new ArliSymbol(name);
    }
    throw new SyntaxError(`Unexpected token: ${tok.type} (${tok.value})`);
  }

  _parseParenList(stream) {
    stream.expect(TOKEN_OPEN);
    const items = [];
    let isFirst = true;
    let quotedForm = false;
    while (true) {
      const tok = stream.peek();
      if (tok === null) throw new SyntaxError('Unclosed parenthesis');
      if (tok.type === TOKEN_CLOSE) { stream.next(); break; }
      const allowArity = !isFirst && !quotedForm;
      const expr = this._parseExpr(stream, allowArity);
      if (expr !== null && expr !== undefined) {
        if (isFirst && expr instanceof ArliSymbol && expr.name === 'quote') quotedForm = true;
        if (isFirst && Array.isArray(expr) && expr.length > 0 && expr[0] instanceof ArliSymbol && expr[0].name === 'defn' && stream.peek() !== null && stream.peek().type === TOKEN_CLOSE) { stream.next(); return expr; }
        items.push(expr);
      }
      isFirst = false;
    }
    return items;
  }

  _parseBracketList(stream) {
    stream.expect(TOKEN_VECTOR_OPEN);
    const items = [];
    while (true) {
      const tok = stream.peek();
      if (tok === null) throw new SyntaxError('Unclosed bracket');
      if (tok.type === TOKEN_VECTOR_CLOSE) { stream.next(); break; }
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) items.push(expr);
    }
    return [new ArliSymbol('list')].concat(items);
  }

  _parseMapLiteral(stream) {
    stream.expect(TOKEN_MAP_OPEN);
    const items = [];
    while (true) {
      const tok = stream.peek();
      if (tok === null) throw new SyntaxError('Unclosed map brace');
      if (tok.type === TOKEN_MAP_CLOSE) { stream.next(); break; }
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) items.push(expr);
    }
    return [new ArliSymbol('hash-map')].concat(items);
  }

  _parseNameSym(stream) {
    const tok = stream.peek();
    if (tok === null || tok.type !== TOKEN_SYMBOL) throw new SyntaxError('Expected a symbol name');
    return new ArliSymbol(stream.next().value);
  }

  _parseParamsList(stream) {
    const params = this._parseExpr(stream, false);
    if (!Array.isArray(params)) throw new SyntaxError('Expected a parameter list in parens');
    const paramSyms = [];
    for (const p of params) {
      if (p instanceof ArliSymbol) paramSyms.push(p);
      else throw new SyntaxError(`Expected symbol in param list, got ${p}`);
    }
    return paramSyms;
  }

  _parseDefn(stream) {
    const nameSym = this._parseNameSym(stream);
    const paramSyms = this._parseParamsList(stream);
    this.arityTable.register(nameSym.name, paramSyms.length);
    const body = [];
    while (true) {
      const tok = stream.peek();
      if (tok === null || tok.type === TOKEN_CLOSE || tok.type === TOKEN_VECTOR_CLOSE || tok.type === TOKEN_MAP_CLOSE) break;
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) body.push(expr);
    }
    return [new ArliSymbol('defn'), nameSym, paramSyms].concat(body.length > 0 ? body : [nil]);
  }

  _parseFn(stream) {
    const paramSyms = this._parseParamsList(stream);
    const body = [];
    while (true) {
      const tok = stream.peek();
      if (tok === null || tok.type === TOKEN_CLOSE || tok.type === TOKEN_VECTOR_CLOSE || tok.type === TOKEN_MAP_CLOSE) break;
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) body.push(expr);
    }
    return [new ArliSymbol('fn'), paramSyms].concat(body.length > 0 ? body : [nil]);
  }

  _parseDefnRec(stream) {
    const nameSym = this._parseNameSym(stream);
    const paramSyms = this._parseParamsList(stream);
    this.arityTable.register(nameSym.name, paramSyms.length);
    const body = [];
    const tok = stream.peek();
    if (tok !== null && tok.type !== TOKEN_CLOSE && tok.type !== TOKEN_VECTOR_CLOSE && tok.type !== TOKEN_MAP_CLOSE) {
      const expr = this._parseExpr(stream, true);
      if (expr !== null && expr !== undefined) body.push(expr);
    }
    return [new ArliSymbol('defn'), nameSym, paramSyms].concat(body.length > 0 ? body : [nil]);
  }
}

export function parseSource(source, arityTable = null) {
  if (arityTable === null) arityTable = new ArityTable();
  return new Parser(arityTable).parse(source);
}
