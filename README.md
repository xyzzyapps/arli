# arli

**arli** (Arity-driven Lisp) is a Forth-like Lisp dialect that eliminates parentheses through arity-driven parsing. If the number of arguments a function takes is known, you don't need parentheses. When arity is unknown (variadic), use parentheses as usual.

## Quick Example

```clojure
;; Traditional Lisp: (+ 1 (* 2 3))
;; arli (arity-driven, no parens):
+ 1 * 2 3         ;; => 7

;; Define a function â€” no parens needed (defn=3, fn=2)
defn add (x y) + x y
add 1 2           ;; => 3  (add has arity 2)

;; Recursive fibonacci (defn-rec requires parens)
(defn-rec fib (n)
    if (= n 0)
        0
        if (= n 1)
            1
            + fib (- n 1) fib (- n 2))

print fib 10      ;; prints 55
```

## Key Features

- **Arity-driven syntax**: Functions with known arity don't need parentheses
- **Stack-based evaluation**: Forth-like data stack with `dup`, `swap`, `drop`, `over`, `rot`, `pick`, `roll`
- **Stack reflection**: Capture and replace the data stack with `stack`/`stack!` for metaprogramming
- **Exec stack (Push-style)**: Self-modifying code via `exec-stack`, `exec!`, `exec-push`, `(exec)` â€” the exec stack IS the call stack
- **Lisp semantics**: S-expressions, lexical scoping, closures, first-class functions
- **Vector/Map literals**: `[1 2 3]` and `{:key val}` syntax
- **Keywords**: Self-evaluating `:keyword` symbols
- **Hex/octal/bin literals**: `0xFF`, `0o77`, `0b1010`
- **Triple-quoted strings**: `"""multi-line"""` strings
- **Result type**: `Ok`/`Err` with `map-ok`, `and-then`, `or-else`
- **Pattern matching**: `(match val (pat result) (_ default))`
- **Sequence operations**: `map`, `filter`, `reduce` builtins
- **Docstrings**: Documentation system via `(doc symbol "text")`
- **Testing**: `(assert expr message)` for inline tests
- **Module system**: `(import-module "path.arli")` for loading files
- **Python interop**: Use any Python library via `import`, `import!`, `.`, or `host`
- **Go backend**: Compiled Go binary with Go standard library access
- **Unicode symbols**: Greek, Cyrillic, Chinese, math symbols as function names
- **REPL**: Interactive with stack inspection, debug mode, multi-line input
- **F-expressions**: User-defined functions that don't evaluate arguments eagerly; custom control flow via `defn-fexpr` and `eval`

## Installation

```bash
git clone <repo-url>
cd arli
pip install -e .
```

## Usage

### REPL
```bash
python -m arli
```

### Run a file
```bash
python -m arli examples/fizzbuzz.arli
```

### Evaluate an expression
```bash
python -m arli -e "+ 1 2"
```

### JavaScript Backend
```bash
cd js/arli
node src/main.js              # REPL
node src/main.js file.arli    # Run file
node src/main.js -e "+ 1 2"   # Evaluate expression
node tests/run_tests.js       # Run test suite
```

### Browser (ES Module)

```html
<script type="module">
  import { Evaluator, arliRepr } from './js/arli/src/arli-browser.js';
  const ev = new Evaluator();
  ev.exec('+ 1 2');
  console.log(arliRepr(ev.stack.pop())); // 3

  // Access browser APIs via host special form
  ev.exec('host "document.title"');       // returns the page title
  ev.exec('. document querySelector "h1"');

  // Import ESM modules via dynamic import()
  ev.exec('import lodash');  // fire-and-forget, module arrives when resolved
  ev.exec('import! "https://cdn.skypack.dev/lodash" _');
</script>
```

