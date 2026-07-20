/**
 * Environment for arli (JavaScript port) — maps symbols to values.
 */

export class Environment {
  constructor(parent = null, name = 'global') {
    this.parent = parent;
    this.name = name;
    this._bindings = {};
  }

  define(name, value) {
    this._bindings[name] = value;
    return value;
  }

  lookup(name) {
    if (Object.prototype.hasOwnProperty.call(this._bindings, name)) return this._bindings[name];
    if (this.parent !== null) return this.parent.lookup(name);
    return null;
  }

  get(name) {
    const val = this.lookup(name);
    if (val === null || val === undefined) throw new ReferenceError(`Undefined symbol: ${name}`);
    return val;
  }

  set(name, value) {
    if (Object.prototype.hasOwnProperty.call(this._bindings, name)) { this._bindings[name] = value; return value; }
    if (this.parent !== null) return this.parent.set(name, value);
    throw new ReferenceError(`Cannot set! undefined symbol: ${name}`);
  }

  has(name) {
    if (Object.prototype.hasOwnProperty.call(this._bindings, name)) return true;
    if (this.parent !== null) return this.parent.has(name);
    return false;
  }

  extend(name = 'block') { return new Environment(this, name); }

  toString() { return `<Env ${this.name}: ${JSON.stringify(Object.keys(this._bindings))}>`; }
}
