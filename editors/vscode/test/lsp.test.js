// lsp.test.js starts the real language server the way VS Code does: it
// builds gocpp from this repository, runs src/extension.js with a fake
// vscode module to get the server options it gives the client, and starts
// the server with the real vscode-languageclient's createMessageTransports,
// the code that turns those options into a command line and spawns it. So
// an argument that the client adds and gocpp lsp does not take (such as
// "--stdio") fails this test. Then it does what VS Code does at the start
// and at the end: initialize, open a file and wait for its diagnostics,
// shutdown and exit.
//
// GOCPP_TEST_BIN can name a gocpp binary to use instead of building one.
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const cp = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const rpc = require('vscode-jsonrpc/node');
const { fakeVSCode, load, context, realClient } = require('./fakes');

const repoRoot = path.resolve(__dirname, '..', '..', '..');

// gocppBinary returns the gocpp binary to test, in dir.
function gocppBinary(dir) {
	if (process.env.GOCPP_TEST_BIN) {
		return path.resolve(process.env.GOCPP_TEST_BIN);
	}
	const bin = path.join(dir, process.platform === 'win32' ? 'gocpp.exe' : 'gocpp');
	const r = cp.spawnSync('go', ['build', '-o', bin, './cmd/gocpp'], { cwd: repoRoot, stdio: ['ignore', 'inherit', 'inherit'] });
	if (r.error) {
		throw new Error(`go build: ${r.error.message} (run the tests in the dev shell, "nix develop", or set GOCPP_TEST_BIN)`);
	}
	assert.strictEqual(r.status, 0, 'go build ./cmd/gocpp failed');
	return bin;
}

// withTimeout fails when promise takes longer than ms.
function withTimeout(promise, ms, what) {
	let timer;
	const timeout = new Promise((_, reject) => {
		timer = setTimeout(() => reject(new Error(`timeout: ${what} after ${ms} ms`)), ms);
	});
	return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

test('gocpp lsp, started by the real client from the extension\'s options: initialize, diagnostics, shutdown and exit', async (t) => {
	const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gocpp-vscode-test-'));
	t.after(() => fs.rmSync(tmp, { recursive: true, force: true }));
	const bin = gocppBinary(tmp);

	// A project with an error that gocpp check finds.
	const project = path.join(tmp, 'hello');
	fs.mkdirSync(project);
	const text = 'package main\n\nfunc main() {\n\tx := 1\n}\n';
	const file = path.join(project, 'main.gp');
	fs.writeFileSync(file, text);
	const uri = pathToFileURL(file).href;

	// The settings: the binary by path, a log file through gocpp.lsp.args,
	// and a variable through gocpp.lsp.env.
	const logfile = path.join(tmp, 'lsp.log');
	const vscode = fakeVSCode(
		{ path: bin, 'lsp.args': ['-logfile', logfile], 'lsp.env': { GOCPP_VSCODE_TEST: '1' } },
		{ folder: project },
	);

	// The extension makes its LanguageClient with the real client module,
	// except that the LanguageClient class keeps its options and does not
	// start. Making the real class needs much more of VS Code.
	const lc = realClient(vscode);
	const made = [];
	class KeepOptions {
		constructor(id, name, serverOptions, clientOptions) {
			made.push({ serverOptions, clientOptions });
		}
		async start() {}
		async dispose() {}
	}
	const ext = load(t, vscode, { ...lc, LanguageClient: KeepOptions });
	await ext.activate(context());
	assert.deepStrictEqual(vscode.log.errors, []);
	assert.strictEqual(made.length, 1);
	const { serverOptions, clientOptions } = made[0];

	// Start the server with the real client's createMessageTransports. Its
	// "this" needs only the fields that this method reads.
	const proto = lc.LanguageClient.prototype;
	let stderr = '';
	const host = {
		_serverOptions: serverOptions,
		_forceDebug: false,
		clientOptions,
		outputChannel: { append: (s) => { stderr += s; } },
		// It reports the exit of the server with error.
		error: (message) => { stderr += `[client] ${message}\n`; },
		_getServerWorkingDir: proto._getServerWorkingDir,
		_mainGetRootPath: proto._mainGetRootPath,
	};
	const { reader, writer } = await proto.createMessageTransports.call(host, 'utf8');
	const proc = host._serverProcess;
	assert.ok(proc, 'createMessageTransports started no process');
	t.after(() => proc.kill());
	// "close" comes after the last of stderr, so a failure shows all of it.
	const exited = new Promise((resolve) => proc.on('close', (code) => resolve(code)));
	// The command line the client ran, and the environment it got.
	assert.deepStrictEqual(proc.spawnargs, [bin, 'lsp', '-logfile', logfile]);
	assert.strictEqual(serverOptions.options.env.GOCPP_VSCODE_TEST, '1');

	const conn = rpc.createMessageConnection(reader, writer);
	const diagnostics = new Promise((resolve) => {
		conn.onNotification('textDocument/publishDiagnostics', (p) => {
			if (p.uri === uri && p.diagnostics.length > 0) {
				resolve(p.diagnostics);
			}
		});
	});
	// The server asks to watch files (client/registerCapability), as VS Code allows.
	conn.onRequest('client/registerCapability', () => null);
	conn.listen();

	// A server that exits at once (a flag it does not take, say) fails here
	// with its stderr, not with a timeout.
	const died = exited.then((code) => {
		throw new Error(`gocpp lsp exited with ${code} before it answered initialize; stderr: ${stderr}`);
	});
	died.catch(() => {});
	const init = await withTimeout(Promise.race([died, conn.sendRequest('initialize', {
		processId: process.pid,
		clientInfo: { name: 'gocpp-vscode-test' },
		rootUri: pathToFileURL(project).href,
		workspaceFolders: [{ uri: pathToFileURL(project).href, name: 'hello' }],
		capabilities: {
			workspace: { didChangeWatchedFiles: { dynamicRegistration: true } },
			textDocument: { publishDiagnostics: { relatedInformation: true } },
		},
		trace: 'off',
	})]), 10000, 'initialize');
	assert.strictEqual(init.serverInfo?.name, 'gocpp');
	assert.ok(init.capabilities.textDocumentSync, 'the server has no textDocumentSync');
	await conn.sendNotification('initialized', {});

	await conn.sendNotification('textDocument/didOpen', {
		textDocument: { uri, languageId: 'gocpp', version: 1, text },
	});
	const diags = await withTimeout(diagnostics, 30000, 'diagnostics of main.gp');
	assert.ok(diags.some((d) => /declared and not used/.test(d.message) && d.range.start.line === 3),
		`no diagnostic for the unused x on line 4: ${JSON.stringify(diags)}`);

	await withTimeout(conn.sendRequest('shutdown'), 10000, 'shutdown');
	await conn.sendNotification('exit');
	const code = await withTimeout(exited, 10000, 'the server exits');
	conn.dispose();
	assert.strictEqual(code, 0, `gocpp lsp exited with ${code}; stderr: ${stderr}`);

	// gocpp.lsp.args reached the server: the log is in the file.
	assert.match(fs.readFileSync(logfile, 'utf8'), /gocpp-vscode-test/);
});
