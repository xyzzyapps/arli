# arli Language Specification

**arli** (Hy + Arity) is a stack-based Lisp dialect that eliminates parentheses through arity-driven parsing. It combines ideas from Forth (stack-based evaluation, concatenative operations) with Lisp (symbolic expressions, functional programming, Python interop).

## Architecture Overview

```
Source Code
    |
    v
[Tokenizer] --> Tokens
    |
    v
[Arity-Driven Parser] --> AST (nested Python lists/S-expressions)
    |
    v
[Stack-Based Evaluator] --> Result
    |       |
    |       v
    |   [Data Stack]  (Forth-like explicit stack)
    |
    v
[Environment]  (lexical scoping with closures)
```

### Component Diagram

```
+-------------------+     +-------------------+     +-------------------+
|   Tokenizer       |---->|     Parser        |---->|   Evaluator       |
| (tokenize.py)     |     | (parse.py)        |     | (eval.py)         |
|                   |     |                   |     |                   |
| Source -> Tokens  |     | Tokens -> AST     |     | AST -> Result     |
|                   |     | uses ArityTable   |     | uses Environment  |
+-------------------+     +-------------------+     +-------------------+
                                  |                         |
                                  v                         v
                          +-------------------+     +-------------------+
                          |   ArityTable      |     |   Environment     |
                          | (parse.py)        |     | (env.py)          |
                          |                   |     |                   |
                          | name -> arity     |     | name -> value     |
                          +-------------------+     +-------------------+
                                                          |
                                                          v
                                                  +-------------------+
                                                  |   Builtins        |
                                                  | (builtins.py)     |
                                                  |                   |
                                                  | + - * / cons ...  |
                                                  +-------------------+
```

## Data Types

### Core Types (defined in `types.py`)

| Type | Description | Example |
|------|-------------|---------|
| `int` | Python integer | `42` |
| `float` | Python float | `3.14` |
| `str` | Python string | `"hello"` |
| `Symbol` | Named symbol | `x`, `+`, `define`, `:keyword` |
| `NilType` (singleton `nil`) | Empty list / false | `nil` |
| `Builtin` | Python function with arity | `<Builtin + arity=2>` |
| `Function` | User-defined closure | `<fn add arity=2>` |
| `list` | Python list (S-expression) | `(1 2 3)` |
| `dict` | Python dict (via `hash-map`) | `{:a 1 :b 2}` |

### Keywords
Symbols starting with `:` are keywords — they self-evaluate. `:foo` evaluates to `:foo`.
Keywords are used as map keys and for named parameters.

### Vector/Map Literal Syntax
```
[1 2 3]           ;; -> (list 1 2 3) -> [1, 2, 3]
{:a 1 :b 2}       ;; -> (hash-map :a 1 :b 2) -> {':a': 1, ':b': 2}
```
Vectors and maps are syntactic sugar that expand to `list` and `hash-map` calls.

### Truthiness
Only `nil` is falsey. Everything else (including `0`, `""`, `[]`) is truthy.
## Arity-Driven Parsing

### Core Algorithm

The parser uses an `ArityTable` mapping symbol names to their argument count:

1. **Known arity (>= 0)**: When a symbol with registered arity N is encountered at the top level (or in argument position inside parentheses), it consumes the next N expressions as its arguments, forming an S-expression `[symbol, arg1, ..., argN]`.

2. **Unknown / variadic (-1)**: Symbols with arity -1 (like `if`, `do`, `let`) require parentheses. Inside parens, they are treated as regular symbols.

3. **Inside parentheses**: The FIRST element (operator position) does NOT use arity -- the parentheses themselves define the grouping. All SUBSEQUENT elements use arity-driven parsing.

### Parsing Rules (Position-dependent)

```
Context                     | allow_arity | Behavior
----------------------------|-------------|---------------------------------------
Top level                   | True        | arity-driven + special handlers
First element inside parens | False       | symbol only, no arity, no special handlers
Subsequent inside parens    | True        | arity-driven (but no defn/fn special)
defn-rec (always)           | N/A         | special handler regardless of position
```

### Special Form Parsers

- **`defn`**: `defn name (params) body...` -- at top level, registers arity immediately
- **`defn-rec`**: `defn-rec name (params) body...` -- registers arity BEFORE body (for recursion)
- **`fn`**: `fn (params) body...` -- creates anonymous function

### Parsing Examples

```
Expression                     | Parsed AST
-------------------------------|----------------------------------------------
+ 1 2                          | [+, 1, 2]
* + 1 2 3                      | [*, [+, 1, 2], 3]
print + 1 2                    | [print, [+, 1, 2]]
(if cond a b)                  | [if, cond, a, b]
(defn add (x y) (+ x y))       | [defn, add, [x, y], [+, x, y]]   (flat list)
define double (fn (x) (* x 2)) | [define, double, [fn, [x], [*, x, 2]]]
```

## Evaluation Model

### Stack-Based Evaluation

The evaluator maintains an explicit **data stack** (Forth-like). Every expression evaluation:
1. Computes a result value
2. Pushes the result onto the stack
3. Returns the result

### Evaluation Flow (lexical scoping)

```
Expression
    |
    v
Is it a literal?         --> return value, push to stack
Is it nil/true/false?    --> return constant
Is it a Symbol?          --> look up in environment (handle nil/true/false specially)
Is it a list (S-expr)?   --> check special forms first:
    |                        - quote, define, defn, if, do, fn, while, set!, let
    |                        If not special: evaluate head as function,
    v                        evaluate args, apply function
```

