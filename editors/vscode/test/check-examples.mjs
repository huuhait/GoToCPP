// check-examples.mjs tokenizes every .gp file in examples/ with the gocpp
// grammar, the way VS Code does it (the same vscode-textmate and
// vscode-oniguruma versions as VS Code 1.109). It fails when it finds:
//
//   - a token with an invalid.* scope;
//   - a line that ends a top-level declaration (a line of only "}" or ")" at
//     the start, like the last line of a func or of an import group) after
//     which a grammar rule is still open: all the lines after it would be
//     highlighted as if they were still inside that declaration;
//   - a gocpp form that is not highlighted as gocpp: a "#" (outside comments
//     and strings) that is not a macro, a name!( or name![ that is not a must
//     call or must function, a Go keyword or predeclared name right after "."
//     that is still highlighted as Go's, a header or namespace import that is
//     highlighted like a Go import path, a "macro Name(" declaration whose
//     "macro" is not a keyword, or a "lazy" in the parameters of a macro that
//     is not a modifier;
//   - "macro" or "lazy" highlighted as gocpp's keyword or modifier in any
//     other place, where they are ordinary names (D18).
//
// Run it with "npm test" in editors/vscode, or "make vscode-test".

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const tm = require('vscode-textmate');
const oniguruma = require('vscode-oniguruma');

const extensionDir = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const repoRoot = path.resolve(extensionDir, '..', '..');
const examplesDir = path.join(repoRoot, 'examples');

const keywords = 'break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var'.split(' ');
const predeclared = 'any bool byte comparable complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr true false nil iota append cap clear close complex copy delete imag len make max min new panic print println real recover'.split(' ');
// The scopes that the Go grammar gives a keyword or a predeclared name.
const goWordScope = /^(keyword\.|storage\.type\.|constant\.language\.|entity\.name\.function\.support\.builtin\.go$|entity\.name\.type\.(any|comparable)\.go$)/;

async function loadGrammar() {
  const manifest = JSON.parse(fs.readFileSync(path.join(extensionDir, 'package.json'), 'utf8'));
  const { scopeName, path: grammarPath } = manifest.contributes.grammars[0];
  const file = path.join(extensionDir, grammarPath);
  const raw = tm.parseRawGrammar(fs.readFileSync(file, 'utf8'), file);
  const wasm = fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'));
  await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
  const registry = new tm.Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
      createOnigString: (s) => new oniguruma.OnigString(s),
    }),
    loadGrammar: async (name) => (name === scopeName ? raw : null),
  });
  return registry.loadGrammar(scopeName);
}

function gpFiles(dir) {
  const out = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...gpFiles(p));
    else if (e.name.endsWith('.gp')) out.push(p);
  }
  return out.sort();
}

const grammar = await loadGrammar();
const topDepth = grammar.tokenizeLine('', tm.INITIAL).ruleStack.depth;
const errors = [];
const files = gpFiles(examplesDir);
let lineCount = 0;

