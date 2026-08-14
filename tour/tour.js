/**
 * A Tour of Arli — Interactive Tour Engine
 * Powered by client-side ES Modules from ../js/arli/src/arli-browser.js
 */

import { Evaluator, arliRepr } from '../js/arli/src/arli-browser.js';

// ---------------------------------------------------------------------------
// Tour Curriculum Database (20 Modules)
// ---------------------------------------------------------------------------

const LESSONS = [
  {
    id: 'welcome',
    title: '1. Welcome to Arli',
    article: `
      <h2>Welcome to Arli</h2>
      <p><strong>Arli</strong> (Arity-driven Lisp) is a Forth-like Lisp dialect designed around a simple yet powerful idea: <strong>arity-driven parenthesis elimination</strong>.</p>
      <p>In traditional Lisps, every function application requires parentheses: <code>(+ 1 (* 2 3))</code>. In Arli, if the arity (number of arguments) of an operator is known, you don't need parentheses at all!</p>
      <p>Arli combines:</p>
      <ul>
        <li><strong>Arity-Driven S-Expressions</strong>: Zero-parenthesis prefix code.</li>
        <li><strong>Forth Data Stack</strong>: Native stack operations (<code>dup</code>, <code>swap</code>, <code>rot</code>).</li>
        <li><strong>Push-style Exec Stack</strong>: Self-modifying code & metaprogramming.</li>
        <li><strong>Lexical Closures</strong>: First-class functions and lexical scoping.</li>
      </ul>
      <div class="info-box">
        <p><strong>Try it!</strong> Click <strong>Run</strong> (or press <code>Ctrl+Enter</code>) on the right to evaluate the code in your browser.</p>
      </div>
    `,
    code: `;; Welcome to Arli!
print "Welcome to Arli!"

;; Arithmetic without parentheses:
+ 10 20
`
  },
  {
    id: 'arithmetic',
    title: '2. Prefix Arithmetic (No Parens)',
    article: `
      <h2>Prefix Arithmetic Without Parens</h2>
      <p>All basic arithmetic and comparison operators have a fixed arity of <strong>2</strong>:</p>
      <ul>
        <li><code>+</code>, <code>-</code>, <code>*</code>, <code>/</code>, <code>//</code> (integer div), <code>%</code> (modulo)</li>
        <li><code>=</code>, <code><</code>, <code>></code>, <code><=</code>, <code>>=</code>, <code>!=</code></li>
        <li><code>neg</code> (arity 1)</li>
      </ul>
      <p>When the parser encounters <code>+ 1 * 2 3</code>, it knows <code>+</code> needs 2 arguments: the first is <code>1</code>, and the second is <code>* 2 3</code> (which itself consumes <code>2</code> and <code>3</code>). It automatically builds the tree <code>(+ 1 (* 2 3))</code> with zero parentheses!</p>
      <div class="info-box">
        <p><strong>Tip:</strong> Expressions parse from left to right, recursively consuming arguments based on known operator arity.</p>
      </div>
    `,
    code: `;; Traditional Lisp: (+ 1 (* 2 3))
;; Arli:
+ 1 * 2 3

;; Nested operations:
* + 10 20 + 3 7

;; Negation (arity 1):
neg + 40 2
`
  },
  {
    id: 'variables',
    title: '3. Variables with define',
    article: `
      <h2>Variables with define</h2>
      <p>Use <code>define</code> (arity 2) to bind a name to a value in the current environment:</p>
      <pre><code>define x 42
define total + x 10</code></pre>
      <p>Because <code>define</code> has arity 2, it consumes the name and the value expression without outer parentheses.</p>
      <p>To mutate an existing variable, use <code>set!</code>:</p>
      <pre><code>set! x 100</code></pre>
    `,
    code: `define base 100
define rate 1.15

define total * base rate
print total

;; Mutate variable
set! base 200
* base rate
`
  },
  {
    id: 'functions',
    title: '4. Functions with defn',
    article: `
      <h2>Functions with defn</h2>
      <p>Define functions using <code>defn</code> (arity 3):</p>
      <pre><code>defn name (params) body</code></pre>
      <p>When you define a function with <code>defn</code>, its arity (number of parameters) is automatically registered. Once defined, calls to your function <strong>require no parentheses</strong>!</p>
      <div class="info-box">
        <p>Anonymous functions are created with <code>fn (params) body</code> (arity 2).</p>
      </div>
    `,
    code: `;; Define a 2-argument function (arity 2)
defn add (x y) + x y

;; Define a 1-argument function (arity 1)
defn square (x) * x x

;; Calls need zero parentheses:
square add 3 4
`
  },
  {
    id: 'recursion',
    title: '5. Recursion Without Parens',
    article: `
      <h2>Recursion Without Parens</h2>
      <p>In Arli, functions defined with <code>defn</code> or <code>defn-rec</code> register their arity into the parser's <code>ArityTable</code> <strong>before</strong> parsing the function body.</p>
      <p>This means recursive calls inside the function body immediately know their arity, allowing you to write recursive functions with <strong>zero outer parentheses</strong>!</p>
      <pre><code>defn fact (n)
    if (= n 0)
        1
        * n fact (- n 1)</code></pre>
    `,
    code: `;; Factorial without parentheses:
defn fact (n)
    if (= n 0)
        1
        * n fact - n 1

print fact 5

;; Fibonacci without parentheses:
defn-rec fib (n)
    if (= n 0)
        0
        if (= n 1)
            1
            + fib (- n 1) fib (- n 2)

print fib 10
`
  },
  {
    id: 'higher-order',
    title: '6. Higher-Order Functions & Dynamic Arity',
    article: `
      <h2>Higher-Order Functions & Dynamic Arity</h2>
      <p>Arli's parenthesis elimination is <strong>static (parse-time)</strong> based on known function names.</p>
      <p>When calling defined functions directly, their arity is known at parse time:</p>
      <pre><code>inc add 1 2   ;; statically parses as (inc (add 1 2)) -> 4
add inc 1 2   ;; statically parses as (add (inc 1) 2) -> 4</code></pre>
      <p>However, when functions are passed as parameters (e.g. <code>f</code>, <code>g</code>), their arity is <strong>dynamic and unknown at parse time</strong>. Therefore, dynamic higher-order calls <strong>require explicit parentheses</strong> to specify argument grouping:</p>
      <ul>
        <li>For <code>f(g(1, 2))</code>: <code>defn apply2 (f g) (f (g 1 2))</code></li>
        <li>For <code>f(g(1), 2)</code>: <code>defn apply2 (f g) (f (g 1) 2)</code></li>
      </ul>
      <div class="info-box">
        <p><strong>Passing Functions as Values:</strong> To pass an existing function by name without triggering its static arity consumer, quote it as a symbol (e.g., <code>'inc 'add</code>) or pass an anonymous closure <code>(fn (x) + x 1)</code>.</p>
      </div>
    `,
    code: `defn inc (x) + x 1
defn add (x y) + x y

;; Static known calls (no parens):
print inc add 1 2

;; Higher-order functions with dynamic parameters (parens required):
defn apply-composed (f g) (f (g 1 2))

;; Pass quoted symbols or anonymous closures as function values:
print apply-composed 'inc 'add
`
  },
  {
    id: 'conditionals',
    title: '7. Conditionals (if & cond)',
    article: `
      <h2>Conditionals: if and cond</h2>
      <p>The <code>if</code> special form has arity 3: <code>if condition then-expr else-expr</code>.</p>
      <p>For multi-branch conditionals, use <code>cond</code> with pairs of test/result clauses:</p>
      <pre><code>cond (
    (test1) result1
    (test2) result2
    true    default-result
)</code></pre>
    `,
    code: `define score 88

;; if (arity 3):
if (>= score 90) "Excellent" "Good"

;; cond multi-branch:
cond (
    (>= score 90) "Grade A"
    (>= score 80) "Grade B"
    (>= score 70) "Grade C"
    true          "Grade F"
)
`
  },
  {
    id: 'loops',
    title: '8. Loops (while & for)',
    article: `
      <h2>Loops: while and for</h2>
      <p>Arli provides structured loops:</p>
      <ul>
        <li><code>while cond body</code> (arity 2)</li>
        <li><code>for var list body</code> (arity 3)</li>
      </ul>
      <p>When you need multiple expressions inside a loop body, sequence them with <code>(do ...)</code>.</p>
    `,
    code: `;; while loop
define counter 0
while (< counter 4)
    (do
        (print counter)
        (set! counter + counter 1))

;; for loop over a list
for item [10 20 30]
    print * item 2
`
  },
  {
    id: 'let',
    title: '9. Local Scopes with let',
    article: `
      <h2>Local Scopes with let</h2>
      <p>Use <code>let</code> (arity 2) to establish local variable bindings that do not leak into outer scopes:</p>
      <pre><code>let ((var1 val1) (var2 val2)) body</code></pre>
      <p>Local bindings shadow outer bindings within the <code>let</code> block and are cleaned up on exit.</p>
    `,
    code: `define x 1000

let ((x 10) (y 20))
    (do
        (print "Inside let:")
        (print + x y))

print "Outside let:"
print x
`
  },
  {
    id: 'stack-ops',
    title: '10. Forth Stack Operations',
    article: `
      <h2>Forth Data Stack Operations</h2>
      <p>Arli maintains an active Forth-like data stack during evaluation. All classic Forth words are available as builtins:</p>
      <ul>
        <li><code>dup</code> (1) — duplicate top item</li>
        <li><code>swap</code> (2) — swap top two items</li>
        <li><code>drop</code> (1) — discard top item</li>
        <li><code>over</code> (2) — duplicate 2nd item over top: <code>(a b) -> (a b a)</code></li>
        <li><code>rot</code> (3) — rotate top 3: <code>(a b c) -> (b c a)</code></li>
        <li><code>nip</code> (2) — drop 2nd item: <code>(a b) -> (b)</code></li>
        <li><code>tuck</code> (2) — copy top under 2nd: <code>(a b) -> (b a b)</code></li>
        <li><code>pick n</code> / <code>roll n</code> — indexed copy and rotation</li>
      </ul>
    `,
    code: `;; Check the 'Data Stack' tab in the output pane below!
dup 42
swap 10 20
rot 1 2 3
`
  },
  {
    id: 'stack-reflection',
    title: '11. Stack Reflection (stack & stack!)',
    article: `
      <h2>Stack Reflection: stack and stack!</h2>
      <p>Arli allows you to capture and replace the evaluator's entire data stack at runtime:</p>
      <ul>
        <li><code>stack</code> (arity 0) — pushes a snapshot of the current stack as a list.</li>
        <li><code>stack!</code> (arity 1) — replaces the evaluator's stack with a given list.</li>
      </ul>
      <p>This enables metaprogramming, stack filters, and custom stack combinators.</p>
    `,
    code: `;; Push numbers to stack
10 20 30

;; Inspect current stack
define s stack
print s

;; Replace entire stack with new items:
stack! [100 200 300]
`
  },
  {
    id: 'exec-stack',
    title: '12. Push-Style Exec Stack',
    article: `
      <h2>Push-Style Exec Stack (Self-Modifying Code)</h2>
      <p>Inspired by the Push programming language, Arli includes an <strong>exec stack</strong> for pending instructions:</p>
      <ul>
        <li><code>exec-push form</code> (1) — push instruction/data onto exec stack</li>
        <li><code>exec-stack</code> (0) — inspect exec stack</li>
        <li><code>exec! list</code> (1) — replace exec stack</li>
        <li><code>(exec)</code> — process exec stack until empty</li>
      </ul>
    `,
    code: `;; Push operations onto exec stack in reverse:
exec-push (quote +)
exec-push 25
exec-push 15

;; Process exec stack:
(exec)
`
  },
  {
    id: 'sequences',
    title: '13. Sequences: map, filter, reduce',
    article: `
      <h2>Sequence Operations</h2>
      <p>Arli provides built-in sequence operations with fixed arity:</p>
      <ul>
        <li><code>map fn list</code> (arity 2)</li>
        <li><code>filter fn list</code> (arity 2)</li>
        <li><code>reduce fn init list</code> (arity 3)</li>
      </ul>
    `,
    code: `define numbers [1 2 3 4 5 6 7 8]

print map (fn (x) * x 10) numbers
print filter (fn (x) > x 4) numbers
print reduce (fn (acc x) + acc x) 0 numbers
`
  },
  {
    id: 'data-structures',
    title: '14. Vectors and Hash Maps',
    article: `
      <h2>Vectors and Hash Maps</h2>
      <p>Arli provides rich literal syntax for collections:</p>
      <ul>
        <li><strong>Vectors</strong>: <code>[1 2 3]</code></li>
        <li><strong>Hash Maps</strong>: <code>{:key1 val1 :key2 val2}</code></li>
      </ul>
      <p>Mutate nested structures with <code>(set! obj key val)</code>.</p>
    `,
    code: `define user {:name "Arli" :version "1.0" :tags ["lisp" "forth"]}

print user

;; Update nested key:
(set! user :version "1.1")
print user
`
  },
  {
    id: 'literals',
    title: '15. Literals, Bases & Strings',
    article: `
      <h2>Keywords, Base Literals & Strings</h2>
      <p>Arli supports diverse literals:</p>
      <ul>
        <li><strong>Keywords</strong>: <code>:admin</code>, <code>:status</code> (self-evaluating)</li>
        <li><strong>Hex, Octal, Binary</strong>: <code>0xFF</code> (255), <code>0o77</code> (63), <code>0b1010</code> (10)</li>
        <li><strong>Triple-Quoted Strings</strong>: <code>"""multi-line string"""</code></li>
      </ul>
    `,
    code: `print 0xFF
print 0b101010
print 0o77

define docstring """
  Arli supports
  multi-line strings
  seamlessly.
"""
print docstring
`
  },
  {
    id: 'result-type',
    title: '16. Result Type (Ok & Err)',
    article: `
      <h2>Result Type for Error Handling</h2>
      <p>Arli includes a lightweight Result type for safe computations without exceptions:</p>
      <ul>
        <li><code>(Ok val)</code> and <code>(Err err)</code></li>
        <li><code>map-ok result fn</code></li>
        <li><code>and-then result fn</code></li>
        <li><code>or-else result fn</code></li>
      </ul>
    `,
    code: `defn safe-div (a b)
    if (= b 0)
        (Err "Division by zero!")
        (Ok / a b)

define r1 safe-div 10 2
define r2 safe-div 10 0

print map-ok r1 (fn (x) * x 100)
print map-ok r2 (fn (x) * x 100)
`
  },
  {
    id: 'pattern-matching',
    title: '17. Pattern Matching (match)',
    article: `
      <h2>Pattern Matching with match</h2>
      <p>The <code>match</code> special form allows structural pattern matching over values, lists, and symbols:</p>
      <pre><code>(match val
    (pattern1 result1)
    (pattern2 result2)
    (_ default-result))</code></pre>
    `,
    code: `defn describe-shape (s)
    (match s
        ([:circle r] * 3.14159 * r r)
        ([:rect w h] * w h)
        (_ 0))

print describe-shape [:circle 10]
print describe-shape [:rect 4 5]
`
  },
  {
    id: 'fexprs',
    title: '18. F-Expressions (First-Class Operatives)',
    article: `
      <h2>F-Expressions: Metaprogramming Without Macros</h2>
      <p>In traditional Lisps, code transformations rely on <strong>macros</strong> which run in a separate compile-time expansion phase. Arli instead adopts <strong>F-expressions</strong> (operatives defined via <code>defn-fexpr</code>).</p>
      <p>Unlike regular functions, F-expressions <strong>do not evaluate their arguments eagerly</strong>. They receive the raw unevaluated AST directly at runtime:</p>
      <ul>
        <li><strong>First-Class Control Flow</strong>: Create custom conditionals, loops, and short-circuit operators.</li>
        <li><strong>No Macro Phasing</strong>: F-expressions are regular first-class values and execute in the standard runtime environment.</li>
        <li><strong>Selective Evaluation</strong>: Use <code>eval expr</code> (arity 1) to explicitly evaluate chosen AST branches in the caller scope.</li>
      </ul>
      <div class="info-box">
        <p><strong>Operatives vs Macros:</strong> Because F-expressions operate directly on raw forms with runtime <code>eval</code>, Arli achieves full metaprogramming power without needing a complex macro expander or code quasiquoting pass.</p>
      </div>
    `,
    code: `;; Custom short-circuiting conditional:
defn-fexpr my-if (cond-expr then-expr else-expr)
    if (eval cond-expr)
        (eval then-expr)
        (eval else-expr)

print my-if true "Evaluated True!" (print "Never executed")

;; Custom short-circuiting OR:
defn-fexpr short-or (a b)
    let ((av (eval a)))
        if av av (eval b)

print short-or true (print "Never runs")
print short-or false "Evaluated second branch"

;; Inspect raw unevaluated AST:
defn-fexpr ast-inspect (expr) expr
print ast-inspect (+ 10 (* 20 30))
`
  },
  {
    id: 'doc-assert',
    title: '19. Documentation & Assertions',
    article: `
      <h2>Documentation & Unit Testing</h2>
      <p>Arli includes built-in facilities for docstrings and assertions:</p>
      <ul>
        <li><code>doc! symbol "description"</code> — attach docstring</li>
        <li><code>doc symbol</code> — retrieve docstring</li>
        <li><code>(assert expr "error message")</code> — assert truthiness</li>
      </ul>
    `,
    code: `defn add (x y) + x y
doc! add "Adds two numbers and returns the sum."

print doc add

;; Assert test:
(assert (= (add 2 3) 5) "Addition test failed")
print "Assertion passed successfully!"
`
  },
  {
    id: 'host-interop',
    title: '20. Host & Browser Interoperability',
    article: `
      <h2>Host & Browser Interoperability</h2>
      <p>In JavaScript / Browser environments, Arli can interact directly with the host runtime:</p>
      <ul>
        <li><code>host "expression"</code> — evaluates host JavaScript in global scope</li>
        <li><code>. obj attr [args...]</code> — attribute lookup and method calls</li>
      </ul>
    `,
    code: `;; Access browser Math APIs:
print host "Math.sqrt(144)"
print host "document.title"

;; Access DOM via dot syntax:
print . document title
`
  }
];

