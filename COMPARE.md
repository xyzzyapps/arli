# arli Compared: What's There, What's Missing

## The Design Philosophy

Before the comparison, let's state the principle explicitly:

> arli's core is intentionally minimal. Everything beyond basic control flow and
> data comes from the **base language** (Python, and eventually Go/C).
> Syntax sugar belongs in libraries and macros, not in the core.

This means "missing" features fall into three categories:

1. **Core should have it** — essential for any language backend
2. **Base language already provides it** — no need to add to core
3. **Macro / library territory** — can be added without touching core

---

## Comparison Matrix

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| **Arity-driven syntax** | No | No | No | **Yes** | Unique strength |
| **Stack evaluation** | No | No | No | **Yes** | Unique strength |
| **Python interop** | No | **Full** | No | **Full** | Matches HyLang |
| **Core data structures** | Vec, Map, Set, List, Seq | List, Dict, Set, Vector | Array, Tuple, Struct, Table, Buffer | **List, nil** | Intentionally minimal; Python provides the rest |

### Control Flow

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| if / cond / when | Yes | Yes | Yes | **Yes** | Present |
| let / binding | Yes | Yes | Yes | **Yes** | Present |
| loop / recur | Yes | loop | loop | **for / while** | TCO missing |
| **try / catch / throw** | Yes | Yes | Yes | **No** | **CORE GAP** |
| Threading macros (->, ->>) | Yes | Yes | Yes | **No** | Macro territory |

### Data & Sequences

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| nil / false / true | Yes | Yes | Yes | **Yes** | Present |
| cons / car / cdr / list | Yes | Yes | Yes | **Yes** | Present |
| Vectors `[a b c]` | Yes | Yes | Yes | **No** | `python "[1 2 3]"` |
| Maps `{:a 1}` | Yes | Yes | Yes | **No** | `python "{'a': 1}"` |
| Sets `#{1 2}` | Yes | Yes | Yes | **No** | `python "{1, 2}"` |
| Keywords `:foo` | Yes | Yes | Yes | **No** | Strings suffice |
| **Sequence abstraction** | Yes | partial | Yes | **No** | Python iterators |
| **Lazy sequences** | Yes | No | Yes | **No** | Python generators |
| **Destructuring** | Yes | Yes | Yes | **No** | **Nice-to-have** |

### Functions

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| fn / defn / closures | Yes | Yes | Yes | **Yes** | Present |
| Arity overloading | Yes | Yes | Yes | **No** | Base language |
| Multi-methods | Yes | Yes | Yes | **No** | Base language |
| **Macros** | Yes | Yes | Yes | **No** | **Design decision** |

### Error Handling

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| **try / catch / finally** | Yes | Yes | Yes | **No** | **CORE GAP** |
| Stack traces | Yes | Yes | Yes | partial | Python traces |
| User-defined errors | Yes | Yes | Yes | **No** | raise via python |

### Module System

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| require / import | Yes | Yes | Yes | **import** | Python's is fine |
| Namespaces | Yes | Yes | Yes | **global env** | Minimal |
| Forward declarations | Yes | No | No | **No** | Interleaved parse-eval helps |

### I/O

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| print / read | Yes | Yes | Yes | **print** | Python has everything |
| File I/O | Yes | Yes | Yes | **No** | `python "open(...)"` |
| Networking | libraries | libraries | **built-in** | **No** | Python libraries |

### Concurrency

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| Threads | Yes | Yes | Yes | **No** | Python threading |
| Async/await | libraries | libraries | **built-in fibers** | **No** | `python "asyncio..."` |
| STM / atoms / agents | Yes | No | No | **No** | Abstractions over base |

### Reflection & Meta

| Feature | Clojure | HyLang | Janet | arli | Verdict |
|---------|---------|--------|-------|-----|---------|
| **Macros** | Yes | Yes | Yes | **No** | **Design decision** |
| Eval / read | Yes | Yes | Yes | **python** | Python's eval |
| Docstrings | Yes | Yes | No | **No** | Would be nice |