### Special Forms

| Form | Syntax | Behavior |
|------|--------|----------|
| `quote` | `quote expr` | Return expr unevaluated |
| `define` | `define name value` | Bind name to evaluated value; register arity if value is Function |
| `defn` | `(defn name (params) body...)` | Define function with known arity |
| `defn-rec` | `(defn-rec name (params) body...)` | Define recursive function |
| `if` | `(if cond then else)` | Conditional (short-circuit) |
| `do` | `(do expr...)` | Evaluate sequence, return last |
| `fn` | `(fn (params) body...)` | Create lambda |
| `while` | `(while cond body...)` | Loop |
| `set!` | `(set! name value)` | Mutate binding |
| `let` | `(let ((name val)...) body)` | Local bindings |
| `assert` | `(assert expr message)` | Raise if expr is falsy |
| `doc` | `(doc symbol)` / `(doc symbol "text")` | Retrieve/store docs |
| `match` | `(match val clause...)` | Pattern matching |
| `import-module` | `(import-module "path")` | Load .arli file |

### Function Application

**Builtins** receive arguments + optional `evaluator` keyword (for stack access):
```python
fn_val(*args, evaluator=self)
```

**User functions** create a new lexical scope, bind parameters, evaluate body:
```python
call_env = fn.env.extend(f"call({fn.name})")
for param, arg in zip(fn.params, args):
    call_env.define(param.name, arg)
result = eval_body(fn.body, call_env)
```

## Builtins

### Arithmetic (arity 2)
`+`, `-`, `*`, `/`, `//`, `%`, `neg` (arity 1)

### Comparison (arity 2)
`=`, `<`, `>`, `<=`, `>=`, `!=`

### Logic
`and` (variadic, -1), `or` (variadic, -1), `not` (arity 1)

### Stack Operations (Forth-like, known arity)
| Word | Arity | Description |
|------|-------|-------------|
| `dup` | 1 | Duplicate top of stack |
| `swap` | 2 | Swap top two items |
| `drop` | 1 | Discard top of stack |
| `over` | 2 | Copy second item to top |
| `rot` | 3 | Rotate top three items |
| `nip` | 2 | Drop second item |
| `tuck` | 2 | Duplicate top under second |

### List Operations
| Word | Arity | Description |
|------|-------|-------------|
| `cons` | 2 | Prepend item to list |
| `car` | 1 | First element of list |
| `cdr` | 1 | Rest of list (all but first) |
| `list` | -1 | Create list (variadic, requires parens) |
| `nil?` | 1 | Check if nil |
| `list?` | 1 | Check if list |
| `map` | 2 | Apply fn to each element: `(map fn list)` |
| `filter` | 2 | Keep elements where fn returns truthy |
| `reduce` | 3 | Accumulate: `(reduce fn init list)` |

### Data Structures
| Word | Arity | Description |
|------|-------|-------------|
| `hash-map` | -1 | Create dict from key-value pairs |
| `list` | -1 | Create list |

### I/O
`print` (arity 1), `.` (arity 1, like Forth `.`), `read` (arity 0)

### Type Checking
`number?`, `string?`, `symbol?`, `fn?` (all arity 1)

### Testing & Documentation
| Word | Arity | Description |
|------|-------|-------------|
| `assert` | 2 | `(assert expr message)` — raises if expr is falsy |
| `doc` | 2 | `(doc sym "text")` stores, `(doc sym)` retrieves |

### Pattern Matching
| Word | Arity | Description |
|------|-------|-------------|
| `match` | -1 | `(match val (pat result) (_ default))` — first matching clause wins |
## Evaluation & Parsing Interleaving

arli uses **interleaved parse-eval** execution. Each top-level expression is parsed and evaluated before the next expression is parsed. This ensures that arity registrations from `defn`/`define` take effect for immediately following expressions.

```
For source: (defn add (x y) (+ x y))
             add 1 2

Step 1: Parse "(defn add (x y) (+ x y))" -> [defn, add, [x, y], [+, x, y]]
Step 2: Evaluate -> creates function, registers arity "add" = 2
Step 3: Parse "add 1 2" -> add has arity 2 -> [add, 1, 2]
Step 4: Evaluate -> 3
```

## File Format

arli source files use the `.arli` extension. Files can contain multiple expressions separated by whitespace or newlines.

### Example: Fibonacci
```clojure
;; Recursive fibonacci
(defn-rec fib (n)
    (if (= n 0)
        0
        (if (= n 1)
            1
            + fib (- n 1) fib (- n 2))))

print fib 10  ;; prints 55
```

### Example: Stack Operations
```clojure
dup 5           ;; duplicate 5
swap 1 2        ;; swap (1 2) -> (2 1)
drop 42         ;; drop 42
over 1 2        ;; over: (1 2) -> (1 2 1)
```

## REPL Features

- Multi-line input (blank line terminates expression)
- Meta-commands:
  - `/stack` - inspect data stack
  - `/env` - view environment
  - `/arity` - view arity table
  - `/clear` - clear stack
  - `/debug` - toggle debug tracing
  - `/reset` - reset evaluator

## Future Directions

1. **Python Interop**: Compile arli to Python AST (like Hy)
2. **Macros**: Lisp-style macro system using arity-based syntax
3. **Tail Call Optimization**: For recursive functions
4. **Pattern Matching**: Destructuring on function parameters
5. **Module System**: Namespaced imports/exports
6. **Error Handling**: try/catch with stack traces
7. **Performance**: JIT compilation, type inference
