# arli Language Specification

**arli** (Arity-driven Lisp) is a stack-based Lisp dialect that eliminates parentheses through arity-driven parsing. It combines ideas from Forth (stack-based evaluation, concatenative operations) with Lisp (symbolic expressions, functional programming, Python interop).

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

2. **Unknown / variadic (-1)**: Symbols with arity -1 (like `do`, `list`, `match`, `and`, `or`, `hash-map`) require parentheses. Inside parens, they are treated as regular symbols.

3. **Inside parentheses**: The FIRST element (operator position) does NOT use arity — the parentheses themselves define the grouping. All SUBSEQUENT elements use arity-driven parsing.

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

- **`defn`**: `defn name (params) body...` — at top level, uses arity 3. Inside parens, the `defn` symbol is followed by params and body.
- **`defn-rec`**: `(defn-rec name (params) body...)` — special handler that registers arity BEFORE parsing body (for recursion). Must use parens.
- **`fn`**: `fn (params) body...` — at top level, uses arity 2. Creates anonymous function.

### Parsing Examples

```
Expression                     | Parsed AST
-------------------------------|----------------------------------------------
+ 1 2                          | [+, 1, 2]
* + 1 2 3                      | [*, [+, 1, 2], 3]
print + 1 2                    | [print, [+, 1, 2]]
if cond "yes" "no"             | [if, cond, "yes", "no"]
(if cond a b)                  | [if, cond, a, b]
(defn add (x y) (+ x y))       | [defn, add, [x, y], [+, x, y]]
define double (fn (x) (* x 2)) | [define, double, [fn, [x], [*, x, 2]]]
```

### Quote Shorthands (Tokenized, Evaluated)

| Shorthand | Token | Expands to | Status |
|-----------|-------|------------|--------|
| `'expr` | `TOKEN_QUOTE` | `(quote expr)` | Implemented |
| `` `expr `` | `TOKEN_QUASIQUOTE` | `(quasiquote expr)` | Tokenized only — evaluation not yet implemented |
| `,expr` | `TOKEN_UNQUOTE` | `(unquote expr)` | Tokenized only — evaluation not yet implemented |
| `,@expr` | `TOKEN_UNQUOTE_SPLICE` | `(unquote-splicing expr)` | Tokenized only — evaluation not yet implemented |

## Complete Arity Table

Every operator and special form in arli has a documented arity. **All operators with arity >= 0 can be used WITHOUT parentheses.** Only variadic operators (arity -1) require parentheses.

### Special Forms (arity >= 0)

| Form | Arity | Syntax | Description |
|------|-------|--------|-------------|
| `define` | 2 | `define name value` | Bind name to evaluated value; auto-registers arity if value is a function |
| `defn` | 3 | `defn name (params) body` | Define function with auto arity registration |
| `defn-rec` | -1 | `(defn-rec name (params) body)` | Define recursive function (requires parens; registers arity before body) |
| `fn` | 2 | `fn (params) body` | Create anonymous function |
| `if` | 3 | `if cond then else` | Conditional with short-circuit evaluation |
| `while` | 2 | `while cond body` | Loop while condition is truthy |
| `for` | 3 | `for var list body` | Iterate over list elements |
| `let` | 2 | `let bindings body` | Local lexical bindings |
| `set!` | 2 | `set! name value` | Mutate variable binding |
| `set!` (nested) | 3 | `set! obj key value` | Mutate dict/list element: `(set! m :key val)` or `(set! lst idx val)` |
| `cond` | 1 | `cond clauses` | Multi-branch conditional (clauses is a flat list of test result pairs) |
| `quote` | 1 | `quote expr` | Return expression unevaluated |
| `do` | -1 | `(do expr...)` | Evaluate sequence, return last (variadic — requires parens) |
| `import` | 1 | `import module` | Import Python module by name |
| `import!` | 2 | `import! module alias` | Import Python module with alias |
| `.` | 2 | `. obj attr` | Attribute access / method call (arity 2 for top-level attribute access; chains inside parens) |
| `host` | 1 | `host "code"` | Evaluate arbitrary host language expression string |
| `assert` | 2 | `assert expr message` | Raise AssertionError if expr is falsy |
| `doc` | -1 | `(doc sym)` / `(doc sym "text")` | Retrieve or store documentation (variadic — requires parens) |
| `match` | -1 | `(match val clause...)` | Pattern matching (variadic — requires parens) |
| `import-module` | 1 | `import-module "path"` | Load and execute an `.arli` file |
| `defn-fexpr` | 3 | `defn-fexpr name (params) body` | Define f-expression (creates function that doesn't evaluate its arguments) |
| `eval` | 1 | `eval form` | Evaluate a form (expression) in the current environment |

### Builtin Functions

All builtins have known arities (>= 0) except where noted. All can be used without parentheses.

#### Arithmetic

| Word | Arity | Description |
|------|-------|-------------|
| `+` | 2 | Addition |
| `-` | 2 | Subtraction |
| `*` | 2 | Multiplication |
| `/` | 2 | Division |
| `//` | 2 | Floor division |
| `%` | 2 | Modulo |
| `neg` | 1 | Negation |