// ---------------------------------------------------------------------------
// Tour Application Engine
// ---------------------------------------------------------------------------

class TourApp {
  constructor() {
    this.currentLessonIndex = 0;
    this.evaluator = new Evaluator(false);

    // DOM Elements
    this.slideArticle = document.getElementById('slide-article');
    this.codeEditor = document.getElementById('code-editor');
    this.lineNumbers = document.getElementById('line-numbers');
    this.consoleOutput = document.getElementById('console-output');
    this.stackItems = document.getElementById('stack-items');
    this.stackCount = document.getElementById('stack-count');
    this.pageIndicator = document.getElementById('page-indicator');
    this.tocList = document.getElementById('toc-list');
    this.tocDrawer = document.getElementById('toc-drawer');
    this.tocBackdrop = document.getElementById('toc-backdrop');

    // Buttons
    this.btnPrev = document.getElementById('btn-prev');
    this.btnNext = document.getElementById('btn-next');
    this.btnSlidePrev = document.getElementById('btn-slide-prev');
    this.btnSlideNext = document.getElementById('btn-slide-next');
    this.btnRun = document.getElementById('btn-run');
    this.btnReset = document.getElementById('btn-reset');
    this.btnClear = document.getElementById('btn-clear');
    this.btnToc = document.getElementById('btn-toc');
    this.btnCloseToc = document.getElementById('btn-close-toc');

    this.init();
  }

