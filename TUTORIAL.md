# Practical arli: A Lisp Without Parentheses

## Or, How I Learned to Stop Worrying and Love Arity

### Introduction: Why Another Language?

You know the complaint. It's the same one that's been leveled at Lisp since McCarthy first scrawled parentheses on a blackboard in 1958: "Too many parens."

But here's the thing â€” the parentheses in Lisp aren't noise. They're structure. They tell you (and the computer) where one expression ends and another begins. The problem isn't that Lisp has parentheses. The problem is that **every expression** needs them, even when the structure is already obvious.

Consider this:

```lisp
(+ 1 (* 2 3))
```

You read this as: "add 1 to the result of multiplying 2 by 3." The parentheses around `(* 2 3)` tell you it's a sub-expression. But look at `+` â€” it takes two arguments. Everyone knows `+` takes two arguments. Why do we need the outer parentheses to tell us that?

What if we wrote this instead:

```
+ 1 * 2 3
```

The `+` takes two arguments. The first is `1`. The second is `* 2 3`, which is itself an expression where `*` takes two arguments: `2` and `3`. The structure is encoded in the **arity** of the operators, not in parentheses.

This is the core idea behind **arli**: when a function's arity is known, you don't need parentheses. When it's unknown (variadic), you still use them as a fallback.

But arli isn't just "Lisp with fewer parens." It's also **stack-based** â€” inspired by Forth â€” so data flows through an explicit stack. Functions push and pop values. It's a different way of thinking about computation, and once it clicks, it changes how you structure programs.

And finally, arli sits **on top of Python**. Every Python module, function, and object is accessible from arli. You can `import os`, call `os.getcwd()`, or `eval("python code")` directly. This means arli isn't a toy â€” it's a scripting language with access to the entire Python ecosystem.

---

### Chapter 1: The Simplest Things

**What you'll build by the end of this chapter:** A working postfix calculator that reads expressions like `3 4 + 2 *` and computes them correctly. Along the way you'll learn how arli's arity-driven syntax works, how to do arithmetic, and how to see what's on the stack.

Let's start with arithmetic. Fire up the arli REPL:

```
$ python -m arli
arli v0.1.0
Arity-driven Lisp with Forth-like stack operations
Type 'help' for commands, 'exit' or Ctrl+C to quit

arli>
```

Type `+ 1 2` and press Enter:

```
arli> + 1 2
3
```

No parentheses. Just `+`, then `1`, then `2`. The `+` function has **arity 2** â€” it takes two arguments. The parser sees `+`, knows it needs two things, and consumes `1` and `2` as its arguments. The evaluator adds them, and pushes `3` onto the stack. The REPL prints the top of the stack.

Nesting works the same way:

```
arli> * + 2 3 4
20
```

This is `(* (+ 2 3) 4)`. The `*` has arity 2. Its first argument is `+ 2 3` (because `+` has arity 2 and consumes `2` and `3`). Its second argument is `4`. The evaluator computes `(2 + 3) * 4 = 20`.

You can chain as deep as you want:

```
arli> / - 10 2 3
2.666...
```

This is `(/ (- 10 2) 3)`. The `-` subtracts `2` from `10`, giving `8`. Then `/` divides `8` by `3`.

**The rule**: every function has a known arity (argument count). The parser uses this to group expressions automatically. No parentheses needed.

But what about functions that take a variable number of arguments? Like `list`, which can take any number of items? For those, you **do** use parentheses:

```
arli> (list 1 2 3)
(1 2 3)
```

The parentheses tell the parser "everything inside is one expression." The `list` function receives all three items as its arguments.

This is the complete syntax of arli:
1. If a function's arity is **known**, write it without parentheses: `+ 1 2`
2. If a function's arity is **unknown** (variadic), use parentheses: `(list 1 2 3)`
3. If you want to be explicit, use parentheses anywhere: `(+ 1 2)` is the same as `+ 1 2`

**All** of arli's core operators and special forms have known arities (>= 0) and can be used without parentheses:

| Operator | Arity | No-parens example |
|----------|-------|--------------------|
| `if` | 3 | `if cond "yes" "no"` |
| `define` | 2 | `define x 42` |
| `defn` | 3 | `defn add (x y) + x y` |
| `fn` | 2 | `fn (x) * x 2` |
| `while` | 2 | `while cond body` |
| `for` | 3 | `for x list print x` |
| `let` | 2 | `let ((x 1)) + x 1` |
| `set!` | 2 | `set! x 42` |
| `cond` | 1 | `cond ((> x 0) "pos")` |
| `quote` | 1 | `quote + 1 2` |
| `import` | 1 | `import os` |
| `.` | 2 | `. os sep` |
| `python` | 1 | `python "repr(42)"` |

The only forms that **do** require parentheses are variadic ones (arity -1) and `defn-rec` (which needs special handling for recursion):

| Operator | Arity | Example |
|----------|-------|---------|
| `do` | -1 | `(do a b c)` |
| `list` | -1 | `(list 1 2 3)` |
| `match` | -1 | `(match val ...)` |
| `and` | -1 | `(and a b c)` |
| `or` | -1 | `(or a b c)` |
| `hash-map` | -1 | `(hash-map :a 1)` |
| `defn-rec` | -1 | `(defn-rec fact (n) ...)` |

And `defn-rec` specifically requires parentheses because it must register the function's arity **before** parsing the body, which requires a special parser handler that only activates inside parens.

**Unicode symbols** are fully supported as function and variable names â€” Greek letters, Cyrillic, Chinese characters, even math symbols:
```
arli> define Ï€ 3.14159
3.14159
arli> define Î» (fn (x) * x 2)
<fn Î»>
arli> Î» 5
10
arli> define åŠ å€ (fn (x) * x 2)
<fn åŠ å€>
```

All of arli's built-in operators have known arities. **None of them need parentheses.** Let's see what that looks like.

#### Practical Exercise: Temperature Converter