#### Comparison

| Word | Arity | Description |
|------|-------|-------------|
| `=` | 2 | Equality |
| `<` | 2 | Less than |
| `>` | 2 | Greater than |
| `<=` | 2 | Less than or equal |
| `>=` | 2 | Greater than or equal |
| `!=` | 2 | Not equal |

#### Logic

| Word | Arity | Description |
|------|-------|-------------|
| `and` | -1 | Short-circuit AND (variadic — requires parens) |
| `or` | -1 | Short-circuit OR (variadic — requires parens) |
| `not` | 1 | Logical NOT |

#### Stack Operations (Forth-like)

| Word | Arity | Effect | Description |
|------|-------|--------|-------------|
| `dup` | 1 | `a -> a a` | Duplicate top of stack |
| `swap` | 2 | `a b -> b a` | Swap top two items |
| `drop` | 1 | `a ->` | Discard top of stack |
| `over` | 2 | `a b -> a b a` | Copy second item to top |
| `rot` | 3 | `a b c -> b c a` | Rotate top three items |
| `nip` | 2 | `a b -> b` | Drop second item |
| `tuck` | 2 | `a b -> b a b` | Duplicate top under second |
| `pick` | 1 | `n -> val` | Copy nth element (0=top) to top |
| `roll` | 1 | `n -> val` | Rotate nth element (0=top) to top |

#### Stack Reflection

| Word | Arity | Description |
|------|-------|-------------|
| `stack` | 0 | Push a copy of the data stack as a list |
| `stack!` | 1 | Replace data stack with a list (returns None to skip push) |

#### Exec Stack Operations (Push-style)

The exec stack holds pending code forms for self-modifying programs. It is the call stack — every function call pushes its body and a `__restore_env__` sentinel onto it.

| Word | Arity | Description |
|------|-------|-------------|
| `exec-stack` | 0 | Push a copy of the exec stack to the data stack |
| `exec!` | 1 | Replace exec stack with a list (returns None) |
| `exec-push` | 1 | Push a form onto the exec stack (returns None) |
| `exec-pop` | 0 | Pop top of exec stack to data stack |
| `exec-depth` | 0 | Push exec stack depth to data stack |
| `exec-step` | 0 | Pop and evaluate one form from exec stack |
| `(exec)` | -1 | Process entire exec stack until empty (variadic, parens required) |

The `(exec)` Push interpreter processes items from the exec stack:
- **Literals**: pushed to the data stack
- **Symbols**: looked up in the environment and called. Arguments are popped from the **data stack** according to the function's arity.
- **Lists**: evaluated as normal arli expressions (atomic, prefix)

Special forms supported inside `(exec)`:
- `if`: pops boolean from data stack, then then/else branches from exec stack
- `do`: no-op (body items are already on exec stack)
- `quote`: pops the next exec stack item and pushes it to data stack as data

#### List Operations