---

## The Gaps That Matter

### 1. CORE GAP: Error Handling (try / catch / throw)

This is the single biggest missing piece. Every language needs to handle errors.
Without it, a Python exception from an imported library will crash the entire
arli program with no way to recover.

**Should be core** because it's backend-independent and every evaluator needs it.

```
try risky-operation catch-handler
with-err error-message handler
```

I'd propose arity 2: `try body handler` where handler is a function taking the
error as argument. Or arity 3 for try/catch/finally.

### 2. DESIGN DECISION: Macros

arli doesn't have macros. This is intentional — macros require a separate
compile phase, which conflicts with arli's interleaved parse-eval model
(where each expression is parsed and evaluated immediately).

However, you can achieve similar things via:

- **Python functions** that return S-expressions: `python "make_expression()"`
- **The `python` special form**: evaluate arbitrary code at compile time
- **The `.` operator**: chain Python functions freely

For example, a threading macro equivalent in arli:

```clojure
;; Instead of (-> x (f 1) (g 2))
;; Just write:
g f x 1 2

;; Wait — that's wrong. Threading re-orders arguments.
;; With arity-driven syntax, threading is less necessary because
;; composition is already flat:
+ 1 * 2 3    ;; = (+ 1 (* 2 3)) -- no nesting needed!
```

Actually, arity-driven syntax already solves one of the main problems that
threading macros solve: deep nesting. You rarely need `->` in arli because
there's no deeply nested parentheses to read inside-out.

### 3. NICE-TO-HAVE: Destructuring

Destructuring in let/fn/defn bindings is pure ergonomics. Lists handle it
partially because `car` and `cdr` are trivially accessible. Python destructuring
is available:

```clojure
python "a, b = [1, 2]; a"   ;; → 1
```

### 4. NOT A GAP: Data Structure Literals

Python dicts, sets, and lists are fully accessible:

```clojure
python "[1, 2, 3]"                     ;; vector
python "{'a': 1, 'b': 2}"             ;; map
python "{1, 2, 3}"                    ;; set
import json                            ;; then use json.loads, etc.
```

Adding `[1 2 3]` vector syntax to the tokenizer is ~10 lines. But do you need
it when `python "[1, 2, 3]"` works? That's a philosophy call.

---

## What's Unique About arli

Nothing else has this combination:

| Feature | Only arli |
|---------|----------|
| **Arity-driven syntax** | Eliminates parentheses for ALL known-arity functions |
| **Stack evaluation** | Forth-like explicit data stack |

These two features together create a language that reads differently from
anything else. Code is flat, not nested. Data flows through the stack rather
than through nested expressions. The arity table replaces the paren-matching
that other Lisps force you to do mentally.

## What to Add to Core (if anything)

| Priority | Feature | Arity | Reason |
|----------|---------|-------|--------|
| **HIGH** | `try` / `catch` / `throw` | 2 / 1 | Every backend needs error handling |
| MEDIUM | `->` threading builtin | 2 | Complements arity syntax |
| LOW | Destructuring | — | Ergonomic sugar |
| FUTURE | Macros | — | Requires compile phase |

## Future Backends (Go / C)

The architecture already supports this division:

**Platform-independent** (shared core):
- Tokenizer and arity-driven parser
- Core data types (number, string, symbol, list, nil, function)
- Control flow (if, do, while, let, for, cond)
- Stack operations (dup, swap, drop, etc.)
- ArityTable

**Backend-specific** (swap Python for Go/C):
- Evaluator (`_eval_expr`) — walks AST, applies rules
- Builtins — reimplemented per backend
- I/O — platform I/O
- Module loading — `import foo` in Go loads Go module, etc.
- `.` operator — `getattr` in Python, reflection in Go/C
- `python` special form → `goeval` / `ceval`

The key is that the **Parser** and **AST** are completely language-agnostic.
They produce nested Python lists (which could be JSON or protobuf for other
backends). The evaluator is where backend-specific behavior lives.
