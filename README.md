# arli

**arli** (Hy + Arity) is a Forth-like Lisp dialect that eliminates parentheses through arity-driven parsing. If the number of arguments a function takes is known, you don't need parentheses. When arity is unknown, use parentheses as usual.

## Quick Example

```clojure
;; Traditional Lisp: (+ 1 (* 2 3))
;; arli (arity-driven, no parens):
+ 1 * 2 3         ;; => 7

;; Define a function
(defn add (x y) (+ x y))
add 1 2           ;; => 3  (arity-driven: add takes 2 args)

;; Recursive fibonacci
(defn-rec fib (n)
    (if (= n 0)
        0
        (if (= n 1)
            1
            + fib (- n 1) fib (- n 2))))

print fib 10      ;; prints 55
```

## Key Features

- **Arity-driven syntax**: Functions with known arity don't need parentheses
- **Stack-based evaluation**: Forth-like data stack with `dup`, `swap`, `drop`, `over`, `rot`
- **Lisp semantics**: S-expressions, lexical scoping, closures, first-class functions
- **Python interop**: Use any Python library via `import`, `.`, or `python`
- **Go backend**: Compiled Go binary with Go standard library access
- **Unicode symbols**: Greek, Cyrillic, Chinese, math symbols as function names
- **REPL**: Interactive with stack inspection, debug mode, multi-line input
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

## Syntax Guide

### No parentheses needed (known arity)
```clojure
+ 1 2              ;; arithmetic
* + 1 2 3          ;; nested: (* (+ 1 2) 3)
define x + 1 2     ;; variable definition
print x            ;; function call
dup 5              ;; stack operations
```

### Parentheses required (unknown/variadic arity)
```clojure
(if cond then else)
(defn name (params) body...)
(do expr1 expr2 ...)
(let ((x 1)) body)
(while cond body...)
(fn (x) (* x 2))
```

### Stack Operations
```clojure
dup 5       ;; -> [5, 5]
swap 1 2    ;; -> [2, 1]
drop 42     ;; -> []
over 1 2    ;; -> [1, 2, 1]
rot 1 2 3   ;; -> [2, 3, 1]
```

## Built-in Functions

**Arithmetic**: `+`, `-`, `*`, `/`, `//`, `%`, `neg`
**Comparison**: `=`, `<`, `>`, `<=`, `>=`, `!=`
**Logic**: `and`, `or`, `not`
**Stack**: `dup`, `swap`, `drop`, `over`, `rot`, `nip`, `tuck`
**Lists**: `cons`, `car`, `cdr`, `list`, `nil?`, `list?`
**I/O**: `print`, `.`, `read`
**Types**: `number?`, `string?`, `symbol?`, `fn?`

## Development

```bash
# Run tests
python -m pytest tests/ -v
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