  init() {
    this.buildToc();
    this.bindEvents();
    this.loadLesson(this.getSavedLessonIndex());
  }

  getSavedLessonIndex() {
    const saved = localStorage.getItem('arli_tour_lesson');
    const idx = parseInt(saved, 10);
    return isNaN(idx) || idx < 0 || idx >= LESSONS.length ? 0 : idx;
  }

  saveLessonIndex(idx) {
    localStorage.setItem('arli_tour_lesson', String(idx));
  }

  buildToc() {
    this.tocList.innerHTML = '';
    LESSONS.forEach((lesson, index) => {
      const li = document.createElement('li');
      li.className = `toc-item ${index === this.currentLessonIndex ? 'active' : ''}`;
      li.innerHTML = `
        <span class="toc-item-num">${index + 1}</span>
        <span class="toc-item-title">${lesson.title.replace(/^\d+\.\s*/, '')}</span>
      `;
      li.addEventListener('click', () => {
        this.loadLesson(index);
        this.closeToc();
      });
      this.tocList.appendChild(li);
    });
  }

  updateTocActive() {
    const items = this.tocList.querySelectorAll('.toc-item');
    items.forEach((item, index) => {
      if (index === this.currentLessonIndex) {
        item.classList.add('active');
      } else {
        item.classList.remove('active');
      }
    });
  }