| Word | Arity | Description |
|------|-------|-------------|
| `cons` | 2 | Prepend item to list |
| `car` | 1 | First element of list |
| `cdr` | 1 | Rest of list (all but first) |
| `list` | -1 | Create list (variadic — requires parens) |
| `nil?` | 1 | Check if nil |
| `list?` | 1 | Check if list |

#### Sequence Operations

| Word | Arity | Description |
|------|-------|-------------|
| `map` | 2 | Apply function to each element: `(map fn list)` |
| `filter` | 2 | Keep elements where function returns truthy |
| `reduce` | 3 | Accumulate: `(reduce fn init list)` |

#### Data Structures

| Word | Arity | Description |
|------|-------|-------------|
| `hash-map` | -1 | Create dict from key-value pairs (variadic — requires parens) |

#### I/O

| Word | Arity | Description |
|------|-------|-------------|
| `print` | 1 | Print value followed by newline |
| `.` | 2 | Attribute access / method call. At top level: `. obj attr`. Inside parens chains: `(. obj attr1 attr2 args...)` |
| `read` | 0 | Read a line from stdin |

> **Note on `.`**: The `.` symbol is overloaded. As a builtin function with arity 1 (for `print`-style output from Forth), it is registered but the arity-2 attribute-access version takes priority in the arity table. Prefer `print` for output.

#### Type Checking

| Word | Arity | Description |
|------|-------|-------------|
| `number?` | 1 | Check if value is int or float |
| `string?` | 1 | Check if value is string |
| `symbol?` | 1 | Check if value is Symbol |
| `fn?` | 1 | Check if value is callable (Builtin or Function) |

#### Result Type Constructors & Combinators

| Word | Arity | Description |
|------|-------|-------------|
| `Ok` | 1 | Create success result: `(Ok 42)` |
| `Err` | 1 | Create error result: `(Err "msg")` |
| `map-ok` | 2 | Transform Ok value: `(map-ok result fn)` |
| `and-then` | 2 | Chain Ok result: `(and-then result fn)` — passes through Err |
| `or-else` | 2 | Recover from Err: `(or-else result fn)` — passes through Ok |

#### Testing

| Word | Arity | Description |
|------|-------|-------------|
| `assert` | 2 | `(assert expr message)` — raises if expr is falsy |

#### Documentation

| Word | Arity | Description |
|------|-------|-------------|
| `doc` | 1 | `doc sym` | Retrieve documentation for a symbol |
| `doc!` | 2 | `doc! sym "text"` | Store documentation for a symbol |

#### Module System

| Word | Arity | Description |
|------|-------|-------------|
| `import-module` | 1 | Load and execute an `.arli` file |

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
    |                          assert, doc, match, import-module
    |                        - Host interop: import, import!, ., host
    |                        If not special: evaluate head as function,
    v                        evaluate args, apply function
```

### Special Form Behaviors

| Form | Behavior |
|------|----------|
| `quote` | Returns the argument unevaluated |
| `define` | Binds name to evaluated value; auto-registers arity if value is a Function |
| `defn` | Defines a function; arity registered during parsing (arity = number of params) |
| `defn-rec` | Defines a recursive function; arity registered BEFORE body is parsed |
| `if` | Evaluates condition; if truthy evaluates then-branch, else evaluates else-branch |
| `do` | Evaluates all expressions in sequence, returns value of last |
| `fn` | Creates a closure with given params and body |
| `while` | Repeatedly evaluates body while condition is truthy |
| `for` | Iterates variable over list elements, evaluating body each time |
| `let` | Creates new lexical scope, binds variables, evaluates body |
| `set!` | Mutates an existing binding (variable, dict key, or list element) |
| `cond` | Walks flat list of (test result) pairs; returns result of first truthy test |
| `assert` | Evaluates expression; raises AssertionError if falsy |
| `doc` | Stores or retrieves documentation string for a symbol |
| `match` | Matches value against patterns; returns result of first matching clause |
| `import-module` | Loads and executes an `.arli` file, returning the last expression's value |

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

## Pattern Matching

`match` is variadic (arity -1), so it requires parentheses:

```clojure
(match val
    (pattern1 result1)
    (pattern2 result2)
    (_ default))
