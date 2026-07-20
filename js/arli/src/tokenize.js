/**
 * Tokenizer for arli (JavaScript port).
 *
 * Converts source text into a flat list of tokens.
 * Tokens are: numbers, strings, symbols, and punctuation ()[]{} ' ` , ,@
 *
 * Each token is an object: { type, value }
 */

// Token types
export const TOKEN_OPEN = '(';
export const TOKEN_CLOSE = ')';
export const TOKEN_VECTOR_OPEN = '[';
export const TOKEN_VECTOR_CLOSE = ']';
export const TOKEN_MAP_OPEN = '{';
export const TOKEN_MAP_CLOSE = '}';
export const TOKEN_STRING = 'STRING';
export const TOKEN_NUMBER = 'NUMBER';
export const TOKEN_SYMBOL = 'SYMBOL';
export const TOKEN_QUOTE = "'";
export const TOKEN_QUASIQUOTE = '`';
export const TOKEN_UNQUOTE = ',';
export const TOKEN_UNQUOTE_SPLICE = ',@';
export const TOKEN_KEYWORD = 'KEYWORD';

/**
 * Check if a character can start a symbol name.
 */
function isSymbolStart(ch) {
  if (!ch) return false;
  if (/[a-zA-Z_$]/.test(ch)) return true;
  const code = ch.charCodeAt(0);
  if (code > 127) {
    const cat = getUnicodeCategory(code);
    if (cat && cat.startsWith('S')) return true;
    if (/[\u00C0-\u024F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u2C00-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]/.test(ch)) return true;
  }
  return '!$%&*./:<=>?@^_~'.includes(ch);
}

function getUnicodeCategory(code) {
  if ((code >= 0x2200 && code <= 0x22FF) ||
      (code >= 0x2100 && code <= 0x214F) ||
      (code >= 0x2190 && code <= 0x21FF) ||
      (code >= 0x2300 && code <= 0x23FF) ||
      (code >= 0x2400 && code <= 0x243F) ||
      (code >= 0x2440 && code <= 0x245F) ||
      (code >= 0x2460 && code <= 0x24FF) ||
      (code >= 0x2500 && code <= 0x257F) ||
      (code >= 0x2580 && code <= 0x259F) ||
      (code >= 0x25A0 && code <= 0x25FF) ||
      (code >= 0x2600 && code <= 0x26FF) ||
      (code >= 0x2700 && code <= 0x27BF) ||
      (code >= 0x27C0 && code <= 0x27EF) ||
      (code >= 0x27F0 && code <= 0x27FF) ||
      (code >= 0x2800 && code <= 0x28FF) ||
      (code >= 0x2900 && code <= 0x297F) ||
      (code >= 0x2980 && code <= 0x29FF) ||
      (code >= 0x2A00 && code <= 0x2AFF) ||
      (code >= 0x2B00 && code <= 0x2BFF)) {
    return 'Sm';
  }
  if ((code >= 0x00A2 && code <= 0x00A5) ||
      (code >= 0x20A0 && code <= 0x20CF)) {
    return 'Sc';
  }
  if ((code >= 0x02B0 && code <= 0x02FF) ||
      (code >= 0x1D00 && code <= 0x1D7F) ||
      (code >= 0x1D80 && code <= 0x1DBF)) {
    return 'Sk';
  }
  if (code >= 0x0370 && code <= 0x03FF) return 'L';
  if (code >= 0x0400 && code <= 0x04FF) return 'L';
  if (code >= 0x4E00 && code <= 0x9FFF) return 'L';
  if (code >= 0x3400 && code <= 0x4DBF) return 'L';
  if (code >= 0xF900 && code <= 0xFAFF) return 'L';
  if (code >= 0x3040 && code <= 0x309F) return 'L';
  if (code >= 0x30A0 && code <= 0x30FF) return 'L';
  return null;
}

/**
 * Tokenize source text into an array of tokens.
 */
