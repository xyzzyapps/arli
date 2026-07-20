"""Interactive REPL for Hya.

Provides a Read-Eval-Print Loop with:
- Full arity-driven parsing
- Stack inspection
- Multi-line input (blank line to evaluate)
- History and editing via readline when available
"""

from __future__ import annotations
from typing import Optional

from .eval import Evaluator
from .types import hya_repr


class REPL:
    """Interactive Hya REPL."""

    def __init__(self, evaluator: Optional[Evaluator] = None,
                 debug: bool = False) -> None:
        self.evaluator = evaluator or Evaluator(debug=debug)
        self.history: list[str] = []

    def run(self) -> None:
        """Start the REPL loop."""
        import sys

        # Try to enable readline
        try:
            import readline  # noqa: F401
        except ImportError:
            pass

        print(f"Hya v{__import__('hya').__version__}")
        print("Arity-driven Lisp with Forth-like stack operations")
        print("Type 'help' for commands, 'exit' or Ctrl+C to quit")
        print()

        while True:
            try:
                line = self._read_input()
                if line is None:
                    break
                line = line.strip()
                if not line:
                    continue

                # Handle meta-commands
                if line.startswith('/'):
                    self._handle_command(line[1:])
                    continue
                if line == "exit" or line == "quit":
                    break
                if line == "help":
                    self._show_help()
                    continue

                # Evaluate
                self._eval_line(line)

            except KeyboardInterrupt:
                print("\n^C")
                continue
            except EOFError:
                print()
                break
            except Exception as e:
                print(f"Error: {e}")
                if self.evaluator.debug:
                    import traceback
                    traceback.print_exc()

    def _read_input(self) -> Optional[str]:
        """Read input, supporting multi-line expressions.

        Returns None on EOF, otherwise the complete input string.
        """
        import sys
        lines: list[str] = []

        # Read first line
        try:
            line = input("hya> ")
        except EOFError:
            return None

        lines.append(line)

        # Check if we need more input (unclosed parens or incomplete)
        while self._needs_more_input('\n'.join(lines)):
            try:
                line = input("   .. ")
                lines.append(line)
            except EOFError:
                break
            except KeyboardInterrupt:
                return '\n'.join(lines)

        return '\n'.join(lines)

    def _needs_more_input(self, text: str) -> bool:
        """Check if the input is incomplete."""
        stripped = text.strip()
        if not stripped:
            return False

        # Count open vs close parens
        opens = stripped.count('(')
        closes = stripped.count(')')
        if opens > closes:
            return True

        # Check for hanging quote
        in_string = False
        i = 0
        while i < len(stripped):
            if stripped[i] == '"':
                in_string = not in_string
            elif stripped[i] == '\\' and i + 1 < len(stripped):
                i += 1
            i += 1
        if in_string:
            return True

        return False

    def _eval_line(self, line: str) -> None:
        """Parse and evaluate a single line of Hya code."""
        self.history.append(line)

        # Try to parse and evaluate
        try:
            exprs = self.evaluator.parser.parse(line)
            for expr in exprs:
                result = self.evaluator._eval_expr(expr)
                if result is not None:
                    self.evaluator.stack.append(result)
                    print(hya_repr(result))
        except SyntaxError as e:
            # Maybe it's a partial expression — try wrapping in parens
            raise e

    def _handle_command(self, cmd: str) -> None:
        """Handle meta-commands."""
        parts = cmd.strip().split()
        if not parts:
            return
        command = parts[0]

        if command == "stack" or command == "s":
            self._show_stack()
        elif command == "env" or command == "e":
            self._show_env()
        elif command == "arity" or command == "a":
            self._show_arity()
        elif command == "clear" or command == "c":
            self.evaluator.stack.clear()
            print("Stack cleared.")
        elif command == "debug" or command == "d":
            self.evaluator.debug = not self.evaluator.debug
            print(f"Debug mode: {self.evaluator.debug}")
        elif command == "reset":
            self.evaluator = Evaluator(debug=self.evaluator.debug)
            print("Evaluator reset.")
        else:
            print(f"Unknown command: {command}")
            print("Commands: /stack, /env, /arity, /clear, /debug, /reset")

    def _show_stack(self) -> None:
        """Display the current data stack."""
        stack = self.evaluator.stack
        if not stack:
            print("Stack is empty.")
            return
        print(f"Stack ({len(stack)} items):")
        for i, val in enumerate(stack):
            print(f"  {i}: {hya_repr(val)}")

    def _show_env(self) -> None:
        """Display the current environment bindings."""
        env = self.evaluator.env
        bindings = {}
        e = env
        while e:
            for k, v in e._bindings.items():
                if k not in bindings:
                    bindings[k] = v
            e = e.parent

        if not bindings:
            print("Environment is empty.")
            return
        print(f"Environment ({len(bindings)} bindings):")
        for name, val in sorted(bindings.items()):
            print(f"  {name}: {hya_repr(val)}")

    def _show_arity(self) -> None:
        """Display all registered arities."""
        table = self.evaluator.arity_table._table
        if not table:
            print("No arity registrations.")
            return
        print("Registered arities:")
        for name, arity in sorted(table.items()):
            label = str(arity) if arity >= 0 else "variadic"
            print(f"  {name}: {label}")

    def _show_help(self) -> None:
        """Display help."""
        print("Hya — Arity-driven Forth-like Lisp")
        print()
        print("Basic syntax:")
        print("  + 1 2          ; arity-driven, no parens needed")
        print("  * + 1 2 3      ; (= (* (+ 1 2) 3))")
        print("  define x + 1 2 ; define with arity 2")
        print("  (if cond a b)  ; if uses parens since it's a special form")
        print("  (defn add (x y) (+ x y))")
        print()
        print("Stack operations (known arity, no parens):")
        print("  dup, swap, drop, over, rot, nip, tuck")
        print()
        print("Commands:")
        print("  /stack  - show data stack")
        print("  /env    - show environment")
        print("  /arity  - show arity table")
        print("  /clear  - clear stack")
        print("  /debug  - toggle debug mode")
        print("  /reset  - reset evaluator")
        print("  help    - show this help")
        print("  exit    - exit REPL")
