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
                                                          |
                                                          v
                                                  +-------------------+
                                                  |   VSA engine      |
                                                  | (vsa.py)          |
                                                  |                   |
                                                  | FHRR bind/bundle  |
                                                  | vsa-* primitives  |
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
| `VSAVec` | Vector Symbolic Architecture vector (FHRR) | `vsa-encode 'alpha` → `<vsa-vec>` |
| `VSAPair` | VSA-encoded cons pair with cached `car`/`cdr` | `vsa-cons 1 2` → `(1 . 2)` |

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
Subsequent inside parens    | True        | arity-driven (auto-unwraps defn/defn-rec/defn-fexpr)
defn / defn-rec (always)    | N/A         | special handler: registers arity BEFORE parsing body
```

### Special Form Parsers

- **`defn`**: `defn name (params) body` — at top level (or inside parens), registers function arity BEFORE parsing body, enabling parenthesis-free recursive calls.
- **`defn-rec`**: `defn-rec name (params) body` — alias / explicit recursive form, registers arity BEFORE parsing body (zero outer parentheses required).
- **`defn-fexpr`**: `defn-fexpr name (params) body` — defines an f-expression with arity registration before parsing body.
- **`fn`**: `fn (params) body` — creates an anonymous function (arity 2).

### Higher-Order Function Dynamic Arity

Parenthesis elimination in Arli is **static (parse-time)**, based on registered words in the `ArityTable`.
- When invoking named static functions directly, their arity is known (e.g., `inc add 1 2` parses as `(inc (add 1 2))`).
- When functions are passed as parameters (e.g. `f`, `g` in `defn apply2 (f g) ...`), their arity is **dynamic and unknown at parse time**.
- Therefore, higher-order parameter invocations **require explicit parentheses**:
  - `defn apply2 (f g) (f (g 1 2))`
  - `defn apply2 (f g) (f (g 1) 2)`

### Parsing Examples

```
Expression                     | Parsed AST
-------------------------------|----------------------------------------------
+ 1 2                          | [+, 1, 2]
* + 1 2 3                      | [*, [+, 1, 2], 3]
print + 1 2                    | [print, [+, 1, 2]]
if cond "yes" "no"             | [if, cond, "yes", "no"]
(if cond a b)                  | [if, cond, a, b]
defn add (x y) + x y           | [defn, add, [x, y], [+, x, y]]
(defn add (x y) (+ x y))       | [defn, add, [x, y], [+, x, y]]
defn-rec fact (n) if (= n 0) 1 * n fact (- n 1) | [defn, fact, [n], [if, [=, n, 0], 1, [*, n, [fact, [-, n, 1]]]]]
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

Every operator and special form in arli has a documented arity. **All operators with arity >= 0 can be used WITHOUT parentheses.** Only variadic operators (arity -1) and dynamic parameter invocations require parentheses.

### Special Forms (arity >= 0)

| Form | Arity | Syntax | Description |
|------|-------|--------|-------------|
| `define` | 2 | `define name value` | Bind name to evaluated value; auto-registers arity if value is a function |
| `defn` | 3 | `defn name (params) body` | Define function with parse-time arity registration (paren-free recursion supported) |
| `defn-rec` | 3 | `defn-rec name (params) body` | Explicit recursive function definition (paren-free) |
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
| `doc` | 1 | `doc sym` | Retrieve documentation |
| `doc!` | 2 | `doc! sym "text"` | Store documentation |
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
| `map` | 2 | Apply function to each element: `map fn list` |
| `filter` | 2 | Keep elements where function returns truthy: `filter fn list` |
| `reduce` | 3 | Accumulate: `reduce fn init list` |

#### Data Structures

| Word | Arity | Description |
|------|-------|-------------|
| `hash-map` | -1 | Create dict from key-value pairs (variadic — requires parens) |

#### I/O

