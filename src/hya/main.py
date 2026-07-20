"""CLI entry point for Hya.

Usage:
    python -m hya              # Start REPL
    python -m hya file.hya     # Run a script
    python -m hya -d file.hya  # Run with debug
"""

from __future__ import annotations
import sys
import argparse

from .eval import Evaluator
from .repl import REPL
from .types import hya_repr


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Hya — Arity-driven Forth-like Lisp")
    parser.add_argument("files", nargs="*", help=".hya files to execute")
    parser.add_argument("-d", "--debug", action="store_true",
                        help="Enable debug output")
    parser.add_argument("-e", "--eval", type=str,
                        help="Evaluate an expression")

    args = parser.parse_args()

    if args.eval:
        # Evaluate a single expression
        ev = Evaluator(debug=args.debug)
        result = ev.exec(args.eval)
        print(hya_repr(result))
        return

    if args.files:
        # Run files
        ev = Evaluator(debug=args.debug)
        for filepath in args.files:
            try:
                ev.exec_file(filepath)
            except Exception as e:
                print(f"Error in {filepath}: {e}", file=sys.stderr)
                if args.debug:
                    import traceback
                    traceback.print_exc()
                sys.exit(1)
        return

    # REPL
    repl = REPL(debug=args.debug)
    repl.run()


if __name__ == "__main__":
    main()
