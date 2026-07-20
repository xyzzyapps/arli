"""Stack-based evaluator for arli.

Evaluation model (Forth-like + Lisp):
1. Expressions are evaluated left-to-right, top-down
2. The evaluator maintains a data stack (list)
3. Literals push themselves onto the stack
4. Builtins pop arguments (according to their arity), compute, push results
5. User-defined functions (Function) create a new lexical scope, bind args,
   then evaluate their body
6. Special forms (if, define, defn, fn, quote, do, let, set!) have custom
   evaluation rules
"""

from __future__ import annotations
from typing import Any, Optional

from .types import Symbol, nil, Builtin, Function, is_truthy, hya_repr
from .env import Environment
from .parse import ArityTable, Parser, parse_source
from .builtins import get_builtins


# ---------------------------------------------------------------------------
# Evaluator
# ---------------------------------------------------------------------------

class Evaluator:
    """Stack-based arli evaluator.

    The evaluator walks an AST (nested Python lists produced by the parser)
    and evaluates it left-to-right.

    Attributes:
        stack: The data stack (Forth-like).
        env: The current environment.
        global_env: The root environment.
        arity_table: Table of known arities (for the parser).
        debug: If True, print trace info.
    """

    def __init__(self, debug: bool = False) -> None:
        self.stack: list[Any] = []
        self.global_env = Environment(name="global")
        self.env = self.global_env
        self.arity_table = ArityTable()
        self.debug = debug
        self._parser: Optional[Parser] = None

        # Load builtins
        self._load_builtins()

    @property
    def parser(self) -> Parser:
        if self._parser is None:
            self._parser = Parser(self.arity_table)
        return self._parser

    def _load_builtins(self) -> None:
        """Register all builtins in the environment and arity table."""
        for name, builtin in get_builtins().items():
            self.global_env.define(name, builtin)
            if builtin.arity >= 0:
                self.arity_table.register(name, builtin.arity)
            # arity -1 (variadic) means the symbol is registered but with
            # 'variadic' marker — user must use parens
        # Special form arities — ALL fixed, no parens needed
        self.arity_table.register("define", 2)   # define name value
        self.arity_table.register("quote", 1)    # quote expr
        self.arity_table.register("do", -1)      # do -> variadic (use parens)
        self.arity_table.register("set!", 2)     # set! name value
        self.arity_table.register("let", 2)      # let bindings body
        self.arity_table.register("if", 3)       # if cond then else
        self.arity_table.register("while", 2)    # while cond body
        self.arity_table.register("for", 3)      # for var list body
        self.arity_table.register("cond", 1)     # cond clauses-list
        self.arity_table.register("defn", 3)     # defn name (params) body
        self.arity_table.register("fn", 2)       # fn (params) body
        self.arity_table.register("import", 1)   # import module-name
        self.arity_table.register("import!", 2)  # import! module alias
        self.arity_table.register(".", 2)        # . obj attr
        self.arity_table.register("python", 1)   # python "code"
        self.arity_table.register("assert", 2)     # assert expr message
        self.arity_table.register("doc", 2)        # doc symbol "text"
        self.arity_table.register("match", -1)     # match expr clause...
        self.arity_table.register("import-module", 1)  # import-module "path"

    def eval(self, expr: Any) -> Any:
        """Evaluate a single expression and return the result.

        The result is also pushed onto the data stack.
        """
        result = self._eval_expr(expr)
        # Push result onto the data stack
        if result is not None:
            self.stack.append(result)
        return result

    def _eval_expr(self, expr: Any) -> Any:
        """Internal recursive evaluation of a single expression."""
        if self.debug:
            print(f"  EVAL: {hya_repr(expr)}  stack=[{','.join(hya_repr(e) for e in self.stack[-3:])}]")

        # Literals evaluate to themselves
        if isinstance(expr, (int, float, str)):
            return expr

        if expr is nil:
            return nil

        if expr is True:
            return True

        if expr is False:
            return False

        # Symbol: look up in environment (handle special constants)
        if isinstance(expr, Symbol):
            name = expr.name
            # Keywords self-evaluate
            if name.startswith(':'):
                return expr
            # Built-in constants
            if name == "nil":
                return nil
            if name == "true":
                return True
            if name == "false":
                return False
            val = self.env.get(name)
            if val is None:
                raise NameError(f"Undefined symbol: {name}")
            return val
        # List: S-expression application
        if isinstance(expr, list):
            if not expr:
                return nil

            head = expr[0]

            # ---- Special forms ----

            # QUOTE: return the argument unevaluated
            if isinstance(head, Symbol) and head.name == "quote":
                if len(expr) < 2:
                    raise SyntaxError("quote expects 1 argument")
                return expr[1]

            # DEFINE: bind a name to a value
            if isinstance(head, Symbol) and head.name == "define":
                if len(expr) < 3:
                    raise SyntaxError("define expects (define name value)")
                name_expr = expr[1]
                value_expr = expr[2]
                if isinstance(name_expr, Symbol):
                    name = name_expr.name
                    value = self._eval_expr(value_expr)
                    self.env.define(name, value)
                    # Register arity for function values
                    if isinstance(value, Function):
                        self.arity_table.register(name, value.arity)
                    elif callable(value):
                        # Try to detect arity of Python callables
                        try:
                            import inspect
                            sig = inspect.signature(value)
                            required = sum(
                                1 for p in sig.parameters.values()
                                if p.default is inspect.Parameter.empty
                                and p.kind in (
                                    inspect.Parameter.POSITIONAL_ONLY,
                                    inspect.Parameter.POSITIONAL_OR_KEYWORD))
                            if required >= 0:
                                self.arity_table.register(name, required)
                        except (ValueError, TypeError):
                            pass  # Can't determine arity, skip
                    return value
                raise SyntaxError(
                    f"define expects a symbol name, got {name_expr}")

            # DEFN: define a function with known arity
            if isinstance(head, Symbol) and head.name == "defn":
                if len(expr) < 4:
                    raise SyntaxError(
                        "defn expects (defn name (params) body...)")
                name_sym = expr[1]
                params = expr[2]
                body = expr[3:]
                if not isinstance(name_sym, Symbol):
                    raise SyntaxError(
                        f"defn expects a symbol name, got {name_sym}")
                if not isinstance(params, list):
                    raise SyntaxError(
                        "defn expects a parameter list")
                # Create function
                fn = Function(params, body, self.env, name_sym.name)
                self.env.define(name_sym.name, fn)
                # Arity was registered by parser; ensure it matches
                self.arity_table.register(name_sym.name, len(params))
                return fn

            # IF: conditional with arity 3
            if isinstance(head, Symbol) and head.name == "if":
                if len(expr) < 4:
                    raise SyntaxError("if expects (if cond then else)")
                cond = self._eval_expr(expr[1])
                if is_truthy(cond):
                    return self._eval_expr(expr[2])
                else:
                    return self._eval_expr(expr[3])

            # DO: evaluate multiple exprs, return last
            if isinstance(head, Symbol) and head.name == "do":
                result = nil
                for subexpr in expr[1:]:
                    result = self._eval_expr(subexpr)
                return result

            # FN: create anonymous function
            if isinstance(head, Symbol) and head.name == "fn":
                if len(expr) < 3:
                    raise SyntaxError("fn expects (fn (params) body...)")
                params = expr[1]
                body = expr[2:]
                if not isinstance(params, list):
                    raise SyntaxError(
                        "fn expects a parameter list")
                return Function(params, body, self.env)

            # WHILE: loop while condition is truthy (arity 2)
            if isinstance(head, Symbol) and head.name == "while":
                if len(expr) < 3:
                    raise SyntaxError(
                        "while expects (while cond body)")
                cond_expr = expr[1]
                body_exprs = expr[2:]
                result = nil
                while is_truthy(self._eval_expr(cond_expr)):
                    for subexpr in body_exprs:
                        result = self._eval_expr(subexpr)
                return result

            # FOR: iterate over a list (arity 3: var list body)
            if isinstance(head, Symbol) and head.name == "for":
                if len(expr) < 4:
                    raise SyntaxError(
                        "for expects (for var list body)")
                var_expr = expr[1]
                list_expr = self._eval_expr(expr[2])
                body_expr = expr[3:]
                if not isinstance(var_expr, Symbol):
                    raise SyntaxError(
                        f"for expects a symbol as variable, got {var_expr}")
                if not isinstance(list_expr, list):
                    raise TypeError(
                        f"for expects a list, got {type(list_expr)}")
                result = nil
                for item in list_expr:
                    self.env.define(var_expr.name, item)
                    for subexpr in body_expr:
                        result = self._eval_expr(subexpr)
                return result

            # COND: multi-branch conditional (arity 1: clause-list)
            # Clauses are flat pairs: (test1 result1 test2 result2 ...)
            if isinstance(head, Symbol) and head.name == "cond":
                if len(expr) < 2:
                    raise SyntaxError(
                        "cond expects (cond clause...)")
                clauses = expr[1]
                if isinstance(clauses, list):
                    i = 0
                    while i < len(clauses) - 1:
                        test = self._eval_expr(clauses[i])
                        result = clauses[i + 1]
                        if is_truthy(test):
                            return self._eval_expr(result)
                        i += 2
                    # Trailing single clause (else-like)
                    if i < len(clauses):
                        if is_truthy(self._eval_expr(clauses[i])):
                            return self._eval_expr(clauses[i])
                return nil

            # SET!: mutate a binding or nested structure
            if isinstance(head, Symbol) and head.name == "set!":
                if len(expr) < 3:
                    raise SyntaxError("set! expects (set! target value)")
                name_expr = expr[1]
                value = self._eval_expr(expr[-1])  # last arg is value
                if isinstance(name_expr, Symbol) and len(expr) == 3:
                    # Simple variable set!
                    self.env.set(name_expr.name, value)
                    return value
                # Nested structure set!: (set! obj key val) or (set! obj :key val)
                if len(expr) >= 4:
                    obj = self._eval_expr(expr[1])
                    key = expr[2]
                    if len(expr) > 3:
                        # Multi-arg: (set! map :key val)
                        if isinstance(obj, dict):
                            if isinstance(key, Symbol) and key.name.startswith(':'):
                                obj[key.name] = value
                            elif isinstance(key, str):
                                obj[key] = value
                            else:
                                obj[str(key)] = value
                            return value
                        if isinstance(obj, list) and isinstance(key, (int, float)):
                            idx = int(key)
                            if 0 <= idx < len(obj):
                                obj[idx] = value
                                return value
                raise SyntaxError(
                    f"set! cannot set on target: {hya_repr(name_expr)}")
            # LET: local bindings
            if isinstance(head, Symbol) and head.name == "let":
                if len(expr) < 3:
                    raise SyntaxError(
                        "let expects (let ((name val)...) body...)")
                bindings = expr[1]
                body = expr[2:]
                # Create a new scope
                let_env = self.env.extend("let")
                old_env = self.env
                self.env = let_env
                try:
                    if isinstance(bindings, list):
                        for binding in bindings:
                            if (isinstance(binding, list)
                                    and len(binding) >= 2):
                                bname = binding[0]
                                bval = self._eval_expr(binding[1])
                                if isinstance(bname, Symbol):
                                    self.env.define(bname.name, bval)
                    result = nil
                    for subexpr in body:
                        result = self._eval_expr(subexpr)
                    return result
                finally:
                    self.env = old_env

            # ---- Python Interop ----

            # IMPORT: import a Python module (arity 1: import os)
            if isinstance(head, Symbol) and head.name in ("import", "import!"):
                if len(expr) < 2:
                    raise SyntaxError("import expects (import module-name)")
                module_name = expr[1]
                if isinstance(module_name, Symbol):
                    import importlib
                    try:
                        mod = importlib.import_module(module_name.name)
                        name = module_name.name
                        # Use import! to bind under a different name
                        if head.name == "import!" and len(expr) >= 3:
                            if isinstance(expr[2], Symbol):
                                name = expr[2].name
                        self.env.define(name, mod)
                        return mod
                    except ImportError as e:
                        raise ImportError(
                            f"Cannot import Python module '{module_name.name}': {e}")
                raise TypeError(
                    f"import expects a symbol, got {module_name}")

            # DOT: chained attribute access
            # (. os path join "a" "b") = os.path.join("a", "b")
            # Top-level: . os path (arity 2) = getattr(os, 'path')
            if isinstance(head, Symbol) and head.name == ".":
                if len(expr) < 3:
                    raise SyntaxError(
                        ". expects (. obj attr [attr...] [args...])")
                obj = self._eval_expr(expr[1])
                # Walk through attr chain, then collect call args
                i = 2
                while i < len(expr):
                    item = expr[i]
                    if isinstance(item, Symbol):
                        # Attribute access
                        attr_name = item.name
                        if hasattr(obj, attr_name):
                            obj = getattr(obj, attr_name)
                            i += 1
                        else:
                            raise AttributeError(
                                f"'{type(obj).__name__}' "
                                f"has no attribute '{attr_name}'")
                    else:
                        # Not a symbol — start of call args
                        break
                # If we have remaining items, call the result
                if i < len(expr):
                    call_args = [self._eval_expr(a)
                                 for a in expr[i:]]
                    if callable(obj):
                        return obj(*call_args)
                    raise TypeError(
                        f"Cannot call non-callable: {obj}")
                return obj

            # PYTHON: evaluate arbitrary Python expression
            if isinstance(head, Symbol) and head.name == "python":
                if len(expr) < 2:
                    raise SyntaxError(
                        "python expects (python \"code\")")
                code = expr[1]
                if isinstance(code, str):
                    import builtins as py_builtins
                    # Evaluate in context of the global environment
                    local_vars = {}
                    for k, v in self.global_env._bindings.items():
                        local_vars[k] = v
                    try:
                        result = eval(code, py_builtins.__dict__, local_vars)
                        return result
                    except Exception as e:
                        raise RuntimeError(
                            f"Python eval error: {e}")
                raise TypeError(
                    f"python expects a string, got {type(code)}")

            # ASSERT: (assert expr message)
            if isinstance(head, Symbol) and head.name == "assert":
                if len(expr) < 2:
                    raise SyntaxError("assert expects (assert expr message)")
                val = self._eval_expr(expr[1])
                if not is_truthy(val):
                    msg = ""
                    if len(expr) >= 3:
                        msg = self._eval_expr(expr[2])
                    raise AssertionError(f"Assertion failed: {hya_repr(msg)}")
                return val

            # DOC: (doc symbol) or (doc symbol "text")
            if isinstance(head, Symbol) and head.name == "doc":
                if len(expr) >= 3:
                    # Storing doc
                    sym = expr[1]
                    doc_text = self._eval_expr(expr[2])
                    if isinstance(sym, Symbol):
                        self.env.define(f"__doc_{sym.name}", doc_text)
                        return doc_text
                elif len(expr) >= 2:
                    # Retrieving doc
                    sym = expr[1]
                    if isinstance(sym, Symbol):
                        doc_val = self.env.lookup(f"__doc_{sym.name}")
                        if doc_val is not None:
                            return doc_val
                    return nil
                return nil

            # MATCH: (match expr clause...)
            if isinstance(head, Symbol) and head.name == "match":
                if len(expr) < 2:
                    raise SyntaxError("match expects (match expr clause...)")
                match_val = self._eval_expr(expr[1])
                for clause in expr[2:]:
                    if isinstance(clause, list) and len(clause) >= 2:
                        pattern = clause[0]
                        result = clause[1]
                        if self._match_pattern(pattern, match_val):
                            return self._eval_expr(result)
                return nil

            # IMPORT-MODULE: load and execute an .arli file
            if isinstance(head, Symbol) and head.name == "import-module":
                if len(expr) < 2:
                    raise SyntaxError("import-module expects (import-module \"path\")")
                path = self._eval_expr(expr[1])
                if isinstance(path, str):
                    import os
                    full_path = path
                    if not os.path.exists(full_path):
                        full_path = os.path.join(os.path.dirname(os.path.abspath('.')), path)
                    if os.path.exists(full_path):
                        return self.exec_file(full_path)
                    raise FileNotFoundError(f"Cannot find module: {path}")
                raise TypeError(f"import-module expects a string path")

            # ---- Generic function application ----
            if isinstance(head, Symbol) and head.name == "defn-rec":
                # Same as defn but the function body can recurse
                if len(expr) < 4:
                    raise SyntaxError(
                        "defn-rec expects (defn-rec name (params) body...)")
                name_sym = expr[1]
                params = expr[2]
                body = expr[3:]
                fn = Function(params, body, self.env, name_sym.name)
                # Define BEFORE evaluating body (for recursion)
                self.env.define(name_sym.name, fn)
                self.arity_table.register(name_sym.name, len(params))
                return fn

            # Generic function call
            fn_val = self._eval_expr(head)
            args = [self._eval_expr(arg) for arg in expr[1:]]

            if isinstance(fn_val, Builtin):
                return fn_val(*args, evaluator=self)

            if isinstance(fn_val, Function):
                return self._apply_function(fn_val, args)

            if callable(fn_val):
                return fn_val(*args)

            raise TypeError(
                f"Cannot call non-function: {hya_repr(fn_val)}")

        raise TypeError(f"Unknown expression type: {type(expr)}: {expr}")

    def apply(self, fn: Any, args: list) -> Any:
        """Apply a function (builtin, user-defined, or callable) to args."""
        if isinstance(fn, Builtin):
            return fn(*args, evaluator=self)
        if isinstance(fn, Function):
            return self._apply_function(fn, args)
        if callable(fn):
            return fn(*args)
        raise TypeError(f"Cannot call non-function: {hya_repr(fn)}")

    def _apply_function(self, fn: Function, args: list) -> Any:
        """Apply a user-defined function with given arguments.

        Creates a new lexical scope, binds parameters to args,
        evaluates the body, and returns the result.
        """
        if len(args) != len(fn.params):
            raise TypeError(
                f"Function expected {len(fn.params)} args, "
                f"got {len(args)}")

        # Create call environment
        call_env = fn.env.extend(f"call({fn.name or 'anon'})")
        old_env = self.env
        self.env = call_env

        try:
            # Bind parameters
            for param, arg in zip(fn.params, args):
                self.env.define(param.name, arg)

            # Evaluate body
            if isinstance(fn.body, list):
                result = nil
                for subexpr in fn.body:
                    result = self._eval_expr(subexpr)
                return result
            else:
                return self._eval_expr(fn.body)
        finally:
            self.env = old_env

    def _match_pattern(self, pattern: Any, value: Any) -> bool:
        """Simple pattern matching for match form.
        - Symbol _ matches anything
        - A list matches if each element matches
        - Literals match by equality"""
        if isinstance(pattern, Symbol):
            if pattern.name == "_":
                return True
            # Symbols match by name equality
            if isinstance(value, Symbol):
                return pattern.name == value.name
            return False
        if isinstance(pattern, list) and isinstance(value, list):
            if len(pattern) != len(value):
                return False
            for p, v in zip(pattern, value):
                if not self._match_pattern(p, v):
                    return False
            return True
        # Literal matching
        return pattern == value

    def exec(self, source: str) -> Any:
        """Parse and evaluate arli source code.

        Expressions are parsed and evaluated ONE AT A TIME so that
        arity registrations from `defn`/`define` take effect for
        subsequent expressions in the same source text.

        Args:
            source: arli source string.

        Returns:
            Result of the last expression.
        """
        from .tokenize import tokenize
        from .parse import TokenStream
        tokens = tokenize(source)
        stream = TokenStream(tokens)
        result = nil
        while not stream.is_eof:
            expr = self.parser._parse_expr(stream, allow_arity=True)
            if expr is not None:
                result = self.eval(expr)
        return result

    def exec_file(self, path: str) -> Any:
        """Load and execute a .arli file."""
        with open(path, 'r', encoding='utf-8') as f:
            source = f.read()
        return self.exec(source)