export function tokenize(source) {
  const tokens = [];
  const len = source.length;
  let i = 0;

  while (i < len) {
    const ch = source[i];

    if (/[ \t\n\r\f]/.test(ch)) { i++; continue; }

    if (ch === ';') {
      while (i < len && source[i] !== '\n' && source[i] !== '\r') i++;
      continue;
    }

    if (ch === '(') { tokens.push({ type: TOKEN_OPEN, value: '(' }); i++; continue; }
    if (ch === ')') { tokens.push({ type: TOKEN_CLOSE, value: ')' }); i++; continue; }
    if (ch === '[') { tokens.push({ type: TOKEN_VECTOR_OPEN, value: '[' }); i++; continue; }
    if (ch === ']') { tokens.push({ type: TOKEN_VECTOR_CLOSE, value: ']' }); i++; continue; }
    if (ch === '{') { tokens.push({ type: TOKEN_MAP_OPEN, value: '{' }); i++; continue; }
    if (ch === '}') { tokens.push({ type: TOKEN_MAP_CLOSE, value: '}' }); i++; continue; }

    if (ch === "'") { tokens.push({ type: TOKEN_QUOTE, value: "'" }); i++; continue; }
    if (ch === '`') { tokens.push({ type: TOKEN_QUASIQUOTE, value: '`' }); i++; continue; }
    if (ch === ',') {
      if (i + 1 < len && source[i + 1] === '@') {
        tokens.push({ type: TOKEN_UNQUOTE_SPLICE, value: ',@' }); i += 2;
      } else {
        tokens.push({ type: TOKEN_UNQUOTE, value: ',' }); i++;
      }
      continue;
    }

    if (ch === '"') {
      if (i + 2 < len && source[i + 1] === '"' && source[i + 2] === '"') {
        i += 3;
        let s = '';
        while (i < len) {
          if (i + 2 < len && source[i] === '"' && source[i + 1] === '"' && source[i + 2] === '"') {
            i += 3; break;
          }
          if (source[i] === '\\' && i + 1 < len) {
            const esc = source[i + 1];
            if (esc === 'n') { s += '\n'; i += 2; }
            else if (esc === 't') { s += '\t'; i += 2; }
            else if (esc === 'r') { s += '\r'; i += 2; }
            else if (esc === '"') { s += '"'; i += 2; }
            else if (esc === '\\') { s += '\\'; i += 2; }
            else { s += source[i]; i++; }
          } else { s += source[i]; i++; }
        }
        tokens.push({ type: TOKEN_STRING, value: s });
      } else {
        i++;
        let s = '';
        while (i < len) {
          const c = source[i];
          if (c === '"') { i++; break; }
          if (c === '\\' && i + 1 < len) {
            const esc = source[i + 1];
            if (esc === 'n') { s += '\n'; i += 2; }
            else if (esc === 't') { s += '\t'; i += 2; }
            else if (esc === 'r') { s += '\r'; i += 2; }
            else if (esc === '"') { s += '"'; i += 2; }
            else if (esc === '\\') { s += '\\'; i += 2; }
            else { s += c; i++; }
          } else { s += c; i++; }
        }
        tokens.push({ type: TOKEN_STRING, value: s });
      }
      continue;
    }

    if (ch === ':') {
      const start = i; i++;
      if (i < len && (source[i].toLowerCase() !== source[i].toUpperCase() || '_!$%&*./<=>?@^~'.includes(source[i]))) {
        while (i < len && !/[ \t\n\r()\[\]{}"'`;,]/.test(source[i])) i++;
        tokens.push({ type: TOKEN_KEYWORD, value: source.slice(start, i) });
      } else {
        tokens.push({ type: TOKEN_SYMBOL, value: ':' });
      }
      continue;
    }

    const start = i;
    if (ch === '-') { if (i + 1 < len && /[0-9.]/.test(source[i + 1])) i++; }
    else if (ch === '+') { if (i + 1 < len && /[0-9.]/.test(source[i + 1])) i++; }
    else if (/[0-9]/.test(ch)) { /* number */ }
    else if (isSymbolStart(ch)) { /* symbol */ }
    else { i++; continue; }

    while (i < len) {
      const c = source[i];
      if (/[ \t\n\r()\[\]{}"'`;,]/.test(c)) break;
      i++;
    }
    const tokenStr = source.slice(start, i);

    if (tokenStr === '-' || tokenStr === '+') {
      tokens.push({ type: TOKEN_SYMBOL, value: tokenStr });
    } else {
      const num = parseNumber(tokenStr);
      if (num !== null) {
        tokens.push({ type: TOKEN_NUMBER, value: num });
      } else {
        tokens.push({ type: TOKEN_SYMBOL, value: tokenStr });
      }
    }
  }

  return tokens;
}

function parseNumber(s) {
  if (/^0x/i.test(s)) { const n = parseInt(s.slice(2), 16); if (!isNaN(n)) return n; return null; }
  if (/^0o/i.test(s)) { const n = parseInt(s.slice(2), 8); if (!isNaN(n)) return n; return null; }
  if (/^0b/i.test(s)) { const n = parseInt(s.slice(2), 2); if (!isNaN(n)) return n; return null; }
  const n = Number(s);
  if (!isNaN(n) && s.trim() !== '') return n;
  return null;
}

// ---------------------------------------------------------------------------
// TokenStream
// ---------------------------------------------------------------------------

export class TokenStream {
  constructor(tokens) {
    this.tokens = tokens;
    this.pos = 0;
  }

  get isEOF() { return this.pos >= this.tokens.length; }

  peek() {
    if (this.isEOF) return null;
    return this.tokens[this.pos];
  }

  next() {
    if (this.isEOF) return null;
    const tok = this.tokens[this.pos];
    this.pos++;
    return tok;
  }

  expect(expectedType) {
    const tok = this.next();
    if (tok === null) throw new SyntaxError(`Expected ${expectedType}, got end of input`);
    if (tok.type !== expectedType) throw new SyntaxError(`Expected ${expectedType}, got ${tok.type} (${tok.value})`);
    return tok;
  }
}
