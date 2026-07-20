# Practical Hya: A Lisp Without Parentheses

## Or, How I Learned to Stop Worrying and Love Arity

### Introduction: Why Another Language?

You know the complaint. It's the same one that's been leveled at Lisp since McCarthy first scrawled parentheses on a blackboard in 1958: "Too many parens."

But here's the thing — the parentheses in Lisp aren't noise. They're structure. They tell you (and the computer) where one expression ends and another begins. The problem isn't that Lisp has parentheses. The problem is that **every expression** needs them, even when the structure is already obvious.

Consider this:

```lisp
(+ 1 (* 2 3))
```

You read this as: "add 1 to the result of multiplying 2 by 3." The parentheses around `(* 2 3)` tell you it's a sub-expression. But look at `+` — it takes two arguments. Everyone knows `+` takes two arguments. Why do we need the outer parentheses to tell us that?

What if we wrote this instead:

```
+ 1 * 2 3
```

The `+` takes two arguments. The first is `1`. The second is `* 2 3`, which is itself an expression where `*` takes two arguments: `2` and `3`. The structure is encoded in the **arity** of the operators, not in parentheses.

This is the core idea behind **Hya**: when a function's arity is known, you don't need parentheses. When it's unknown (variadic), you still use them as a fallback.

But Hya isn't just "Lisp with fewer parens." It's also **stack-based** — inspired by Forth — so data flows through an explicit stack. Functions push and pop values. It's a different way of thinking about computation, and once it clicks, it changes how you structure programs.

And finally, Hya sits **on top of Python**. Every Python module, function, and object is accessible from Hya. You can `import os`, call `os.getcwd()`, or `eval("python code")` directly. This means Hya isn't a toy — it's a scripting language with access to the entire Python ecosystem.

---

### Chapter 1: The Simplest Things

Let's start with arithmetic. Fire up the Hya REPL:

```
$ python -m hya
Hya v0.1.0
Arity-driven Lisp with Forth-like stack operations
Type 'help' for commands, 'exit' or Ctrl+C to quit

hya>
```

Type `+ 1 2` and press Enter:

```
hya> + 1 2
3
```

No parentheses. Just `+`, then `1`, then `2`. The `+` function has **arity 2** — it takes two arguments. The parser sees `+`, knows it needs two things, and consumes `1` and `2` as its arguments. The evaluator adds them, and pushes `3` onto the stack. The REPL prints the top of the stack.

Nesting works the same way:

```
hya> * + 2 3 4
20
```

This is `(* (+ 2 3) 4)`. The `*` has arity 2. Its first argument is `+ 2 3` (because `+` has arity 2 and consumes `2` and `3`). Its second argument is `4`. The evaluator computes `(2 + 3) * 4 = 20`.

You can chain as deep as you want:

```
hya> / - 10 2 3
2.666...
```

This is `(/ (- 10 2) 3)`. The `-` subtracts `2` from `10`, giving `8`. Then `/` divides `8` by `3`.

**The rule**: every function has a known arity (argument count). The parser uses this to group expressions automatically. No parentheses needed.

But what about functions that take a variable number of arguments? Like `list`, which can take any number of items? For those, you **do** use parentheses:

```
hya> (list 1 2 3)
(1 2 3)
```

The parentheses tell the parser "everything inside is one expression." The `list` function receives all three items as its arguments.

This is the complete syntax of Hya:
1. If a function's arity is **known**, write it without parentheses: `+ 1 2`
2. If a function's arity is **unknown** (variadic), use parentheses: `(list 1 2 3)`
3. If you want to be explicit, use parentheses anywhere: `(+ 1 2)` is the same as `+ 1 2`

All of Hya's built-in operators have known arities. **None of them need parentheses.** Let's see what that looks like.

---

### Chapter 2: The Core Operators

#### if: The Three-Way Conditional

In most Lisps, you write `(if condition then else)`. In Hya, `if` has arity 3, so:

```
hya> if (> 5 3) "yes" "no"
"yes"
hya> if nil "yes" "no"
"no"
```

The `if` takes three arguments: a condition, a then-value, and an else-value. It evaluates the condition first. If truthy (anything except `nil` and `false`), it evaluates and returns the then-value. Otherwise, the else-value.

The condition itself is arity-driven: `> 5 3` is `(> 5 3)`, which returns `true` because `5 > 3`. No parens needed anywhere.

#### define: Giving Things Names

`define` has arity 2:

```
hya> define x 42
42
hya> x
42
```

You can define a name to any value, including the result of an expression:

```
hya> define y + 1 2
3
hya> y
3
```

#### set!: Changing Things

`set!` has arity 2 — it mutates an existing binding:

```
hya> define x 10
10
hya> set! x 20
20
hya> x
20
```

#### quote: Don't Evaluate This

`quote` has arity 1. It returns its argument unevaluated:

```
hya> quote + 1 2
(+ 1 2)
```

Instead of computing `+ 1 2` (which would be `3`), `quote` returns the expression itself. This is how we work with code as data.

#### do: Sequencing