Let's apply what you've learned. Write an expression that converts 100 degrees Fahrenheit to Celsius using the formula `(F - 32) * 5/9`:

```
arli> * - 100 32 / 5 9
37.77777777777778
```

How does this work? The parser sees `*` (arity 2). Its first argument is `- 100 32` (since `-` has arity 2). Its second argument is `/ 5 9` (since `/` has arity 2). So it's `(* (- 100 32) (/ 5 9))` = `68 * 0.555...` = `37.78`.

No parentheses, no variables, just the math expressed naturally. The stack handles all intermediate values automatically.



---

### Chapter 2: The Core Operators

**What you'll build:** A number-guessing game using `if`, `while`, `define`, and `let`. You'll learn how to make decisions, store values, loop, and create local scopes — the essential toolkit for any program.

#### if: The Three-Way Conditional

In most Lisps, you write `(if condition then else)`. In arli, `if` has arity 3, so:

```
arli> if (> 5 3) "yes" "no"
"yes"
arli> if nil "yes" "no"
"no"
```

The `if` takes three arguments: a condition, a then-value, and an else-value. It evaluates the condition first. If truthy (anything except `nil` and `false`), it evaluates and returns the then-value. Otherwise, the else-value.

The condition itself is arity-driven: `> 5 3` is `(> 5 3)`, which returns `true` because `5 > 3`. No parens needed anywhere.

#### define: Giving Things Names

`define` has arity 2:

```
arli> define x 42
42
arli> x
42
```

You can define a name to any value, including the result of an expression:

```
arli> define y + 1 2
3
arli> y
3
```

#### set!: Changing Things

`set!` has arity 2 â€” it mutates an existing binding:

```
arli> define x 10
10
arli> set! x 20
20
arli> x
20
```

#### quote: Don't Evaluate This

`quote` has arity 1. It returns its argument unevaluated:

```
arli> quote + 1 2
(+ 1 2)
```

Instead of computing `+ 1 2` (which would be `3`), `quote` returns the expression itself. This is how we work with code as data.

#### do: Sequencing (Variadic)

`do` is variadic â€” it takes any number of expressions and returns the last. Because it's variadic, you use parentheses:

```
arli> (do print "hello" + 1 2)
"hello"
3
```

The first expression `print "hello"` prints "hello" as a side effect. The second expression `+ 1 2` produces `3`, which becomes the result of the `do`. The parentheses tell the parser where the `do` block starts and ends.

You can chain as many expressions as you need:

```
arli> (do print 1 print 2 print 3)
1
2
3
```
#### while: Looping

`while` has arity 2. It takes a condition and a body:

```
arli> define i 0
0
arli> while (< i 5) (do print i set! i + i 1)
0
1
2
3
4
```

The `while` evaluates the condition `(< i 5)`. If truthy, it evaluates the body `(do print i set! i + i 1)`. The `do` block sequences two expressions: print the current value of `i`, then increment it. This repeats until `i` reaches `5`.
#### let: Local Bindings

`let` has arity 2. It takes a bindings list and a body:

```
arli> let ((x 5) (y 3)) + x y
8
```

The bindings list `((x 5) (y 3))` binds `x` to `5` and `y` to `3` in a new lexical scope. The body `+ x y` evaluates to `8`. The bindings are only visible inside the `let`.

#### for: Iteration

`for` has arity 3: a variable, a list, and a body:

```
arli> for x (list 1 2 3) print x
1
2
3
```

The `for` binds `x` to each element of the list and evaluates the body. It's a simple, clean loop.

#### cond: Multi-way Branching

`cond` has arity 1. It takes a list of (test result) pairs:

```
arli> define x 5
5
arli> cond ((> x 10) "big" (< x 3) "small" true "medium")
"medium"
```

The list contains pairs: `(> x 10)` paired with `"big"`, `(< x 3)` paired with `"small"`, and `true` (always matches) paired with `"medium"`. `cond` walks through the pairs, evaluates each test, and returns the result of the first truthy test.



#### Practical Exercise: Number Guessing Game

Let's combine `if`, `while`, `define`, and `let` into a real program. This game picks a number and gives you feedback:

```clojure
(import random)
(define target (. random randint 1 100))
(define guess -1)
(while (!= guess target)
    (do
        (print "Guess a number (1-100):")
        (define guess (. int (host "input()")))
        (if (< guess target)
            (print "Too low!")
            (if (> guess target)
                (print "Too high!")
                (print "Correct!")))))
```

This uses Python's `random` module, `input()` for user input, and arli's control flow — all working together without parentheses on any known-arity form.


---

### Chapter 3: The Stack

**What you'll build:** A postfix (RPN) calculator that evaluates expressions like `3 4 + 2 *` using stack operations — and understand exactly how data flows through arli's evaluator.

Now let's talk about the stack. arli isn't just a Lisp â€” it's a **Forth-like** Lisp. This means there's an explicit data stack that all operations push to and pop from.

When you evaluate an expression, the result is pushed onto the stack:

```
arli> + 1 2
3
```

The stack now contains `[3]`.

But you can also inspect the stack with `/stack`:

```
arli> + 1 2
3
arli> * 3 4
12
arli> /stack
Stack (2 items):
  0: 3
  1: 12
```

Both results are on the stack. The most recent is on top (index 1).

arli provides Forth-like stack manipulation words with known arities:

**`dup` (arity 1)** â€” duplicates the top of the stack:
```
arli> dup 5
5
arli> /stack
Stack (2 items):
  0: 5
  1: 5
```

**`swap` (arity 2)** â€” swaps the top two items:
```
arli> swap 1 2
2
1
arli> /stack
Stack (2 items):
  0: 2
  1: 1
```

**`drop` (arity 1)** â€” discards the top of the stack:
```
arli> drop 42
nil
arli> /stack
Stack is empty.
```