## Syntax Guide
### No parentheses needed (all have known arity)
```clojure
+ 1 2              ;; arithmetic
* + 1 2 3          ;; nested: (* (+ 1 2) 3)
define x + 1 2     ;; variable definition
if cond "yes" "no" ;; conditional (if=3)
while cond body    ;; loop (while=2)
set! x 42          ;; mutation (set!=2)
for i list body    ;; iteration (for=3)
let ((x 1)) body   ;; local bindings (let=2)
dup 5              ;; stack operations
print x            ;; function call
. os sep           ;; attribute access (.=2)
import os          ;; module import (import=1)
quote + 1 2        ;; quote (quote=1)
cond clauses       ;; multi-branch (cond=1)
fn (x) * x 2       ;; anonymous function (fn=2)
defn add (x y) + x y  ;; function definition (defn=3)
```

### Parentheses required (unknown/variadic arity)
```clojure
(do expr1 expr2 ...)              ;; sequencing
(list 1 2 3)                      ;; list creation
(match val (pat result) (_ dflt)) ;; pattern matching
(and a b c)                       ;; short-circuit AND
(or a b c)                        ;; short-circuit OR
(hash-map :a 1 :b 2)              ;; map creation
```

### Stack Operations
```clojure
dup 5       ;; -> [5, 5]
swap 1 2    ;; -> [2, 1]
drop 42     ;; -> []
over 1 2    ;; -> [1, 2, 1]
rot 1 2 3   ;; -> [2, 3, 1]
nip 1 2     ;; -> [2]
tuck 1 2    ;; -> [2, 1, 2]
pick 0 42 1  ;; -> copies 42 (index 0=top): stack [42, 1, 42]
pick 1 42 1  ;; -> copies 1 (index 1):   stack [42, 1, 1]
roll 1 42 1  ;; -> swaps:                 stack [1, 42]
```

### Stack Reflection

Capture, inspect, and replace the data stack:

```clojure
stack              ;; -> ()       â€” push a copy of the current stack as a list
stack! (list 1 2 3) ;; -> replaces entire stack with [1, 2, 3]
stack! nil         ;; -> clears the stack
```

### Exec Stack (Push-style Self-Modifying Code)

The exec stack holds pending code. Manipulate it from within executing code:

```clojure
exec-push (quote +)   ;; push a function symbol onto exec stack
exec-push 2           ;; push arguments (become data via (exec))
exec-push 1
(exec)                ;; process exec stack: 1->data, 2->data, + pops 2->3

exec-stack            ;; push a copy of the exec stack to data stack
exec! (list ...)      ;; replace the entire exec stack
exec-pop              ;; pop top of exec stack to data stack
exec-depth            ;; push exec stack depth
exec-step             ;; pop and evaluate one form from exec stack
```

### F-Expressions (Custom Control Flow)

Define functions that **don't evaluate their arguments eagerly** â€” use `defn-fexpr` (arity 3) and `eval` (arity 1):

```clojure
;; Custom if â€” no parens needed (arity 3)
defn-fexpr my-if (c t e)
    if (eval c) (eval t) (eval e)

my-if true "yes" "no"      ;; -> "yes"
my-if false "yes" "no"     ;; -> "no"

;; Short-circuit OR
defn-fexpr short-or (a b)
    let ((av (eval a)))
        if av av (eval b)

short-or true (print "never runs")     ;; -> True, no side effect
short-or false (print "runs")           ;; prints "runs", returns nil

;; Inspect raw AST (unevaluated arguments)
defn-fexpr show (x) x
show + 1 * 2 3          ;; returns raw AST, not 7
```

`defn-fexpr` works like `defn` (arity 3): `defn-fexpr name (params) body`.  
`eval` evaluates a raw form in the current environment. Bodies with multiple expressions use `(do ...)`.

**Caveat**: Parameter names must not match registered operator names (e.g., avoid `cond`, `list`, `not` as parameter names â€” they have arity and consume extra tokens).

## Complete Arity Table

Every operator has a documented arity. Arity >= 0 = no parens needed. Arity -1 = variadic (parens required).

