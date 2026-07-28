#!/usr/bin/env python3
"""Common test runner for arli — runs .arli test files and verifies outputs.

Usage:
    python tests/run_tests.py              # test Python backend
    python tests/run_tests.py --go PATH    # test Go backend via binary
    python tests/run_tests.py --list       # list available tests
"""

import sys
import os
import subprocess
import re
import glob

# Add src to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'src'))

TEST_DIR = os.path.join(os.path.dirname(__file__), 'common')
GO_BINARY = os.path.join(os.path.dirname(__file__), '..', 'go', 'arli', 'arli.exe')


# ---------------------------------------------------------------------------
# Test discovery
# ---------------------------------------------------------------------------

def discover_tests():
    """Return list of (test_name, filepath) tuples sorted by filename."""
    files = sorted(glob.glob(os.path.join(TEST_DIR, '*.arli')))
    tests = []
    for f in files:
        name = os.path.splitext(os.path.basename(f))[0]
        tests.append((name, f))
    return tests


def parse_expected(filepath):
    """Extract expected output lines from ;; expect: comments."""
    expected = []
    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            m = re.match(r'^\s*;;\s*expect:\s*(.*)', line)
            if m:
                expected.append(m.group(1).strip())
    return expected


# ---------------------------------------------------------------------------
# Python backend runner
# ---------------------------------------------------------------------------

def run_python(filepath):
    """Run a .arli file with the Python backend, return output lines."""
    from arli.eval import Evaluator
    from arli.types import arli_repr, nil

    ev = Evaluator()
    source = open(filepath, 'r', encoding='utf-8').read()

    # Capture output
    from io import StringIO
    import contextlib

    output = []
    tokens = None
    from arli.tokenize import tokenize
    from arli.parse import TokenStream

    tokens = tokenize(source)
    stream = TokenStream(tokens)
    while not stream.is_eof:
        expr = ev.parser._parse_expr(stream, allow_arity=True)
        if expr is not None:
            result = ev.eval(expr)
            if result is not None:
                output.append(arli_repr(result))
    return output


# ---------------------------------------------------------------------------
# Go backend runner
# ---------------------------------------------------------------------------

def run_go(filepath, go_binary=GO_BINARY):
    """Run a .arli file with the Go backend, return output lines."""
    if not os.path.exists(go_binary):
        return None  # Go binary not available

    result = subprocess.run(
        [go_binary, filepath],
        capture_output=True,
        text=True,
        timeout=30
    )
    if result.returncode != 0:
        return [f"<ERROR: {result.stderr.strip()}>"]

    output = [line.rstrip('\r') for line in result.stdout.split('\n') if line]
    return output


# ---------------------------------------------------------------------------
# Test runner
# ---------------------------------------------------------------------------

def run_test(name, filepath, use_go=False, go_binary=GO_BINARY):
    """Run a single test and return (passed, expected, actual, errors)."""
    expected = parse_expected(filepath)
    if not expected:
        return True, [], [], []  # No expectations to check

    if use_go:
        actual = run_go(filepath, go_binary)
        if actual is None:
            return False, expected, ["<Go binary not found>"], ["Go binary missing"]
    else:
        actual = run_python(filepath)

    errors = []
    passed = True

    # Compare line by line
    min_len = min(len(expected), len(actual))
    for i in range(min_len):
        if expected[i] != actual[i]:
            errors.append(f"  Line {i+1}: expected {expected[i]!r}, got {actual[i]!r}")
            passed = False

    # Extra lines in actual
    if len(actual) > len(expected):
        for i in range(len(expected), len(actual)):
            errors.append(f"  Extra output line {i+1}: {actual[i]!r}")
            passed = False

    # Missing lines in actual
    if len(actual) < len(expected):
        for i in range(len(actual), len(expected)):
            errors.append(f"  Missing line {i+1}: expected {expected[i]!r}")
            passed = False

    return passed, expected, actual, errors


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    import argparse

    parser = argparse.ArgumentParser(description='Run arli common tests')
    parser.add_argument('--go', nargs='?', const=GO_BINARY, default=None,
                        help='Test Go backend (optional path to binary)')
    parser.add_argument('--python', action='store_true',
                        help='Test Python backend (default)')
    parser.add_argument('--list', action='store_true',
                        help='List available tests')
    parser.add_argument('--filter', type=str, default='',
                        help='Run only tests matching this substring')
    args = parser.parse_args()

    use_go = args.go is not None
    go_binary = args.go if args.go else GO_BINARY

    if args.list:
        print("Available tests:")
        for name, path in discover_tests():
            expected = parse_expected(path)
            print(f"  {name}: {len(expected)} expectations")
        return

    if not args.python and not use_go:
        # Default: test Python
        args.python = True

    tests = discover_tests()
    passed_count = 0
    failed_count = 0

    for name, filepath in tests:
        if args.filter and args.filter not in name:
            continue

        passed, expected, actual, errors = run_test(
            name, filepath, use_go=use_go, go_binary=go_binary)

        result = "PASS" if passed else "FAIL"
        if passed:
            passed_count += 1
        else:
            failed_count += 1

        print(f"  [{result}] {name}")

        if not passed:
            for err in errors[:10]:  # Show first 10 errors
                print(f"         {err}")
            if len(errors) > 10:
                print(f"         ... and {len(errors) - 10} more errors")

    print(f"\n{'='*50}")
    total = passed_count + failed_count
    print(f"  {passed_count}/{total} passed, {failed_count} failed")
    print(f"  Backend: {'Go' if use_go else 'Python'}")

    return 0 if failed_count == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