```

Pattern rules:
- `_` (underscore symbol) matches anything
- A symbol matches a symbol of the same name
- A list matches if each element matches recursively
- A literal matches by equality (`==`)

## F-Expressions (Fexprs)

F-expressions (fexprs) are user-defined functions that **do not evaluate their arguments**. Instead, arguments are passed as raw, unevaluated forms (AST nodes). The fexpr body uses `eval` to selectively evaluate arguments.

This enables custom control structures, domain-specific languages, and lazy evaluation without modifying the evaluator.

### defn-fexpr (arity 3)

`defn-fexpr` creates an fexpr function. Like `defn`, it takes 3 arguments: name, parameter list, and body expression.

```clojure
;; Define a custom conditional — no parens needed (arity 3)
defn-fexpr my-if (c t e)
    if (eval c) (eval t) (eval e)

my-if true "yes" "no"      ;; -> "yes"
my-if false "yes" "no"     ;; -> "no"
```

### eval (arity 1)

The `eval` builtin evaluates a raw form in the current environment. This is the primary tool for fexpr bodies to selectively evaluate arguments.

```clojure
;; Only evaluate when needed
defn-fexpr if-verbose (vrb val)
    if (eval vrb) (eval val) nil

if-verbose true (+ 1 2)     ;; -> 3  (evaluates the sum)
if-verbose false (+ 1 2)    ;; -> nil (skips evaluation entirely)
```

### Raw Form Introspection

Since fexprs receive unevaluated forms, they can inspect the AST of their arguments:

```clojure
defn-fexpr show-form (x)
    x

show-form + 1 * 2 3         ;; returns the raw AST: [plus, 1, [mul, 2, 3]]
```

### Short-Circuit Evaluation

Fexprs shine for custom short-circuit logic:

```clojure
defn-fexpr short-or (a b)
    let ((av (eval a)))
        if av av (eval b)

short-or true (do print "not evaluated")  ;; -> True, no side effect
short-or false (do print "evaluated")     ;; prints "evaluated", returns nil
```

### Multiple Body Expressions

Like `defn`, the body of `defn-fexpr` is a single expression. Use `(do ...)` for multiple expressions:

```clojure
defn-fexpr log-if (c t e)
    (do
        print "condition:" c
        print "evaluated:" (eval c)
        if (eval c) (eval t) (eval e))
```

### Key Differences from Regular Functions

| Aspect | Regular Function (`defn`) | F-Expression (`defn-fexpr`) |
|--------|--------------------------|----------------------------|
| Argument evaluation | Eager (all args evaluated before call) | Lazy (raw forms passed in) |
| Custom control flow | Not possible | Full control via `eval` |
| Introspection | No access to caller's source | Can inspect raw AST |
| Performance | Optimal (pre-evaluated) | Overhead from deferred eval |
| Composition | Safe to pass between functions | Raw forms may capture symbols from calling scope |

### Important Caveats

1. **Parameter name conflicts**: Parameter names must not match registered operator names. If a parameter is named `cond` (arity 1), it will consume the next token as an argument during parsing.

2. **Symbol capture**: Raw forms passed from one fexpr to another may contain symbols bound in the outer scope. Use `eval` to resolve them before passing.

3. **Arity limits**: Like all arli functions, fexprs have a fixed arity. Variadic fexprs are not supported without parentheses.

## Result Type (Ok / Err)

The `Ok` and `Err` constructors create tagged values for error handling without exceptions:

```clojure
define result (Ok 42)       ;; -> (Ok 42)
define err   (Err "oops")   ;; -> (Err "oops")
```

Combinators for chaining:

| Combinator | Behavior |
|------------|----------|
| `(map-ok result fn)` | If result is Ok, apply fn to value and wrap in Ok |
| `(and-then result fn)` | If result is Ok, apply fn to value (fn returns a Result) |
| `(or-else result fn)` | If result is Err, call fn for recovery |

## Host Interop

### import (arity 1)

Import any Python module:
```clojure
import os
import json
```

### import! (arity 2)

Import with alias:
```clojure
import! os os_module
(. os_module getcwd)
```

### . (dot, arity 2)

Attribute access and method calls. At top level (arity-driven):
```clojure
. os sep              ;; getattr(os, 'sep')
```

Inside parentheses (chaining):
```clojure
(. os path join "a" "b")   ;; os.path.join("a", "b")
```

### python (arity 1)

Evaluate arbitrary Python expressions:
```clojure
python "repr(42)"           ;; -> "42"
define x 42
python "x * 2"             ;; -> 84 (environment variables available)
```

## Literal Syntax

### Numbers

| Format | Example | Value |
|--------|---------|-------|
| Decimal | `42` | 42 |
| Hex | `0xFF` | 255 |
| Octal | `0o77` | 63 |
| Binary | `0b1010` | 10 |
| Float | `3.14` | 3.14 |

### Strings

- Regular: `"hello"` with `\n`, `\t`, `\r`, `\"`, `\\` escape sequences
- Triple-quoted: `"""multi-line strings"""`

