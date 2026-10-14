// fakes.js has the fakes that extension.test.js and lsp.test.js run
// src/extension.js with: a fake vscode module, a fake
// vscode-languageclient/node, load, which loads the extension with them,
// and realClient, which loads the real vscode-languageclient/node with the
// fake vscode.
'use strict';

const assert = require('node:assert');
const Module = require('node:module');

// fakeVSCode returns a fake vscode module with the settings settings (the
// keys without "gocpp."). choices maps a message to the button that the
// user clicks.
function fakeVSCode(settings, { trusted = true, choices = {}, folder = __dirname } = {}) {
	const log = { errors: [], infos: [], warnings: [], executed: [], output: [] };
	const listeners = { config: [], trust: [] };
	const commands = new Map();
	const disposable = { dispose() {} };
	const show = (list) => async (message, ...items) => {
		list.push({ message, items });
		for (const [part, choice] of Object.entries(choices)) {
			if (message.includes(part)) {
				return choice;
			}
		}
		return undefined;
	};
	const vscode = {
		// The classes that vscode-languageclient extends when it loads, so
		// that lsp.test.js can load the real one with this fake.
		CompletionItem: class {},
		CodeLens: class {},
		DocumentLink: class {},
		CodeAction: class {},
		Diagnostic: class {},
		CallHierarchyItem: class {},
		TypeHierarchyItem: class {},
		SymbolInformation: class {},
		InlayHint: class {},
		CancellationError: class extends Error {},
		log,
		listeners,
		commands: {
			registerCommand(id, fn) {
				commands.set(id, fn);
				return disposable;
			},
			async executeCommand(id, ...args) {
				log.executed.push([id, ...args]);
			},
			registered: commands,
		},
		window: {
			createOutputChannel(name) {
				assert.strictEqual(name, 'gocpp');
				return { appendLine: (s) => log.output.push(s), dispose() {} };
			},
			showErrorMessage: show(log.errors),
			showInformationMessage: show(log.infos),
			showWarningMessage: show(log.warnings),
		},
		workspace: {
			isTrusted: trusted,
			workspaceFolders: folder ? [{ uri: { scheme: 'file', fsPath: folder } }] : undefined,
			getConfiguration(section) {
				assert.strictEqual(section, 'gocpp');
				return { get: (key) => settings[key] };
			},
			onDidChangeConfiguration(fn) {
				listeners.config.push(fn);
				return disposable;
			},
			onDidGrantWorkspaceTrust(fn) {
				listeners.trust.push(fn);
				return disposable;
			},
		},
	};
	return vscode;
}

// fakeClient returns a fake vscode-languageclient/node that keeps every
// LanguageClient it makes in clients.
function fakeClient() {
	const clients = [];
	class LanguageClient {
		constructor(id, name, serverOptions, clientOptions) {
			Object.assign(this, { id, name, serverOptions, clientOptions, started: false, disposed: false });
			clients.push(this);
		}
		async start() {
			this.started = true;
		}
		async dispose() {
			this.disposed = true;
		}
	}
	return { clients, LanguageClient };
}

// load loads a fresh src/extension.js that gets the fakes for vscode and
// vscode-languageclient/node.
function load(t, vscode, lc) {
	const original = Module._load;
	Module._load = function (request, parent, isMain) {
		if (request === 'vscode') {
			return vscode;
		}
		if (request === 'vscode-languageclient/node') {
			return lc;
		}
		return original.call(this, request, parent, isMain);
	};
	t.after(() => {
		Module._load = original;
	});
	const file = require.resolve('../src/extension.js');
	delete require.cache[file];
	delete require.cache[require.resolve('../src/server.js')];
	return require(file);
}

function context() {
	return { subscriptions: [] };
}

// realClient loads the real vscode-languageclient/node with the fake vscode
// module vscode. The client keeps that fake: it gets vscode once, when it
// loads.
function realClient(vscode) {
	const original = Module._load;
	Module._load = function (request, parent, isMain) {
		return request === 'vscode' ? vscode : original.call(this, request, parent, isMain);
	};
	try {
		return require('vscode-languageclient/node');
	} finally {
		Module._load = original;
	}
}

module.exports = { fakeVSCode, fakeClient, load, context, realClient };