**`over` (arity 2)** â€” duplicates the second item:
```
arli> over 1 2
1
arli> /stack
Stack (3 items):
  0: 1
  1: 2
  2: 1
```

**`rot` (arity 3)** â€” rotates the top three items:
```
arli> rot 1 2 3
2
3
1
arli> /stack
Stack (3 items):
  0: 2
  1: 3
  2: 1
```

**`nip` (arity 2)** â€” drops the second item:
```
arli> nip 1 2
2
arli> /stack
Stack (1 items):
  0: 2
```

**`tuck` (arity 2)** â€” duplicates the top under the second:
```
arli> tuck 1 2
2
1
2
arli> /stack
Stack (3 items):
  0: 2
  1: 1
  2: 2
```

These stack operations are the building blocks of concatenative programming. Instead of naming intermediate values with variables, you manipulate them directly on the stack. It takes some getting used to, but it leads to remarkably concise code.

---

### Chapter 4: Defining Functions

**What you'll build:** A library of reusable mathematical functions — `square`, `cube`, `average`, `factorial` — and understand how arli knows how many arguments each one takes without you telling it.

#### defn: The Standard Way

`defn` has arity 3, so you can use it without parentheses:

```
arli> defn add (x y) + x y
<fn add>
arli> add 1 2
3
```

The syntax is `defn name (params) body` â€” three arguments: a name, a parameter list, and a single body expression. If you need multiple body expressions, use an explicit `(do ...)` block:

```
arli> defn add (x y) (do print "adding" + x y)
<fn add>
```

When you define `add` with parameters `(x y)`, arli registers `add` with arity 2. From that point on, `add 1 2` works without parentheses â€” the parser knows `add` needs two arguments and consumes them automatically.
#### defn-rec: Recursive Functions

For recursive functions, use `defn-rec`:

```
arli> (defn-rec fact (n)
...   (if (= n 0)
...       1
...       (* n fact (- n 1))))
<fn fact arity=1>
arli> fact 5
120
```

The difference between `defn` and `defn-rec` is that `defn-rec` registers the function's arity **before** parsing the body. This means the function can call itself recursively â€” when the parser encounters `fact` in the body, it already knows `fact` takes 1 argument.

Without `defn-rec`, you'd get an error because `fact` wouldn't be in the arity table when the body is parsed.

#### fn: Anonymous Functions

Use `fn` to create a lambda:

```
arli> define double (fn (x) (* x 2))
<fn anon arity=1>
arli> double 5
10
```

The `fn` form creates an anonymous function. When you `define` it, arli detects it's a function and registers its arity automatically.

---

### Chapter 5: Using Python From arli

**What you'll build:** A script that reads a JSON configuration file, extracts values, and uses them to process files — combining arli's syntax with Python's entire standard library.

The whole reason arli exists on top of Python is to give you access to Python's libraries. Let's see how it works.

#### import: Pulling in Python Modules

`import` has arity 1:

```
arli> import os
<module 'os' from '...'>
```

The Python `os` module is now bound to the name `os` in arli's environment.

#### . (dot): Accessing Attributes

The `.` operator has arity 2. It gets an attribute from an object:

```
arli> . os sep
"\\"
```

This is `getattr(os, 'sep')` â€” it returns the path separator character.

For chaining attribute access and method calls, use parentheses:

```
arli> (. os path join "a" "b")
"a\\b"
```

Inside the parentheses, `.` chains through attributes. It starts with `os`, gets `path` (giving `os.path`), gets `join` (giving `os.path.join`), then calls it with `"a"` and `"b"`. The result is `"a\\b"` (or `"a/b"` on Linux).

You can assign results to names:

```
arli> define p . os path
<module 'ntpath' from '...'>
arli> define mydir . p join "home" "user"
"home\\user"
```

#### import!: Import with an Alias

Sometimes you want to import a module under a different name to avoid conflicts. `import!` has arity 2:

```
arli> import! os myos
<module 'os' from '...'>
arli> . myos sep
"\\"
```

This imports the Python `os` module but binds it to `myos` instead of `os`.

#### The python Special Form

For arbitrary Python evaluation, use `python` with arity 1:

```
arli> python "repr(42)"
"42"
```

This evaluates the string as a Python expression. The environment's bindings are available as local variables:

```
arli> define x 42
42
arli> python "x * 2"
84
```

#### Putting It All Together

Let's write a arli script that uses Python's `pathlib` library:

```clojure
;; List Python files in the current directory
import pathlib
define p . pathlib Path "."
define files . p glob "*.py"
for f files print f
```

Or use Python's `json` to parse some data:

```clojure
;; Parse JSON and extract a field
import json
define data python "{'name': 'arli', 'type': 'language'}"
define parsed . json loads data
. parsed name
```

This is the power of arli: Lisp-like syntax with Forth-like stack semantics and direct access to the entire Python ecosystem. No FFI. No bindings. Just `import` and go.

---

### Chapter 6: Vectors, Maps, and Keywords

**What you'll build:** A small database of records using maps and keywords, queried with pattern matching — a contact list you can look up by name or field.

Arli provides literal syntax for vectors and maps as syntactic sugar.

#### Vector Literals

Use `[` and `]` to create lists:

```
arli> [1 2 3]
(1 2 3)
arli> car [1 2 3]
1
arli> map (fn (x) * x 2) [1 2 3]
(2 4 6)
```

#### Map Literals

Use `{` and `}` with keywords for hash-map creation:

```
arli> {:name "arli" :version 1}
{':name': 'arli', ':version': 1}
```

#### Keywords

Keywords start with `:` and evaluate to themselves:

```
arli> :hello
:hello
arli> define my-map {:a 1 :b 2}
{':a': 1, ':b': 2}
```

#### Sequence Operations

`map`, `filter`, and `reduce` work with lists:

```
arli> map (fn (x) * x 2) (list 1 2 3)
(2 4 6)
arli> filter (fn (x) > x 2) (list 1 2 3 4 5)
(3 4 5)
arli> reduce (fn (acc x) + acc x) 0 (list 1 2 3 4 5)
15
```

#### Pattern Matching with `match`

The `match` form selects the first clause whose pattern matches:

```
arli> match 3 (1 "one") (2 "two") (_ "other")
"other"
arli> match 1 (1 "one") (2 "two") (_ "other")
"one"
```

Use `_` as a wildcard that matches anything.

#### Documentation with `doc`

Store and retrieve documentation:

```
arli> doc add "Adds two numbers together"
"Adds two numbers together"
arli> doc add
"Adds two numbers together"
```

#### Numbers: Hex, Octal, Binary

```
arli> 0xFF
255
arli> 0o77
63
arli> 0b1010
10
```

#### Triple-Quoted Strings

```
arli> """hello
... world"""
"hello\nworld"
```

#### Result Type (Ok / Err)

The `Ok` and `Err` constructors create tagged values for error handling:

```
arli> define result (Ok 42)
(Ok 42)
arli> match result (Ok val) (print "got:" val) (Err msg) (print "error:" msg)
got: 42
```

Chain operations with `map-ok`, `and-then`, `or-else`:

```
arli> map-ok (Ok 10) (fn (x) * x 2)
(Ok 20)
arli> and-then (Ok 5) (fn (x) (Ok + x 1))
(Ok 6)
arli> and-then (Err "fail") (fn (x) (Ok + x 1))
(Err fail)
```

#### Testing with `assert`

Verify conditions inline:

```
arli> assert true "this passes"
True
arli> assert false "this fails"
Error: Assertion failed: "this fails"
```

---

### Chapter 7: F-Expressions â€” Functions That Don't Evaluate

**What you'll build:** Your own control structures — `unless`, `n-times`, `short-or` — that short-circuit, repeat, or skip evaluation just like built-in forms. You'll understand how Lisp-style fexprs give you macro-like power without a macro system.

Every function you've seen so far evaluates its arguments eagerly. When you write `+ 1 2`, both `1` and `2` are computed before `+` sees them. That's normally what you want.

But what if you want to write your own `if`? Or your own `while`? Or a logging wrapper that inspects expressions without evaluating them?

In most languages, you can't â€” control structures are built into the language. But arli gives you **f-expressions** (fexprs): user-defined functions that receive their arguments **unevaluated**.

#### defn-fexpr: Creating an F-Expression

`defn-fexpr` has arity 3, just like `defn`:

```
arli> defn-fexpr my-if (c t e) ...
```

The body receives `c`, `t`, and `e` as raw, unevaluated forms (AST nodes). To evaluate them, use the `eval` builtin:

```
arli> defn-fexpr my-if (c t e)
...   if (eval c) (eval t) (eval e)
<fexpr my-if arity=3>

arli> my-if true "yes" "no"
"yes"

arli> my-if false "yes" "no"
"no"
```

No parentheses needed â€” `my-if` has arity 3, just like `if`. The difference is that `if` is built into the language, while `my-if` is defined entirely in arli code.

#### Why This Matters: Short-Circuit Evaluation

With normal functions, all arguments are evaluated before the call. This means you can't write a short-circuit `or`:

```
arli> define side-effect (fn (x) (do print "computing..." x))
<fn side-effect arity=1>

arli> or true side-effect 42   ;; side-effect 42 is still evaluated!
"computing..."
true
```

With an fexpr, you control evaluation:

```
arli> defn-fexpr short-or (a b)
...   let ((av (eval a)))
...       if av av (eval b)
<fexpr short-or arity=2>

arli> short-or true (side-effect 42)
true                       ;; no "computing..." â€” side-effect was never called

arli> short-or false (side-effect 42)
"computing..."
42                         ;; only evaluates the second argument when needed
```

This is how Lisp macros work, but fexprs are **runtime** â€” no compile-time macro expansion step needed.

#### Inspecting Raw Forms

Since fexprs receive unevaluated arguments, you can inspect the AST of the caller's code:

```
arli> defn-fexpr show (x) x
<fexpr show arity=1>

arli> show + 1 * 2 3
(+ 1 (* 2 3))
```

This returns the expression `+ 1 * 2 3` as a data structure, not the computed value `7`. You can examine the structure, transform it, or conditionally evaluate parts of it.

#### Multiple Body Expressions

Like `defn`, the body of `defn-fexpr` is a single expression. Use `(do ...)` for multiple:

```
arli> defn-fexpr log-if (c t e)
...   (do
...       print "condition:" c
...       print "evaluated:" (eval c)
...       if (eval c) (eval t) (eval e))
<fexpr log-if arity=3>

arli> log-if true "yes" "no"
"condition:" true
"evaluated:" true
"yes"
```

#### The eval Builtin

`eval` has arity 1 and evaluates a form in the current environment. You can use it outside fexprs too:

```
arli> define expr (quote + 1 2)
(+ 1 2)

arli> eval expr
3
```

In fexprs, `eval` is how you selectively evaluate arguments. An argument you don't `eval` stays as a raw form â€” you can pass it around, inspect it, or discard it.

#### Important Caveats

**1. Name conflicts**: Parameter names must not match registered operator names. If you name a parameter `cond` (arity 1), the parser will try to consume an argument after it:

```
;; BAD â€” 'cond' has arity 1, parser consumes ')'
(defn-fexpr bad (cond body) ...)

;; GOOD â€” use unique names
(defn-fexpr good (cnd bod) ...)
```

**2. Symbol capture**: Raw forms contain symbols from the caller's scope. Passing raw forms between fexprs can cause infinite recursion if the raw form references a parameter that gets rebound. Use `eval` to resolve forms before passing.

**3. No variadic fexprs**: Like all arli functions, fexprs have a fixed arity. Variadic fexprs require parentheses.

#### F-Expressions vs Regular Functions