### Vectors

`[1 2 3]` expands to `(list 1 2 3)`.

### Maps

`{:a 1 :b 2}` expands to `(hash-map :a 1 :b 2)`.

### Keywords

Symbols starting with `:` — self-evaluate. Used as map keys.

### Unicode Symbols

Greek, Cyrillic, Chinese characters, and Unicode math symbols (`Sm` category) are valid in symbol names:
```clojure
define Ï€ 3.14159
define Î» (fn (x) * x 2)
define åŠ å€ (fn (x) * x 2)
```

## File Format

arli source files use the `.arli` extension. Files can contain multiple expressions separated by whitespace or newlines.

### Example: Fibonacci
```clojure
;; Recursive fibonacci
defn-rec fib (n)
    if (= n 0)
        0
        if (= n 1)
            1
            + fib (- n 1) fib (- n 2)

print fib 10  ;; prints 55
```

> Note: `defn-rec` requires parentheses because it must register arity before parsing the body. The form above uses `defn-rec` inside parens.

### Example: Stack Operations
```clojure
dup 5           ;; duplicate 5
swap 1 2        ;; swap (1 2) -> (2 1)
drop 42         ;; drop 42
over 1 2        ;; over: (1 2) -> (1 2 1)
```

### Example: Using Python
```clojure
import os
define cwd . os getcwd     ;; call os.getcwd()
print cwd

import json
python "{'a': 1}"          ;; evaluate Python expression
```

## REPL Features

- Multi-line input (blank line terminates expression)
- Meta-commands:
  - `/stack` — inspect data stack
  - `/env` — view environment
  - `/arity` — view arity table
  - `/clear` — clear stack
  - `/debug` — toggle debug tracing
  - `/reset` — reset evaluator

## Backend Comparison

| Feature | Python Backend | Go Backend | JavaScript Backend |
|---------|---------------|------------|-------------------|
| Run | `python -m arli` | `./arli.exe` | `node src/main.js` |
| Speed | Interpreted | Compiled | Interpreted (JIT) |
| Ecosystem | Any Python library | Pre-registered Go packages | npm modules |
| Dynamic eval | `host "code"` | N/A | `host "code"` via `new Function` |
| Package loading | `import os` (dynamic) | Pre-registered only | `require('fs')` (sync) |
| Extension | Write Python + register | Write Go + rebuild | Write JS + register |

## Future Directions

1. **Macros**: Lisp-style macro system using arity-based syntax
2. **Quasiquote**: Implement `quasiquote`/`unquote`/`unquote-splicing` evaluation
3. **Tail Call Optimization**: For recursive functions
4. **Pattern Matching**: Destructuring on function parameters
5. **Module System**: Namespaced imports/exports
6. **Error Handling**: try/catch with stack traces
7. **Performance**: JIT compilation, type inference
8. **C Backend**: Native binary via C compilation