`do` has arity 2. It evaluates two expressions and returns the second:

```
hya> do print "hello" + 1 2
"hello"
3
```

The first expression `print "hello"` prints "hello" as a side effect. The second expression `+ 1 2` produces `3`, which becomes the result of the `do`.

If you need to sequence more than two expressions, chain them: `do a do b c`. Or use parentheses: `(do a b c)` — inside parentheses, the `do` handler processes all expressions.

I know, I know — this looks weird at first. But it's consistent with the rule: every operator has a fixed arity. You'll get used to it.

#### while: Looping

`while` has arity 2. It takes a condition and a body:

```
hya> define i 0
0
hya> while (< i 5) do print i set! i + i 1
0
1
2
3
4
```

The `while` evaluates the condition `(< i 5)`. If truthy, it evaluates the body `(do print i set! i + i 1)`. The `do` sequences two expressions: print the current value of `i`, then increment it. This repeats until `i` reaches `5`.

#### let: Local Bindings

`let` has arity 2. It takes a bindings list and a body:

```
hya> let ((x 5) (y 3)) + x y
8
```

The bindings list `((x 5) (y 3))` binds `x` to `5` and `y` to `3` in a new lexical scope. The body `+ x y` evaluates to `8`. The bindings are only visible inside the `let`.

#### for: Iteration

`for` has arity 3: a variable, a list, and a body:

```
hya> for x (list 1 2 3) print x
1
2
3
```

The `for` binds `x` to each element of the list and evaluates the body. It's a simple, clean loop.

#### cond: Multi-way Branching

`cond` has arity 1. It takes a list of (test result) pairs:

```
hya> define x 5
5
hya> cond ((> x 10) "big" (< x 3) "small" true "medium")
"medium"
```

The list contains pairs: `(> x 10)` paired with `"big"`, `(< x 3)` paired with `"small"`, and `true` (always matches) paired with `"medium"`. `cond` walks through the pairs, evaluates each test, and returns the result of the first truthy test.

---

### Chapter 3: The Stack

Now let's talk about the stack. Hya isn't just a Lisp — it's a **Forth-like** Lisp. This means there's an explicit data stack that all operations push to and pop from.

When you evaluate an expression, the result is pushed onto the stack:

```
hya> + 1 2
3
```

The stack now contains `[3]`.

But you can also inspect the stack with `/stack`:

```
hya> + 1 2
3
hya> * 3 4
12
hya> /stack
Stack (2 items):
  0: 3
  1: 12
```

Both results are on the stack. The most recent is on top (index 1).

Hya provides Forth-like stack manipulation words with known arities:

**`dup` (arity 1)** — duplicates the top of the stack:
```
hya> dup 5
5
hya> /stack
Stack (2 items):
  0: 5
  1: 5
```

**`swap` (arity 2)** — swaps the top two items:
```
hya> swap 1 2
2
1
hya> /stack
Stack (2 items):
  0: 2
  1: 1
```

**`drop` (arity 1)** — discards the top of the stack:
```
hya> drop 42
nil
hya> /stack
Stack is empty.
```

**`over` (arity 2)** — duplicates the second item:
```
hya> over 1 2
1
hya> /stack
Stack (3 items):
  0: 1
  1: 2
  2: 1
```

**`rot` (arity 3)** — rotates the top three items:
```
hya> rot 1 2 3
2
3
1
hya> /stack
Stack (3 items):
  0: 2
  1: 3
  2: 1
```

**`nip` (arity 2)** — drops the second item:
```
hya> nip 1 2
2
hya> /stack
Stack (1 items):
  0: 2
```

**`tuck` (arity 2)** — duplicates the top under the second:
```
hya> tuck 1 2
2
1
2
hya> /stack
Stack (3 items):
  0: 2
  1: 1
  2: 2
```

These stack operations are the building blocks of concatenative programming. Instead of naming intermediate values with variables, you manipulate them directly on the stack. It takes some getting used to, but it leads to remarkably concise code.

---

### Chapter 4: Defining Functions

#### defn: The Standard Way

To define a function, use `defn`:

```
hya> (defn add (x y) (+ x y))
<fn add arity=2>
hya> add 1 2
3
```

The syntax is `(defn name (params) body...)`. The parentheses around the whole form are necessary because `defn` has an unknown structure (it takes a name, a parameter list, and a body). But inside, everything is arity-driven.

When you define `add` with parameters `(x y)`, Hya registers `add` with arity 2. From that point on, `add 1 2` works without parentheses — the parser knows `add` needs two arguments and consumes them automatically.

#### defn-rec: Recursive Functions

For recursive functions, use `defn-rec`:

```
hya> (defn-rec fact (n)
...   (if (= n 0)
...       1
...       (* n fact (- n 1))))
<fn fact arity=1>
hya> fact 5
120
```

The difference between `defn` and `defn-rec` is that `defn-rec` registers the function's arity **before** parsing the body. This means the function can call itself recursively — when the parser encounters `fact` in the body, it already knows `fact` takes 1 argument.

Without `defn-rec`, you'd get an error because `fact` wouldn't be in the arity table when the body is parsed.