| Aspect | Regular Function | F-Expression |
|--------|-----------------|--------------|
| Arguments | Evaluated eagerly | Passed as raw forms |
| Custom control flow | Not possible | Full control via `eval` |
| Introspection | No | Can inspect caller's AST |
| Performance | Optimal | Deferred eval overhead |
| Definition | `defn name (params) body` | `defn-fexpr name (params) body` |

---

### Chapter 8: Using Go From arli

**What you'll build:** A compiled Go binary that runs arli programs with access to Go's `fmt`, `math`, and `strings` packages — for when you need native performance.

Arli also runs on **Go** (`go/arli/`). The Go backend gives you access to Go's standard library through the same `.` operator.

#### Pre-registered Packages

The Go backend comes with commonly used packages pre-loaded:

```
arli> import fmt
<Go struct { Println func(...interface {}) (int, error); ... }>
arli> (. fmt Println "hello from Go!")
[hello from Go!]
```

Other available packages: `math`, `strings`, `os`.

#### Calling Go Functions

```
arli> import strings
arli> (. strings ToUpper "hello")
"HELLO"
arli> (. strings Join (list "a" "b" "c") ", ")
"a, b, c"
```

#### Using Go's Math

```
arli> import math
arli> . math Pi
3.141592653589793
```

#### Running the Go Backend

```
# Build the binary
cd go/arli
go build -o arli.exe .

# Run a file
./arli.exe myfile.arli

# REPL
./arli.exe
```

#### Adding a New Go Package

To add a new Go package, edit `go/arli/eval.go` and add it to the `goPackages` map:

```go
func init() {
    goPackages["fmt"] = fmtPackage
    goPackages["strings"] = stringsPackage
    goPackages["myapp"] = myappPackage  // Your custom package
}
```

Then define the package struct with the functions you want to expose.

#### Python vs Go Backend

| Feature | Python Backend | Go Backend |
|---------|---------------|------------|
| Run | `python -m arli` | `./arli.exe` |
| Speed | Interpreted | Compiled |
| Ecosystem | Any Python library | Pre-registered Go packages |
| Dynamic eval | `python "code"` | `go "code"` (placeholder) |
| Package loading | `import os` (dynamic) | Pre-registered only |
| Extension | Write Python + register | Write Go + rebuild |

---

### Chapter 9: The Stack in Practice

**What you'll build:** A deep understanding of how data flows through arli — using `/stack`, `pick`, and `roll` to inspect and manipulate accumulated results without naming intermediate values.

Every expression result in arli is pushed onto an explicit **data stack**. You can inspect it with `/stack`:

```
arli> + 1 2
3
arli> * 4 5
20
arli> /stack
Stack (2 items):
  0: 3
  1: 20
```

This stack is where results accumulate as expressions are evaluated left to right. It's also where arli's Forth-like stack words operate.

#### Stack Words

arli provides traditional Forth stack words, but with a twist: they take **explicit arguments** (because the parser consumes them according to arity), not implicit values from the stack:

```
dup a      ;; copy a:                  pushes a, then a
drop a     ;; discard a:               returns nil
over a b   ;; copy 2nd arg to top:     pushes a (3rd value)
```

In the REPL:

```
arli> dup 5
5
arli> /stack
Stack (2 items):
  0: 5
  1: 5

arli> drop 42
nil
arli> /stack
Stack is empty.
```

#### The Real Stack: Expression Results

In practice, the stack is most useful for **accumulating results** from sequential operations. This is the natural arity-driven style:

```
arli> * 2 + 3 4       ;; 2 * (3 + 4) = 14
14
```

The parser groups this as `* 2 (+ 3 4)`. Inside the evaluator:
1. `+ 3 4` computes 7, pushes it to the stack
2. `* 2` pops 2 and the result (7), computes 14, pushes it

Even without explicit stack words, arity-driven parsing **is** the stack style â€” data flows from inner expressions to outer ones, and results accumulate on the stack naturally.

#### pick and roll

For indexed stack access, arli provides `pick` and `roll`. These are the only stack words that work on values **already on the evaluator's stack** (rather than taking explicit arguments):

```
pick n   ;; copy nth element (0=top) to the top
roll n   ;; rotate nth element (0=top) to the top
```

`pick 0` is like `dup` (copy top), `pick 1` is like `over` (copy 2nd), `roll 1` is like `swap`, `roll 2` is like `rot`.

Since `pick` and `roll` are single-argument words (arity 1), their argument is the **index**, not the data. The data must already be on the stack from preceding expressions:

```
arli> 42
42
arli> pick 0            ;; copy top (index 0) â€” 42 must be on the stack
42
arli> /stack
Stack (3 items):
  0: 42
  1: 42
  2: 42
```

Each `pick`/`roll` adds one extra value to the stack (the return value is pushed by the evaluator). Keep this in mind when chaining stack operations.

#### The Key Insight

**arity-driven parsing** and **stack-based evaluation** are two sides of the same coin. Arity tells the parser how many arguments a function consumes. The stack is where those arguments live. The parser groups expressions into trees, and the evaluator walks the tree, pushing intermediate results onto the stack as it goes.

You don't need to actively manage the stack for most code â€” just write expressions naturally and let arity do the grouping. The stack is always there, accumulating results, ready for inspection with `/stack` when you need to debug or understand the flow.
---

### Chapter 10: How It All Works

**What you'll build:** A mental model of arli's internals — tokenization, arity-driven parsing, stack evaluation, and host language interop — so you can reason about what your code actually does.

Let me pull back the curtain and show you how arli works internally.

#### Step 1: Tokenization

The source text is broken into tokens: numbers, strings, symbols, and parentheses.

```
+ 1 * 2 3
-->
[Symbol("+"), Number(1), Symbol("*"), Number(2), Number(3)]
```

#### Step 2: Arity-Driven Parsing

The parser uses an `ArityTable` â€” a mapping from symbol names to their argument counts. When it encounters a symbol with known arity N, it consumes the next N expressions as arguments:

```
[Symbol("+"), Number(1), Symbol("*"), Number(2), Number(3)]
-->
["+" is arity 2, so consume 1 and "* 2 3"]
--> 
["*" is arity 2, so consume 2 and 3]
-->
["+" , 1, ["*", 2, 3]]
```

This produces a tree: `[+, 1, [*, 2, 3]]`.

When a `(` is encountered, the parser switches to "flat list mode." Inside parentheses, the first element is treated as the operator and subsequent elements are parsed with arity-driven rules. This gives you the best of both worlds: arity-driven terseness with parenthesized explicitness when you need it.

#### Step 3: Evaluation

The evaluator walks the tree. For each list:
1. Evaluate the head (first element) to get a function
2. Evaluate the arguments
3. Apply the function to the arguments

The result is pushed onto the data stack (the Forth-like part).

Special forms like `if`, `define`, `while`, and `let` have custom evaluation rules â€” they don't evaluate all their arguments eagerly. `if` evaluates only the matching branch. `define` doesn't evaluate its first argument (the name).

#### Step 4: Python Interop

When you write `import os`, arli calls Python's `importlib.import_module("os")`. When you write `. os getcwd`, arli calls Python's `getattr(os, "getcwd")`. When you call a Python function from arli, it's a direct Python function call â€” no wrapping, no marshaling, no overhead.

The `python` special form uses Python's `eval()` function with the current arli environment as local variables. It's an escape hatch that gives you full access to Python when you need it.

---

### Chapter 11: Stack Reflection and Self-Modifying Code

**What you'll build:** A program that can inspect its own stack, transform it with list operations, and even rewrite its own code using the exec stack — the foundation for genetic programming and self-optimizing systems.

So far, the stack has been a behind-the-scenes mechanism â€” results accumulate, words like `dup` and `swap` rearrange them, and `/stack` lets you peek. But what if you could **capture the stack as data**, manipulate it with list operations, and put it back? And what if there was a **second stack** â€” one that holds code instead of data â€” so programs can rewrite themselves during execution?