  bindEvents() {
    // Navigation
    this.btnPrev.addEventListener('click', () => this.prevLesson());
    this.btnNext.addEventListener('click', () => this.nextLesson());
    this.btnSlidePrev.addEventListener('click', () => this.prevLesson());
    this.btnSlideNext.addEventListener('click', () => this.nextLesson());

    // TOC
    this.btnToc.addEventListener('click', () => this.openToc());
    this.btnCloseToc.addEventListener('click', () => this.closeToc());
    this.tocBackdrop.addEventListener('click', () => this.closeToc());

    // Execution
    this.btnRun.addEventListener('click', () => this.runCode());
    this.btnReset.addEventListener('click', () => this.resetCode());
    this.btnClear.addEventListener('click', () => this.clearOutput());

    // Output tabs
    document.querySelectorAll('.output-tabs .tab').forEach(tab => {
      tab.addEventListener('click', (e) => {
        document.querySelectorAll('.output-tabs .tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.output-view').forEach(v => v.classList.remove('active'));
        e.target.classList.add('active');
        const viewId = `output-view-${e.target.dataset.tab}`;
        const view = document.getElementById(viewId);
        if (view) view.classList.add('active');
      });
    });

    // Editor line numbers & tab key handling
    this.codeEditor.addEventListener('input', () => this.updateLineNumbers());
    this.codeEditor.addEventListener('scroll', () => {
      this.lineNumbers.scrollTop = this.codeEditor.scrollTop;
    });

    this.codeEditor.addEventListener('keydown', (e) => {
      // Ctrl+Enter or Cmd+Enter to Run
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        this.runCode();
        return;
      }
      // Tab key indent
      if (e.key === 'Tab') {
        e.preventDefault();
        const start = this.codeEditor.selectionStart;
        const end = this.codeEditor.selectionEnd;
        this.codeEditor.value = this.codeEditor.value.substring(0, start) + '    ' + this.codeEditor.value.substring(end);
        this.codeEditor.selectionStart = this.codeEditor.selectionEnd = start + 4;
        this.updateLineNumbers();
      }
    });

