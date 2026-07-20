#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { Evaluator } from '../src/eval.js';
import { arliRepr } from '../src/types.js';
import { tokenize, TokenStream } from '../src/tokenize.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const TEST_DIRS = [
  path.join(__dirname, '..', '..', '..', 'tests', 'common'),
  path.join(__dirname, '..', 'test_common'),
];

function findTestDir() {
  for (const d of TEST_DIRS) { if (fs.existsSync(d)) return d; }
  const projectTests = path.join(__dirname, '..', '..', '..', 'tests', 'common');
  if (fs.existsSync(projectTests)) {
    const localCopy = path.join(__dirname, '..', 'test_common');
    if (!fs.existsSync(localCopy)) {
      fs.mkdirSync(localCopy, { recursive: true });
      for (const f of fs.readdirSync(projectTests).filter(f => f.endsWith('.arli'))) fs.copyFileSync(path.join(projectTests, f), path.join(localCopy, f));
    }
    return localCopy;
  }
  return null;
}

function discoverTests(testDir) {
  return fs.readdirSync(testDir).filter(f => f.endsWith('.arli')).sort().map(f => ({ name: path.basename(f, '.arli'), filepath: path.join(testDir, f) }));
}

function parseExpected(filepath) {
  const expected = [];
  for (const line of fs.readFileSync(filepath, 'utf-8').split('\n')) {
    const m = line.match(/^\s*;;\s*expect:\s*(.*)/);
    if (m) expected.push(m[1].trim());
  }
  return expected;
}

function runJS(filepath) {
  const ev = new Evaluator();
  const output = [];
  const stream = new TokenStream(tokenize(fs.readFileSync(filepath, 'utf-8')));
  while (!stream.isEOF) {
    const expr = ev.parser._parseExpr(stream, true);
    if (expr !== null && expr !== undefined) {
      const result = ev.eval(expr);
      if (result !== null && result !== undefined) output.push(arliRepr(result));
    }
  }
  return output;
}

function runTest(name, filepath) {
  const expected = parseExpected(filepath);
  if (expected.length === 0) return { passed: true, expected: [], actual: [], errors: [] };
  const actual = runJS(filepath);
  const errors = [];
  let passed = true;
  const minLen = Math.min(expected.length, actual.length);
  for (let i = 0; i < minLen; i++) {
    if (expected[i] !== actual[i]) { errors.push(`  Line ${i + 1}: expected ${JSON.stringify(expected[i])}, got ${JSON.stringify(actual[i])}`); passed = false; }
  }
  for (let i = minLen; i < actual.length; i++) { errors.push(`  Extra output line ${i + 1}: ${JSON.stringify(actual[i])}`); passed = false; }
  for (let i = minLen; i < expected.length; i++) { errors.push(`  Missing line ${i + 1}: expected ${JSON.stringify(expected[i])}`); passed = false; }
  return { passed, expected, actual, errors };
}

function main() {
  const args = process.argv.slice(2);
  const showList = args.includes('--list');
  const filter = args.includes('--filter') ? args[args.indexOf('--filter') + 1] : '';
  const testDir = findTestDir();
  if (!testDir) { console.error('Error: Could not find test directory.'); process.exit(1); }
  const tests = discoverTests(testDir);

  if (showList) {
    console.log('Available tests:');
    for (const { name, filepath } of tests) console.log(`  ${name}: ${parseExpected(filepath).length} expectations`);
    return;
  }

  let passedCount = 0, failedCount = 0;
  for (const { name, filepath } of tests) {
    if (filter && !name.includes(filter)) continue;
    const { passed, errors } = runTest(name, filepath);
    console.log(`  [${passed ? 'PASS' : 'FAIL'}] ${name}`);
    if (passed) passedCount++;
    else { failedCount++; for (const err of errors.slice(0, 10)) console.log(`         ${err}`); if (errors.length > 10) console.log(`         ... and ${errors.length - 10} more errors`); }
  }

  const total = passedCount + failedCount;
  console.log(`\n${'='.repeat(50)}`);
  console.log(`  ${passedCount}/${total} passed, ${failedCount} failed\n  Backend: JavaScript`);
  process.exit(failedCount > 0 ? 1 : 0);
}

main();