| Word | Arity | Description |
|------|-------|-------------|
| `print` | 1 | Print value followed by newline: `print val` |
| `.` | 2 | Attribute access / method call. At top level: `. obj attr`. Inside parens chains: `(. obj attr1 attr2 args...)` |
| `read` | 0 | Read a line from stdin |

> **Note on `.`**: The `.` symbol is overloaded. As a builtin function with arity 1 (for `print`-style output from Forth), it is registered but the arity-2 attribute-access version takes priority in the arity table. Prefer `print` for output.

#### Type Checking

| Word | Arity | Description |
|------|-------|-------------|
| `number?` | 1 | Check if value is int or float: `number? x` |
| `string?` | 1 | Check if value is string: `string? s` |
| `symbol?` | 1 | Check if value is Symbol: `symbol? sym` |
| `fn?` | 1 | Check if value is callable (Builtin or Function): `fn? f` |

#### Result Type Constructors & Combinators

| Word | Arity | Description |
|------|-------|-------------|
| `Ok` | 1 | Create success result: `Ok 42` |
| `Err` | 1 | Create error result: `Err "msg"` |
| `map-ok` | 2 | Transform Ok value: `map-ok result fn` |
| `and-then` | 2 | Chain Ok result: `and-then result fn` — passes through Err |
| `or-else` | 2 | Recover from Err: `or-else result fn` — passes through Ok |

#### VSA (Vector Symbolic Architecture)