#### fn: Anonymous Functions

Use `fn` to create a lambda:

```
hya> define double (fn (x) (* x 2))
<fn anon arity=1>
hya> double 5
10
```

The `fn` form creates an anonymous function. When you `define` it, Hya detects it's a function and registers its arity automatically.

---

### Chapter 5: Using Python From Hya

The whole reason Hya exists on top of Python is to give you access to Python's libraries. Let's see how it works.

#### import: Pulling in Python Modules

`import` has arity 1:

```
hya> import os
<module 'os' from '...'>
```

The Python `os` module is now bound to the name `os` in Hya's environment.

#### . (dot): Accessing Attributes

The `.` operator has arity 2. It gets an attribute from an object:

```
hya> . os sep
"\\"
```

This is `getattr(os, 'sep')` — it returns the path separator character.

For chaining attribute access and method calls, use parentheses:

```
hya> (. os path join "a" "b")
"a\\b"
```

Inside the parentheses, `.` chains through attributes. It starts with `os`, gets `path` (giving `os.path`), gets `join` (giving `os.path.join`), then calls it with `"a"` and `"b"`. The result is `"a\\b"` (or `"a/b"` on Linux).

You can assign results to names:

```
hya> define p . os path
<module 'ntpath' from '...'>
hya> define mydir . p join "home" "user"
"home\\user"
```

#### The python Special Form

For arbitrary Python evaluation, use `python` with arity 1:

```
hya> python "repr(42)"
"42"
```

This evaluates the string as a Python expression. The environment's bindings are available as local variables:

```
hya> define x 42
42
hya> python "x * 2"
84
```

#### Putting It All Together

Let's write a Hya script that uses Python's `pathlib` library:

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
define data python "{'name': 'Hya', 'type': 'language'}"
define parsed . json loads data
. parsed name
```

This is the power of Hya: Lisp-like syntax with Forth-like stack semantics and direct access to the entire Python ecosystem. No FFI. No bindings. Just `import` and go.

---

### Chapter 6: The Stack in Practice

Now let's see how the stack influences real code organization. Here's a Fibonacci function written two ways:

**With variables (Lisp style)**:
```clojure
(defn fib-iter (n)
    (let ((a 0) (b 1) (i 0))
        (while (< i n)
            (let ((temp b))
                (do
                    (set! b + a b)
                    (set! a temp)
                    (set! i + i 1))))
        a))
```

**With stack operations (Forth style)**:
```clojure
;; This takes some getting used to...
```

The stack style takes practice. Start with the Lisp style (variables and `let`) — it's familiar. As you get comfortable, you'll find yourself naturally using `dup`, `swap`, and `drop` to move data around without naming everything.

The key insight is that **arity-driven parsing** and **stack-based evaluation** are two sides of the same coin. Arity tells the parser how many arguments a function consumes. The stack is where those arguments live. The parser groups, the evaluator executes, and the stack carries data between them.

---

### Chapter 7: How It All Works

Let me pull back the curtain and show you how Hya works internally.

#### Step 1: Tokenization

The source text is broken into tokens: numbers, strings, symbols, and parentheses.

```
+ 1 * 2 3
-->
[Symbol("+"), Number(1), Symbol("*"), Number(2), Number(3)]
```

#### Step 2: Arity-Driven Parsing

The parser uses an `ArityTable` — a mapping from symbol names to their argument counts. When it encounters a symbol with known arity N, it consumes the next N expressions as arguments:

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

Special forms like `if`, `define`, `while`, and `let` have custom evaluation rules — they don't evaluate all their arguments eagerly. `if` evaluates only the matching branch. `define` doesn't evaluate its first argument (the name).

#### Step 4: Python Interop

When you write `import os`, Hya calls Python's `importlib.import_module("os")`. When you write `. os getcwd`, Hya calls Python's `getattr(os, "getcwd")`. When you call a Python function from Hya, it's a direct Python function call — no wrapping, no marshaling, no overhead.

The `python` special form uses Python's `eval()` function with the current Hya environment as local variables. It's an escape hatch that gives you full access to Python when you need it.

---

### Chapter 8: Where To Go From Here

You now know enough to write real programs in Hya. Here's what I'd suggest:

1. **Play with the REPL**. The `/stack`, `/env`, `/arity`, and `/debug` commands are your friends.

2. **Write a script**. Save some Hya code in a `.hya` file and run it with `python -m hya myfile.hya`.

3. **Use Python libraries**. Try `import json`, `import re`, `import collections`. The entire Python standard library is at your fingertips.

4. **Write a function that uses the stack**. Try computing `a * (b + c)` using `swap`, `dup`, and `drop` instead of variables.

5. **Build something real**. A file renamer. A JSON processor. A web scraper using `import requests`. Hya is a scripting language — use it like one.

The complete reference is in [SPEC.md](SPEC.md). The source code is in `src/hya/`. It's about 600 lines of Python — read it, modify it, make it your own.

And remember: parentheses aren't the enemy. They're a tool. Hya just doesn't need them as often.
