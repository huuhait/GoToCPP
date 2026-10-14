// manifest.test.js checks package.json against what VS Code needs from it
// and against what the code and README.md say: the main file, the
// activation event, the commands, the settings, format on save and the
// breakpoints.
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const dir = path.join(__dirname, '..');
const pkg = JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
const readme = fs.readFileSync(path.join(dir, 'README.md'), 'utf8');
const extensionSource = fs.readFileSync(path.join(dir, 'src', 'extension.js'), 'utf8');

// exact matches an exact version, such as "9.0.1".
const exact = /^\d+\.\d+\.\d+$/;

// caret returns the version of a range "^X.Y.Z" as numbers [X, Y, Z]. The
// package.json files here use no other form of range for engines.vscode.
function caret(range) {
	const m = /^\^(\d+)\.(\d+)\.(\d+)$/.exec(range);
	assert.ok(m, `"${range}" is not a range "^X.Y.Z"`);
	return m.slice(1).map(Number);
}

test('main exists and exports activate and deactivate', () => {
	const main = path.join(dir, pkg.main);
	assert.ok(fs.existsSync(main), `main ${pkg.main} does not exist`);
	const src = fs.readFileSync(main, 'utf8');
	assert.match(src, /module\.exports = \{ activate, deactivate \}/);
});

test('the extension starts when a gocpp file opens', () => {
	assert.deepStrictEqual(pkg.activationEvents, ['onLanguage:gocpp']);
	assert.ok(pkg.contributes.languages.some((l) => l.id === 'gocpp' && l.extensions.includes('.gp')));
});

test('the dependencies are pinned, and VS Code is new enough for vscode-languageclient', () => {
	for (const deps of [pkg.dependencies, pkg.devDependencies]) {
		for (const [name, version] of Object.entries(deps)) {
			assert.match(version, exact, `${name}: "${version}" is not an exact version`);
		}
	}
	const lc = require('vscode-languageclient/package.json');
	assert.strictEqual(lc.version, pkg.dependencies['vscode-languageclient']);
	// The oldest VS Code that "^X.Y.Z" allows is X.Y.Z; it must be in the
	// client's range: the same X, and Y.Z as new or newer.
	const ours = caret(pkg.engines.vscode);
	const needs = caret(lc.engines.vscode);
	const newEnough = ours[0] === needs[0] && (ours[1] > needs[1] || (ours[1] === needs[1] && ours[2] >= needs[2]));
	assert.ok(newEnough,
		`engines.vscode ${pkg.engines.vscode} allows VS Code ${ours.join('.')}, but vscode-languageclient ${lc.version} needs ${lc.engines.vscode}`);
	// The extension uses the Node.js client, which spawns the server.
	// (vscode-languageclient/node is node.js, which is require('./lib/node/main').)
	require.resolve('vscode-languageclient/node');
	const main = fs.readFileSync(path.join(path.dirname(require.resolve('vscode-languageclient/package.json')), 'lib', 'node', 'main.js'), 'utf8');
	assert.match(main, /exports\.LanguageClient = LanguageClient/);
});

test('every command is registered by the code and has the gocpp category', () => {
	assert.ok(pkg.contributes.commands.length > 0);
	for (const c of pkg.contributes.commands) {
		assert.ok(extensionSource.includes(`registerCommand('${c.command}'`), `${c.command} is not registered`);
		assert.strictEqual(c.category, 'gocpp');
		assert.ok(readme.includes(`gocpp: ${c.title}`), `README.md does not name the command "gocpp: ${c.title}"`);
	}
	assert.ok(pkg.contributes.commands.some((c) => c.command === 'gocpp.restartServer' && c.title === 'Restart Language Server'));
});

test('the settings: each one is described, has a default of its type, and is in README.md', () => {
	const props = pkg.contributes.configuration.properties;
	assert.deepStrictEqual(Object.keys(props).sort(), ['gocpp.lsp.args', 'gocpp.lsp.env', 'gocpp.path', 'gocpp.trace.server']);
	const typeOf = (v) => (Array.isArray(v) ? 'array' : typeof v);
	for (const [key, p] of Object.entries(props)) {
		assert.ok(p.markdownDescription || p.description, `${key} has no description`);
		assert.strictEqual(typeOf(p.default), p.type, `${key}: the default is not a ${p.type}`);
		assert.ok(readme.includes('`' + key + '`'), `README.md does not describe ${key}`);
		if (key !== 'gocpp.trace.server') {
			assert.ok(extensionSource.includes(`'${key.slice('gocpp.'.length)}'`), `src/extension.js does not read ${key}`);
		}
	}
	assert.strictEqual(props['gocpp.path'].default, 'gocpp');
	// vscode-languageclient reads <client id>.trace.server with these values.
	assert.deepStrictEqual(props['gocpp.trace.server'].enum, ['off', 'messages', 'verbose']);
	assert.match(extensionSource, /new LanguageClient\('gocpp',/);
	// Restricted Mode must not take the start settings from the workspace.
	assert.deepStrictEqual(pkg.capabilities.untrustedWorkspaces.restrictedConfigurations.sort(),
		['gocpp.lsp.args', 'gocpp.lsp.env', 'gocpp.path']);
});

test('format on save and organize imports on save, like the Go extension', () => {
	assert.deepStrictEqual(pkg.contributes.configurationDefaults['[gocpp]'], {
		'editor.insertSpaces': false,
		'editor.formatOnSave': true,
		'editor.formatOnSaveMode': 'file',
		'editor.codeActionsOnSave': { 'source.organizeImports': 'explicit' },
	});
});

test('breakpoints can be set in gocpp files', () => {
	assert.deepStrictEqual(pkg.contributes.breakpoints, [{ language: 'gocpp' }]);
});