All 27 words are built in; every fixed arity works without parentheses. Full
semantics are in [VSA (Vector Symbolic Architecture)](#vsa-vector-symbolic-architecture).

| Word | Arity | Description |
|------|-------|-------------|
| `vsa-dim` | 0 | Current vector dimension (default 4096) |
| `vsa-reset` | 1 | Reinitialize the engine at a new dimension (4–65536), clearing memory |
| `vsa-seed` | 1 | Reseed the random stream: `vsa-seed 7` |
| `vsa-random` | 0 | New random vector from the stream |
| `vsa-bind` | 2 | Binding (element-wise complex product) |
| `vsa-bundle` | -1 | Bundling / superposition (sum, each component re-normalized to unit magnitude; variadic — parens) |
| `vsa-majority` | -1 | FHRR centroid; identical to `vsa-bundle` for unit phasors |
| `vsa-unbind` | 2 | Unbinding (multiply by conjugate — exact inverse of bind) |
| `vsa-similarity` | 2 | Cosine similarity in `[-1, 1]` |
| `vsa-permute` | 2 | Cyclic shift by n (negative shifts back) |
| `vsa-encode` | 1 | Encode any encodable value to a vector |
| `vsa-cons` | 2 | Create a VSA pair |
| `vsa-car` | 1 | First element of a VSA pair / VSA vector / list |
| `vsa-cdr` | 1 | Rest of a VSA pair / VSA vector / list |
| `vsa-list` | -1 | Nil-terminated VSA pair chain (variadic — parens) |
| `vsa->list` | 1 | Convert a VSA pair chain to a plain arli list |
| `vsa-pair?` | 1 | Check if value is a `VSAPair` |
| `vsa-vec?` | 1 | Check if value is a `VSAVec` |
| `vsa-type` | 1 | Type class as a symbol: `symbol`, `number`, `pair`, `vsa-vec`, … |
| `vsa-register` | 1 | Register a value in the cleanup memory |
| `vsa-cleanup` | 1 | Nearest registered value above 0.3, else `nil` |
| `vsa-query` | 2 | `vsa-query vec k` → VSA pairs `(similarity . value)`, nearest first |
| `vsa-clear` | 0 | Empty the cleanup memory |
| `vsa-factorize` | -1 | `(vsa-factorize bundle cb1 cb2 …)` — resonator factorization |
| `vsa-match` | 2 | `vsa-match '(?x ?y) value` — bind `?vars` over VSA structure (pattern is data, so it is quoted) |
| `vsa->floats` | 1 | Interleaved `re, im` floats of the encoded value |
| `floats->vsa` | 1 | Wrap such a list back into a `VSAVec` |

#### Testing

| Word | Arity | Description |
|------|-------|-------------|
| `assert` | 2 | `assert expr message` — raises if expr is falsy |

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

## VSA (Vector Symbolic Architecture)

arli includes a self-contained **FHRR** (Fourier Holographic Reduced
Representation) subsystem, ported from *VSA-Lisp* ("Velo"). Instead of encoding
structure in pointers, structure is encoded in high-dimensional complex
vectors: a pair is a single vector produced by binding and bundling, and
`car`/`cdr` can be recovered by unbinding plus nearest-neighbour cleanup.

```
vsa-cons a b  =  bind(CAR, encode(a))  ⊕  bind(CDR, encode(b))
vsa-car  p    =  cleanup( unbind(p, CAR) )
```

The engine has **no third-party dependencies** — the random generator, vector
math and cleanup memory are implemented per backend. Python, JavaScript and Go
run the same 32-bit PRNG, so a program produces **bit-identical** vectors and
similarities on all three (verified against shared parity probes, e.g.
`vsa-similarity (vsa-bundle 'alpha 'beta) (vsa-encode 'alpha)` = 0.6275918471608055
everywhere).

### Vector model

| Property | Value |
|----------|-------|
| Dimension | 4096 by default (`vsa-dim`, changeable with `vsa-reset`) |
| Representation | Unit-magnitude complex phasors, float32 `(re, im)` pairs |
| Bind | Element-wise complex product (commutative) |
| Unbind | Product with the conjugate — the exact inverse of bind |
| Bundle / majority | Sum, then each component normalized back to unit magnitude (FHRR centroid) |
| Similarity | Mean real part of the inner product, in `[-1, 1]` |
| Permute | Cyclic shift of components (`vsa-permute v n`; negative n shifts back) |
| Cleanup threshold | 0.3 |

### Encoding values

`vsa-encode` (and every primitive that takes "any value") maps a value to a vector:

| Value | Encoding |
|-------|----------|
| symbol | deterministic vector from a hash of its name (stable across runs and backends) |
| string | deterministic vector from a hash of its text |
| number | fractional power encoding — numbers stay similar while they are close |
| pair | `bind(CAR, encode(car))` bundled with `bind(CDR, encode(cdr))` |
| list | nil-terminated chain of VSA pairs |
| `nil`, `true`, `false` | fixed engine vectors |
| map, builtin, fn | not encodable — raises `vsa: cannot encode …` |

### Cleanup memory (associative retrieval)

Unbinding a noisy vector leaves an approximate result. `vsa-register` stores
`value → vector`; `vsa-cleanup` returns the nearest registered value above 0.3,
`vsa-query` returns the k nearest with their similarities, and `vsa-clear`
empties the memory. Memory is per evaluator, so programs start clean.

### Examples

```clojure
;; Similarity and binding
define a vsa-encode 'alpha
define b vsa-encode 'beta
< vsa-similarity a b 0.1          ;; true — unrelated symbols are orthogonal
> vsa-similarity vsa-unbind vsa-bind 'alpha 'beta 'alpha vsa-encode 'beta 0.999
;; true — unbind inverts bind exactly

;; VSA pairs: car/cdr of a pair are exact because the pair caches them
define p vsa-cons 'alpha 'beta
vsa-car p                          ;; alpha
vsa-cdr p                          ;; beta

;; VSA lists and conversion to plain arli lists
define xs (vsa-list 1 2 3)
vsa->list xs                       ;; (1 2 3)
vsa->list vsa-cdr xs               ;; (2 3)

;; Structure inside a single vector, recovered through cleanup memory
vsa-register 'alpha
vsa-register 'beta
define v vsa-encode (vsa-cons 'alpha 'beta)
vsa-car v                          ;; alpha  (unbind + cleanup)
vsa-cdr v                          ;; beta

;; Pattern variables bind through the same unbinding path.
;; Patterns are data, so they are quoted (as in VSA-Lisp):
vsa-match '(?x ?y) (vsa-list 'alpha 'beta)   ;; ((?x alpha) (?y beta))

;; Resonator factorization: recover the factors of a bundle
define c1 (list 'alpha 'beta)
define c2 (list 'gamma 'delta)
define mixed (vsa-bundle vsa-bind 'alpha 'gamma vsa-bind 'beta 'delta)
(vsa-factorize mixed c1 c2)        ;; (alpha gamma)
```

### Ranges and limits

- Retrieval through a **raw vector** costs one unbinding per call: `vsa-car` /
  `vsa-cdr` clean up a single step (≈ 0.64 similarity at D = 4096) and do not
  walk. Walking several steps is `vsa-match`, whose leaf cleanup is reliable to
  two levels (≈ 0.64, then ≈ 0.40) and drops below the 0.3 threshold at three —
  measured: `vsa-match '(?x ?y ?z ?w) vsa-encode (vsa-list 'a 'b 'c 'd)` binds
  `a`, `b`, then `nil`, `nil`.
- Walk the `VSAPair` chain itself (or a plain list) instead of its encoding and
  the read is exact at any depth, because pairs cache their `car`/`cdr` — the
  same program with `(vsa-list 'a 'b 'c 'd)` binds all four.
- A plain bundle of N unrelated items is **not** directly readable: the maximum
  similarity to a member is ≈ 0.9/√N (measured 0.302 at N = 10, 0.119 at
  N = 100, 0.081 at N = 300, against a ±0.016 noise floor), so `vsa-cleanup` of
  the bundle itself clears 0.3 only for N ≲ 10, and what it returns is an
  arbitrary member. Give retrieval a cue instead:
  `vsa-cleanup vsa-unbind bundle role`.
- `vsa-permute` shifts components by whole positions; it converts a component
  into a *position role*. Encode an ordered sequence by binding each item to a
  permuted role — `(vsa-bundle vsa-bind 'alpha vsa-permute one 0 vsa-bind 'beta vsa-permute one 1)`
  — then read position i back with `vsa-cleanup vsa-unbind seq vsa-permute one i`.
  Bundling bare `permute(item, i)` terms instead is cheaper but can only be
  queried by scoring candidates, not by unbinding.
- The random stream is deterministic once seeded (`vsa-seed`), and every
  named entity has a hash-derived vector, so two runs of the same program
  produce the same similarities.

### Differences from VSA-Lisp (Velo)

| VSA-Lisp | arli |
|----------|------|
| Global engine and cleanup memory | One engine per evaluator |
| FHRR, MAP, BSC, HRR, GHRR selectable via `vsa-set-model` | Self-contained FHRR; `vsa-reset` changes dimension only |
| `vsa-codebook` objects | Folded into `vsa-register` / `vsa-query` over the cleanup memory |
| `vsa->array` / `array->vsa` numpy bridge | `vsa->floats` / `floats->vsa` over plain lists |
| `cons` returns a VSA pair | Additive: `vsa-cons`; core `cons`/`car`/`cdr` keep list semantics |
| `match` binds `?vars` | `match` unchanged (structural); `vsa-match` binds `?vars` through VSA |
| `put`/`get`/`prop` property lists, `fexpr` | Already covered by arli's `hash-map` and `defn-fexpr` |

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
| VSA | Built in (FHRR, 27 words) | Built in (FHRR, 27 words) | Built in (FHRR, 27 words) |

## Future Directions

1. **Macros**: Lisp-style macro system using arity-based syntax
2. **Quasiquote**: Implement `quasiquote`/`unquote`/`unquote-splicing` evaluation
3. **Tail Call Optimization**: For recursive functions
4. **Pattern Matching**: Destructuring on function parameters
5. **Module System**: Namespaced imports/exports
6. **Error Handling**: try/catch with stack traces
7. **Performance**: JIT compilation, type inference
8. **C Backend**: Native binary via C compilation
9. **VSA models**: MAP/BSC/HRR backends and a selectable dimension/model beyond the built-in FHRR