### Special Forms

| Form | Arity | Example |
|------|-------|---------|
| `define` | 2 | `define x 42` |
| `defn` | 3 | `defn add (x y) + x y` |
| `defn-rec` | 3 | `defn-rec fact (n) (if ...)` |
| `fn` | 2 | `fn (x) * x 2` |
| `if` | 3 | `if cond "yes" "no"` |
| `while` | 2 | `while (< i 5) (do ...)` |
| `for` | 3 | `for x list print x` |
| `let` | 2 | `let ((x 1)) + x 2` |
| `set!` | 2+ | `set! x 42` / `(set! m :key val)` |
| `cond` | 1 | `cond ((> x 0) "pos" true "other")` |
| `quote` | 1 | `quote + 1 2` |
| `do` | -1 | `(do a b c)` |
| `import` | 1 | `import os` |
| `import!` | 2 | `import! os myos` |
| `.` | 2+ | `. os sep` / `(. os path join "a" "b")` |
| `host` | 1 | `host "repr(42)"` |
| `assert` | 2 | `(assert expr message)` |
| `doc` | 1 | `doc add` | Retrieve documentation |
| `doc!` | 2 | `doc! add "text"` | Store documentation |
| `match` | -1 | `(match val (1 "one") (_ "other"))` |
| `import-module` | 1 | `import-module "lib.arli"` |
| `defn-fexpr` | 3 | `defn-fexpr my-if (c t e) if (eval c) (eval t) (eval e)` |
| `eval` | 1 | `eval expr` |

### Builtins

| Category | Words (arity) |
|----------|---------------|
| Arithmetic | `+`(2) `-`(2) `*`(2) `/`(2) `//`(2) `%`(2) `neg`(1) |
| Comparison | `=`(2) `<`(2) `>`(2) `<=`(2) `>=`(2) `!=`(2) |
| Logic | `and`(-1) `or`(-1) `not`(1) |
| Stack | `dup`(1) `swap`(2) `drop`(1) `over`(2) `rot`(3) `nip`(2) `tuck`(2) `pick`(1) `roll`(1) |
| Reflection | `stack`(0) `stack!`(1) |
| Exec Stack | `exec-stack`(0) `exec!`(1) `exec-push`(1) `exec-pop`(0) `exec-depth`(0) `exec-step`(0) `(exec)`(-1) |
| Lists | `cons`(2) `car`(1) `cdr`(1) `list`(-1) `nil?`(1) `list?`(1) |
| Sequences | `map`(2) `filter`(2) `reduce`(3) |
| Data | `hash-map`(-1) |
| I/O | `print`(1) `.`(2) `read`(0) |
| Types | `number?`(1) `string?`(1) `symbol?`(1) `fn?`(1) |
| Result | `Ok`(1) `Err`(1) `map-ok`(2) `and-then`(2) `or-else`(2) |

## Development

```bash
# Run tests
python -m pytest tests/ -v

# Run shared test suite (Python backend)
python tests/run_tests.py --python

# Run shared test suite (Go backend)
python tests/run_tests.py --go
```

## How It Works

1. **Tokenizer** converts source text to tokens
2. **Parser** uses arity to build S-expression trees from flat token streams
3. **Evaluator** walks the tree using a stack-based evaluation model
4. **Environment** manages lexical scoping with closures

The parser and evaluator are **interleaved**: each expression is parsed and evaluated before the next, so function arity definitions take effect immediately.

## Architecture

```
Source -> Tokenizer -> Parser (arity-driven) -> Evaluator (stack-based) -> Result
                         |                              |
                     ArityTable                    Environment
                         |                              |
                     Builtins                      Data Stack
```

See [SPEC.md](SPEC.md) for full technical specification.

## License

MIT
 
## Signature

Original Research by Xyzzy, built with assistance from **Deepseek 4 Pro**.   
Specification target: the implementation described in [SPEC.md](SPEC.md).
