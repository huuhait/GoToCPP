// extension.test.js runs src/extension.js in plain Node.js, with fakes for
// the vscode module and for vscode-languageclient/node. It checks what the
// extension does with them: the client it creates (id, command, arguments,
// environment, documents), the restart command, the error when the binary
// is not found, and Restricted Mode. lsp.test.js starts the real server.
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const { fakeVSCode, fakeClient, load, context } = require('./fakes');

// A real executable for gocpp.path on every OS: node itself.
const someBinary = process.execPath;

test('activate starts "gocpp lsp" for the gocpp files, with no transport flag', async (t) => {
	const vscode = fakeVSCode({ path: someBinary, 'lsp.args': ['-logfile', 'x.log'], 'lsp.env': { GOCPP_A: 'b' } });
	const lc = fakeClient();
	const ext = load(t, vscode, lc);
	const ctx = context();
	await ext.activate(ctx);

	assert.strictEqual(lc.clients.length, 1);
	const c = lc.clients[0];
	assert.strictEqual(c.id, 'gocpp'); // so the client reads gocpp.trace.server
	assert.ok(c.started);
	assert.strictEqual(c.serverOptions.command, someBinary);
	assert.deepStrictEqual(c.serverOptions.args, ['lsp', '-logfile', 'x.log']);
	// No transport: with TransportKind.stdio the client would add "--stdio",
	// which gocpp lsp does not take. lsp.test.js runs the real client's spawn.
	assert.strictEqual(c.serverOptions.transport, undefined);
	assert.strictEqual(c.serverOptions.options.env.GOCPP_A, 'b');
	assert.strictEqual(c.serverOptions.options.env.PATH, process.env.PATH);
	assert.deepStrictEqual(c.clientOptions.documentSelector, [{ scheme: 'file', language: 'gocpp' }]);
	assert.ok(c.clientOptions.outputChannel, 'no output channel');
	assert.ok(vscode.commands.registered.has('gocpp.restartServer'));
	assert.deepStrictEqual(vscode.log.errors, []);

	await ext.deactivate();
	assert.ok(c.disposed, 'deactivate did not stop the client');
});

test('the restart command stops the client and starts a new one with the new settings', async (t) => {
	const settings = { path: someBinary, 'lsp.args': [] };
	const vscode = fakeVSCode(settings);
	const lc = fakeClient();
	const ext = load(t, vscode, lc);
	await ext.activate(context());
	settings['lsp.args'] = ['-logfile', 'new.log'];
	await vscode.commands.registered.get('gocpp.restartServer')();
	assert.strictEqual(lc.clients.length, 2);
	assert.ok(lc.clients[0].disposed);
	assert.ok(lc.clients[1].started);
	assert.deepStrictEqual(lc.clients[1].serverOptions.args, ['lsp', '-logfile', 'new.log']);
	// Two restarts at once run one after the other.
	const restart = vscode.commands.registered.get('gocpp.restartServer');
	await Promise.all([restart(), restart()]);
	assert.strictEqual(lc.clients.length, 4);
	assert.deepStrictEqual(lc.clients.map((c) => c.disposed), [true, true, true, false]);
});

test('a changed gocpp.path offers a restart', async (t) => {
	const vscode = fakeVSCode({ path: someBinary }, { choices: { 'settings changed': 'Restart' } });
	const lc = fakeClient();
	const ext = load(t, vscode, lc);
	await ext.activate(context());
	const [onChange] = vscode.listeners.config;
	await onChange({ affectsConfiguration: (s) => s === 'editor.tabSize' });
	assert.strictEqual(vscode.log.infos.length, 0);
	assert.strictEqual(lc.clients.length, 1);
	await onChange({ affectsConfiguration: (s) => s === 'gocpp.path' });
	assert.strictEqual(vscode.log.infos.length, 1);
	assert.deepStrictEqual(vscode.log.infos[0].items, ['Restart']);
	assert.strictEqual(lc.clients.length, 2);
});

test('gocpp not found: an error that names gocpp.path, with a button that opens it', async (t) => {
	const missing = path.join(__dirname, 'no-such-dir', 'gocpp');
	const vscode = fakeVSCode({ path: missing }, { choices: { 'Cannot start': 'Open Settings' } });
	const lc = fakeClient();
	const ext = load(t, vscode, lc);
	await ext.activate(context());
	await new Promise((resolve) => setImmediate(resolve)); // the button's command
	assert.strictEqual(lc.clients.length, 0);
	assert.strictEqual(vscode.log.errors.length, 1);
	const { message, items } = vscode.log.errors[0];
	assert.match(message, /"gocpp\.path"/);
	assert.ok(message.includes(missing), message);
	assert.deepStrictEqual(items, ['Open Settings']);
	assert.deepStrictEqual(vscode.log.executed, [['workbench.action.openSettings', 'gocpp.path']]);
	assert.ok(vscode.log.output.some((s) => s.includes(missing)), 'the error is not in the output channel');
	await ext.deactivate(); // nothing to stop
});

test('Restricted Mode: no server until the workspace is trusted', async (t) => {
	const vscode = fakeVSCode({ path: someBinary }, { trusted: false });
	const lc = fakeClient();
	const ext = load(t, vscode, lc);
	await ext.activate(context());
	assert.strictEqual(lc.clients.length, 0);
	await vscode.commands.registered.get('gocpp.restartServer')();
	assert.strictEqual(lc.clients.length, 0);
	assert.strictEqual(vscode.log.warnings.length, 1);
	vscode.workspace.isTrusted = true;
	await vscode.listeners.trust[0]();
	assert.strictEqual(lc.clients.length, 1);
	assert.ok(lc.clients[0].started);
});
