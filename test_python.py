"""Test Python interop for Hya."""
import sys
sys.path.insert(0, 'src')
from hya.eval import Evaluator

ev = Evaluator()

# Test import
ev.exec('import os')
m = ev.env.get('os')
assert m is not None, 'os module not imported'
print('import os: OK')

# Test . (dot) attribute access
r = ev.exec('. os sep')
print('dot access: OK - os.sep =', repr(r))

# Test . with parens for method call
r = ev.exec('(. os path join "a" "b")')
print('method call via parens dot: OK -', r)

# Test python special form
r = ev.exec('python "repr(42)"')
assert r == '42', f'expected 42, got {r}'
print('python eval: OK -', r)

# Test chaining: define p = . os path, then call p.join directly
ev.exec('import os')
ev.exec('define p . os path')
print('os.path:', ev.env.get('p'))

# Direct call via parens dot expression
r = ev.exec('(. os path join "x" "y")')
print('direct (. os path join):', r)

# Chained via p
r = ev.exec('(. p join "x" "y")')
print('via p (. p join):', r)

print()
print('All Python interop tests passed!')