for (const file of files) {
  const rel = path.relative(repoRoot, file);
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/);
  let stack = tm.INITIAL;
  // The head of a macro declaration, from "macro Name" until its parameter
  // list closes (it can span lines): the depth in its [type parameters] and in
  // its (parameters).
  let head = null;
  lines.forEach((line, i) => {
    const { tokens, ruleStack } = grammar.tokenizeLine(line, stack);
    stack = ruleStack;
    lineCount++;
    const at = (col) => tokens.find((t) => t.startIndex <= col && col < t.endIndex);
    const has = (t, re) => t.scopes.some((s) => re.test(s));
    const inText = (t) => has(t, /^(comment|string)\./);
    const report = (col, msg) => errors.push(`${rel}:${i + 1}:${col + 1}: ${msg}`);

    for (const t of tokens) {
      for (const s of t.scopes) {
        if (s.startsWith('invalid.')) report(t.startIndex, `${s} on ${JSON.stringify(line.slice(t.startIndex, t.endIndex))}`);
      }
      if (t.scopes.includes('entity.name.import.go')) {
        const text = line.slice(t.startIndex, t.endIndex);
        if (/^</.test(text) || /^::/.test(text) || /\.(h|hh|hpp|hxx|h\+\+|inl)$/.test(text)) {
          report(t.startIndex, `the import ${JSON.stringify(text)} is highlighted as a Go import path`);
        }
      }
    }
    if (/^[)}\]]+,?\s*(\/\/.*)?$/.test(line) && !inText(at(0)) && ruleStack.depth !== topDepth) {
      report(0, `a grammar rule is still open after this top-level "${line[0]}" (depth ${ruleStack.depth}, want ${topDepth})`);
    }
    for (let col = line.indexOf('#'); col !== -1; col = line.indexOf('#', col + 1)) {
      const t = at(col);
      if (!inText(t) && !has(t, /^entity\.name\.function\.macro\./)) report(col, 'a "#" that is not highlighted as a macro');
    }
    for (const m of line.matchAll(/\b(\w+)!(?=[(\[])/g)) {
      const bang = m.index + m[1].length;
      const t = at(bang);
      if (inText(t)) continue;
      const must = t.scopes.includes('keyword.operator.must.gocpp');
      if (keywords.includes(m[1])) {
        if (must) report(bang, `"${m[1]}!" is Go's ${m[1]} followed by "not", not a must call`);
      } else if (!must || !has(at(m.index), /^entity\.name\.function\./)) {
        report(m.index, `"${m[1]}!" is not highlighted as a must call or must function`);
      }
    }
    for (const m of line.matchAll(/(?<!\.)\.(\w+)\b/g)) {
      if (!keywords.includes(m[1]) && !predeclared.includes(m[1])) continue;
      const t = at(m.index + 1);
      if (!inText(t) && has(t, goWordScope)) report(m.index + 1, `".${m[1]}" is a name after "." but is highlighted as Go's ${m[1]}`);
    }

    // macro and lazy (D18): keywords only in a macro declaration.
    const decl = /^macro\s+(\w+)\s*(?=[(\[])/d.exec(line);
    for (const t of tokens) {
      if (t.scopes.includes('keyword.macro.gocpp') && !(decl && t.startIndex === 0)) report(t.startIndex, '"macro" is highlighted as a keyword outside a macro declaration');
    }
    if (decl) {
      if (!at(0).scopes.includes('keyword.macro.gocpp')) report(0, '"macro" of a macro declaration is not highlighted as a keyword');
      const nameCol = decl.indices[1][0];
      if (!at(nameCol).scopes.includes('entity.name.function.macro.gocpp')) report(nameCol, `the macro name "${decl[1]}" is not highlighted as a macro`);
      head = { brackets: 0, parens: 0 };
    }
    const level = new Array(line.length).fill(0); // 1: directly in the parameters of a macro
    for (let c = decl ? decl[0].length : 0; head && c < line.length; c++) {
      const ch = line[c];
      if ('[]()'.includes(ch) && !inText(at(c))) {
        if (head.parens === 0 && (ch === '[' || ch === ']')) head.brackets += ch === '[' ? 1 : -1;
        else if (head.brackets === 0 && ch === '(') head.parens++;
        else if (head.brackets === 0 && ch === ')' && --head.parens === 0) head = null;
      }
      if (head) level[c] = head.parens;
    }
    for (const m of line.matchAll(/\blazy\b/g)) {
      const t = at(m.index);
      if (inText(t)) continue;
      const modifier = t.scopes.includes('storage.modifier.lazy.gocpp');
      if (level[m.index] === 1 && /\w\s+$/.test(line.slice(0, m.index))) {
        if (!modifier) report(m.index, '"lazy" in the parameters of a macro is not highlighted as a modifier');
      } else if (level[m.index] === 0 && modifier) {
        report(m.index, '"lazy" outside the parameters of a macro is highlighted as a modifier');
      }
    }
  });
  if (stack.depth !== topDepth) errors.push(`${rel}: a grammar rule is still open at the end of the file`);
}

if (files.length === 0) errors.push(`no .gp files found in ${examplesDir}`);
for (const e of errors) console.error(e);
if (errors.length > 0) {
  console.error(`check-examples: ${errors.length} problem(s) in ${files.length} files`);
  process.exit(1);
}
console.log(`check-examples: ${files.length} files, ${lineCount} lines: ok`);