This is what arli's **stack reflection** and **exec stack** provide. They're inspired by the [Push programming language](http://faculty.hampshire.edu/lspector/push.html), where programs live on stacks and self-modification is the default.

---

#### 11.1 Data Stack Reflection

Two builtins expose the data stack as a value:

**`stack` (arity 0)** â€” pushes a *copy* of the current data stack as a list.

```
arli> 1 2 3
3
arli> stack
(1 2 3)
arli> /stack
Stack (4 items):
  0: 1
  1: 2
  2: 3
  3: (1 2 3)
```

The snapshot is a regular arli list. You can filter it, map over it, cons to it â€” anything you can do to a list:

```
arli> 1 "hello" 2 "world" 3
3
arli> stack
(1 "hello" 2 "world" 3)
arli> filter number?
(1 2 3)
```

**`stack!` (arity 1)** â€” replaces the entire data stack with a list. Takes the new stack as an argument. Returns `None` (so exec doesn't push an extra value).

```
arli> stack! (list 10 20 30)
arli> /stack
Stack (3 items):
  0: 10
  1: 20
  2: 30
```

Pass `nil` to clear the stack:

```
arli> 1 2 3
3
arli> stack! nil
arli> /stack
Stack is empty.
```

##### Practical: Filter the Stack In Place

The real power comes from combining `stack` and `stack!`. Capture the current stack, transform it, and replace:

```clojure
;; Keep only numbers from the current stack
(let ((s stack))
    (stack! (filter (fn (x) number? x) s)))
```

Step by step:
1. `stack` captures `[1, "hello", 2, "world", 3]` and binds it to `s`
2. `(filter (fn (x) number? x) s)` keeps only numbers: `[1, 2, 3]`
3. `stack!` replaces the evaluator's entire stack with `[1, 2, 3]`

---

#### 11.2 The Exec Stack

If the data stack is where *values* live, the **exec stack** is where *code* lives. It's a second stack in the evaluator that holds forms (lists, symbols, literals) waiting to be executed.

The exec stack operations give you direct control over **what runs next**:

| Builtin | Arity | Description |
|---------|-------|-------------|
| `exec-stack` | 0 | Push a copy of the exec stack to the data stack |
| `exec!` | 1 | Replace the exec stack with a list |
| `exec-push` | 1 | Push a form onto the exec stack |
| `exec-pop` | 0 | Pop the top of the exec stack to the data stack |
| `exec-depth` | 0 | Push the exec stack depth to the data stack |
| `exec-step` | 0 | Pop and evaluate one form from the exec stack |
| `(exec)` | -1 | Process the entire exec stack until empty |

##### 11.2.1 Pushing and Stepping

`exec-push` puts a form onto the exec stack. `exec-step` pops and evaluates one form:

```
arli> exec-push (quote (+ 1 2))    ;; push the form (+ 1 2) onto exec stack
arli> exec-depth                   ;; how many items on exec?
1
arli> exec-step                    ;; pop and evaluate (+ 1 2)
3
arli> /stack
Stack (1 items):
  0: 3
```

`exec-push` returns `None` (nothing added to data stack). `exec-step` evaluates the form normally and pushes its result to the data stack.

##### 11.2.2 The Push Interpreter: (exec)

`(exec)` is the heart of the Push-style system. It processes the exec stack until empty. For each item popped:

- **If it's a list**: each element is pushed back onto the exec stack in forward order. The first element ends up at the bottom (executed last).
- **If it's a Symbol**: the function is looked up, arguments are popped from the **data stack** according to the function's arity, and the result is pushed to the data stack.
- **If it's a literal (number, string, etc.)**: it's pushed directly to the data stack.

This is a fundamentally different execution model from normal arli. Instead of the parser grouping arguments by arity, functions **pull their arguments from the data stack at runtime**.

Here's how Push-style arithmetic works:

```
;; Push the function symbol, then the arguments, then run
exec-push (quote +)    ;; push the + symbol (not evaluated)
exec-push 2            ;; push a literal (goes to data stack via (exec))
exec-push 1
(exec)
```

When `(exec)` runs:

1. Pop `1` â†’ literal â†’ push to data stack â†’ `[1]`
2. Pop `2` â†’ literal â†’ push to data stack â†’ `[1, 2]`
3. Pop `+` â†’ Symbol â†’ lookup â†’ Builtin, arity 2 â†’ pop 2 from data stack â†’ `1 + 2 = 3` â†’ push to data stack â†’ `[3]`

Result: `[3]`. This is the same as `+ 1 2` in normal arli, but achieved by pushing code onto the exec stack and letting it run.

Nested expressions work the same way:

```
;; Compute (1 + 2) * 4 in Push style
exec-push (quote *)
exec-push 4
exec-push (quote +)
exec-push 2
exec-push 1
(exec)
```

`(exec)` processes: `1` â†’ data, `2` â†’ data, `+` â†’ pop 2 â†’ `3` â†’ data, `4` â†’ data, `*` â†’ pop 2 â†’ `12` â†’ data. Result: `[12]`.

##### 11.2.3 How List Expansion Works

When `(exec)` encounters a list on the exec stack, it expands it by pushing each element back. This means lists act as **program fragments** that get unfolded during execution.

For example, pushing `(quote (+ 1 2))` as a single item:

```
exec-push (quote (+ 1 2))
(exec)
```

`(exec)` pops `(+ 1 2)` (a list), pushes each element back in order: `+`, then `1`, then `2`. Now the exec stack is `[+, 1, 2]` with `2` on top. Processing continues normally: `2` â†’ data, `1` â†’ data, `+` â†’ pop 2 â†’ `3`.

You can build lists dynamically and execute them:

```
;; Build a program at runtime and run it
(let ((prog (list (quote +) 1 2)))
    (exec! (list prog))      ;; put it on the exec stack as a single item
    (exec))                  ;; run it
```

##### 11.2.4 Exec Stack Reflection

Just like the data stack, the exec stack can be inspected and modified:

```
arli> exec-push 10
arli> exec-push 20
arli> exec-stack           ;; push a copy of exec stack to data stack
(10 20)
```

`exec!` replaces the entire exec stack:

```
arli> exec! (list (quote +) 100 200)
arli> exec-depth
3
arli> (exec)
300
```

`exec-pop` moves the top of the exec stack to the data stack:

```
arli> exec-push 99
arli> exec-pop
99
```

---

#### 11.3 Self-Modifying Code Patterns

The combination of `exec-push`, `exec-stack`, `exec!`, and `(exec)` enables true self-modifying code: programs that rewrite themselves during execution.

##### Pattern 1: A Loop That Re-pushes Itself

The simplest self-modifying pattern is a loop body that pushes itself back onto the exec stack:

```clojure
;; A loop that decrements a counter and re-pushes itself
;; Data stack starts with: n
(exec-push (quote
    (do
        dup                     ;; n n
        0 = not                 ;; n (n>0)?
        (if
            (do
                dup 1 -         ;; n-1 (decrement)
                ;; Re-push ourselves to continue the loop
                exec-push (quote
                    (do
                        dup 0 = not
                        (if
                            (do
                                dup 1 -
                                exec-push (quote ...))
                            (drop))))
                ;; Move the decremented value to data stack
                swap
                drop)
            (drop)))))         ;; n=0, we're done
```

This is verbose because we have to quote the loop body literaly. But it demonstrates the core idea: the program pushes a copy of itself onto the exec stack before finishing, creating a loop.

##### Pattern 2: Building Code From Data

More practical: build code on the data stack using list operations, then push to exec stack and run:

```clojure
;; Build a program dynamically
(let ((fn (quote +))        ;; pick a function
      (a 10)
      (b 20))
    ;; Build (fn a b) as a list
    (exec-push (list fn a b))
    (exec))                  ;; run it
;; data stack: [30]
```

##### Pattern 3: Self-Modifying Function

Define a function that modifies its own behavior:

```clojure
;; A function that counts how many times it's been called
(defn counter ()
    (let ((count stack))       ;; capture current stack
        (stack! (list 1))      ;; reset stack to [1]
        (if (= 0 count)        ;; first call?
            1
            (+ count 1))))

;; Each call reads the previous count, increments, and stores it
counter    ;; -> 1  (but wait, counter pops the stack...)
```

This is limited by the data stack being shared. A better pattern stores state on the exec stack:

```clojure
;; Store state on the exec stack
(defn counted-call ()
    (let ((state exec-stack))         ;; read exec stack
        ;; The exec stack has our state: (count next-form)
        ;; Pop and run the next form, then re-push with incremented count
        ...))
```

##### Pattern 4: Generative Programming

The most powerful pattern: generate code based on the current program state:

```clojure
;; A program that evolves
(defn evolve ()
    (let ((code exec-stack))
        ;; Analyze the remaining code on exec stack
        ;; Modify or extend it based on results so far
        ;; Push modified code back
        (exec! (transform code))))

;; Push initial code and evolve
exec-push (quote (initial-step))
exec-push (quote (evolve))
(exec)          ;; runs initial-step, then evolve modifies what's next
```

---

#### 11.4 Relationship to Push

arli's exec stack is inspired by the [Push programming language](http://faculty.hampshire.edu/lspector/push.html), designed for evolutionary computation. In Push:

| Push | arli | Description |
|------|------|-------------|
| `code.stack` | `exec-stack` | Push copy of exec stack to data stack |
| `code.>` | `exec-push` | Push a value onto the exec stack |
| `code.pop` | `exec-pop` | Pop exec stack to data stack |
| `code.depth` | `exec-depth` | Depth of exec stack |
| `exec.do` | `(exec)` | Process exec stack until empty |

The main difference: Push has **typed stacks** (integer stack, float stack, boolean stack, etc.) and operations dispatch based on the top of each stack. arli keeps a single data stack for simplicity, but the exec stack follows Push's model: code is data, programs live on the exec stack, and self-modification happens by manipulating the exec stack at runtime.

---

#### 11.5 When to Use Stack Reflection

Stack reflection and the exec stack are advanced features. Use them when:

1. **You need self-modifying code** â€” programs that rewrite themselves based on runtime conditions
2. **You want to generate code dynamically** â€” building and executing programs from data
3. **You're exploring evolutionary computation** â€” generating, mutating, and selecting programs
4. **You want fine-grained control over evaluation order** â€” the exec stack lets you sequence operations explicitly

For everyday arli programming, the normal arity-driven style (`+ 1 2`, `if cond then else`, `defn name (params) body`) is cleaner and faster. Stack reflection is a power tool for when you need to break the normal rules.

---



---

## Part 2: Common Lisp Cookbook

Task-oriented recipes for getting things done with arli.

---

### Recipe 1: Read a File

```clojure
import pathlib
define root . pathlib Path "."
define content . (root / "data.txt") read_text
print content
```

### Recipe 2: Write a File

```clojure
import pathlib
define root . pathlib Path "."
(. (root / "output.txt") write_text "hello from arli")
```

### Recipe 3: Parse JSON

```clojure
import json
define data host "{'name': 'arli', 'version': 1}"
define parsed . json loads data
. parsed name
```

### Recipe 4: Make an HTTP Request

```clojure
import urllib.request
define req . urllib.request Request "https://api.github.com/repos/arli/arli"
define resp . urllib.request urlopen req
define data . json loads (. resp read decode "utf-8")
. data stargazers_count
```

### Recipe 5: Work with Dates

```clojure
import datetime
define now . datetime datetime now
. now year
. now strftime "%Y-%m-%d"
```

### Recipe 6: Use Regular Expressions

```clojure
import re
define text "The quick brown fox"
define match . re search "brown|red" text
if match (. match group) "no match"
```

### Recipe 7: List Directory Contents

```clojure
import pathlib
define root . pathlib Path "."
for f root iterdir
    if (. f suffix == ".arli")
        print (. f name)
```

### Recipe 8: Build a CLI Tool (wc.arli)

```clojure
;; word count utility
import pathlib, sys
define filename (. sys argv __getitem__ 1)
define text . (pathlib Path filename) read_text
(do
    print "Lines:" (len (. text splitlines))
    print "Words:" (len (. text split))
    print "Chars:" (len text))
```

Run: `python -m arli wc.arli myfile.txt`

### Recipe 9: Error Handling with Ok/Err

```clojure
define safe-divide
    fn (a b)
        if (= b 0) (Err "division by zero")
            (Ok / a b)

define process
    fn (x)
        and-then (safe-divide 100 x)
            fn (val) (Ok * val 2))

(process 25)  ;; (Ok 8)
(process 0)   ;; (Err "division by zero")
```

### Recipe 10: Data Pipeline

```clojure
reduce (fn (acc x) + acc x) 0
    map (fn (x) * x 2)
        filter (fn (x) = 0 % x 2)
            list 1 2 3 4 5 6  ;; -> 24
```

### Recipe 11: Postfix Calculator (Exec Stack)

```clojure
define calc
    fn (expr)
        (do exec! expr (exec))

calc (list 3 4 (quote +) 2 (quote *))  ;; -> 14
```

### Recipe 12: Benchmark

```clojure
import time
define bench
    fn (fn-to-call repeats)
        (do
            define start . time time
            define i 0
            while (< i repeats)
                (do fn-to-call set! i + i 1)
            - . time time start)

print bench (fn () + 1 1) 10000
```

### Recipe 13: String Formatting

```clojure
define name "arli"
define version 1
print (host "f'Hello from {name} v{version}!'")
```

### Recipe 14: Environment Variables

```clojure
import os
define home . os environ get "HOME"
define path . os environ get "PATH"
```

### Recipe 15: Process CSV

```clojure
import csv, io
define data "name,age\\nAlice,30\\nBob,25"
define reader . csv DictReader (. io StringIO data)
for row reader
    print "Name:" (. row get "name")
```

### Recipe 16: Custom Control Flow (F-Expressions)

```clojure
defn-fexpr unless (cond body)
    if (eval cond) nil (eval body)

unless false (print "this runs")

defn-fexpr n-times (n body)
    let ((i 0))
        while (< i (eval n))
            (do (eval body) set! i + i 1)

n-times 3 (print "hello")
```

### Recipe 17: Cross-Backend Code

```clojure
;; Works on Python and JS backends
defn fib (n)
    if (< n 2) n
        + fib (- n 1) fib (- n 2)

print "fib 10:" fib 10

;; host adapts per backend:
;;   Python: host "repr(42)"
;;   JS:     host "JSON.stringify(42)"
```


### Chapter 12: Where To Go From Here

You now know enough to write real programs in arli. Here's what I'd suggest:

1. **Play with the REPL**. The `/stack`, `/env`, `/arity`, and `/debug` commands are your friends.

2. **Write a script**. Save some arli code in a `.arli` file and run it with `python -m arli myfile.arli`.

3. **Use Python libraries**. Try `import json`, `import re`, `import collections`. The entire Python standard library is at your fingertips.

4. **Explore exec stack programming**. Try building a program with `exec-push` and running it with `(exec)`. Then try having the program modify itself during execution.

5. **Build something real**. A file renamer. A JSON processor. A web scraper using `import requests`. arli is a scripting language â€” use it like one.

The complete reference is in [SPEC.md](SPEC.md). The source code is in `src/arli/`. It's about 600 lines of Python â€” read it, modify it, make it your own.

And remember: parentheses aren't the enemy. They're a tool. arli just doesn't need them as often.