    // Global keyboard navigation
    window.addEventListener('keydown', (e) => {
      if (document.activeElement === this.codeEditor) return;
      if (e.key === 'ArrowLeft') {
        this.prevLesson();
      } else if (e.key === 'ArrowRight') {
        this.nextLesson();
      }
    });
  }

  openToc() {
    this.tocDrawer.classList.add('open');
    this.tocBackdrop.classList.add('active');
  }

  closeToc() {
    this.tocDrawer.classList.remove('open');
    this.tocBackdrop.classList.remove('active');
  }

  updateLineNumbers() {
    const lines = this.codeEditor.value.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lines; i++) {
      numbers += i + '\n';
    }
    this.lineNumbers.textContent = numbers.trim();
  }

  loadLesson(index) {
    if (index < 0 || index >= LESSONS.length) return;
    this.currentLessonIndex = index;
    this.saveLessonIndex(index);

    const lesson = LESSONS[index];
    this.slideArticle.innerHTML = lesson.article;
    this.codeEditor.value = lesson.code.trim();
    this.updateLineNumbers();

    // Nav controls
    this.pageIndicator.textContent = `${index + 1} / ${LESSONS.length}`;
    this.btnPrev.disabled = index === 0;
    this.btnSlidePrev.disabled = index === 0;
    this.btnNext.disabled = index === LESSONS.length - 1;
    this.btnSlideNext.disabled = index === LESSONS.length - 1;

    this.updateTocActive();
    this.clearOutput();
    this.slideArticle.scrollTop = 0;
  }

  prevLesson() {
    if (this.currentLessonIndex > 0) {
      this.loadLesson(this.currentLessonIndex - 1);
    }
  }

  nextLesson() {
    if (this.currentLessonIndex < LESSONS.length - 1) {
      this.loadLesson(this.currentLessonIndex + 1);
    }
  }

  resetCode() {
    const lesson = LESSONS[this.currentLessonIndex];
    this.codeEditor.value = lesson.code.trim();
    this.updateLineNumbers();
    this.clearOutput();
  }

  clearOutput() {
    this.consoleOutput.textContent = 'Click "Run" or press Ctrl+Enter to execute.';
    this.consoleOutput.classList.remove('error');
    this.evaluator = new Evaluator(false);
    this.renderStack([]);
  }

  runCode() {
    const source = this.codeEditor.value;
    const outputs = [];

    // Custom logger for print statements
    const origLog = console.log;
    console.log = (...args) => {
      outputs.push(args.map(a => typeof a === 'object' ? arliRepr(a) : String(a)).join(' '));
    };

    try {
      this.evaluator = new Evaluator(false);
      const result = this.evaluator.exec(source);

      // Restore console.log
      console.log = origLog;

      if (result !== undefined && result !== null) {
        outputs.push(`=> ${arliRepr(result)}`);
      }

      this.consoleOutput.textContent = outputs.length > 0 ? outputs.join('\n') : '(executed with no output)';
      this.consoleOutput.classList.remove('error');
      this.renderStack(this.evaluator.stack);
    } catch (err) {
      console.log = origLog;
      this.consoleOutput.textContent = `Error: ${err.message}`;
      this.consoleOutput.classList.add('error');
    }
  }

  renderStack(stack) {
    this.stackCount.textContent = String(stack.length);
    if (!stack || stack.length === 0) {
      this.stackItems.innerHTML = '<span class="empty-stack-msg">Stack is empty.</span>';
      return;
    }

    this.stackItems.innerHTML = '';
    stack.forEach((item, idx) => {
      const badge = document.createElement('div');
      badge.className = 'stack-item-badge';
      badge.innerHTML = `<span class="idx">[${idx}]</span>${escapeHtml(arliRepr(item))}`;
      this.stackItems.appendChild(badge);
    });
  }
}

function escapeHtml(str) {
  return str.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
}

// Start Tour Application when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  new TourApp();
});
