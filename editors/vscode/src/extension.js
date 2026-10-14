// extension.js is the VS Code extension for gocpp: it starts the gocpp
// language server ("gocpp lsp") for .gp files and talks to it over standard
// input and output with vscode-languageclient. The features (diagnostics,
// hover, formatting, ...) come from the server's capabilities, so a new
// server feature needs no change here.
//
// It is plain JavaScript (CommonJS), so it needs no build step.
'use strict';

const vscode = require('vscode');
const { LanguageClient } = require('vscode-languageclient/node');
const { SettingsError, serverCommand } = require('./server');

// The settings that change how the server starts. When one changes, the
// extension offers to restart the server.
const startSettings = ['gocpp.path', 'gocpp.lsp.args', 'gocpp.lsp.env'];

let client; // the running LanguageClient, or undefined
let output; // the "gocpp" output channel: the client's log and the server's stderr
let queue = Promise.resolve(); // start and stop run one after the other

// serialize runs fn after the start or stop before it is done.
//
// So a restart or deactivate waits for a client.start() that is still
// running: if the server never answers initialize, a restart does nothing
// and deactivate does not return. There is no start timeout because
// vscode-languageclient 9 cannot stop a client that is still starting;
// the fix for such a server is to fix the server (or reload the window).
function serialize(fn) {
	const next = queue.then(fn);
	queue = next.catch(() => {});
	return next;
}

// workspaceDir is the folder of the first workspace folder, if it is on disk.
function workspaceDir() {
	const folder = vscode.workspace.workspaceFolders?.[0];
	return folder?.uri.scheme === 'file' ? folder.uri.fsPath : undefined;
}

// showSettingsError shows err with a button that opens its setting.
async function showSettingsError(err) {
	output.appendLine(err.message);
	const open = 'Open Settings';
	const choice = await vscode.window.showErrorMessage(err.message, open);
	if (choice === open) {
		await vscode.commands.executeCommand('workbench.action.openSettings', err.setting);
	}
}

// start starts the language server with the current settings. When the
// settings are wrong or the binary is not found, it shows an error and
// starts nothing.
async function start() {
	const config = vscode.workspace.getConfiguration('gocpp');
	let cmd;
	try {
		cmd = serverCommand(
			{ path: config.get('path'), args: config.get('lsp.args'), env: config.get('lsp.env') },
			{ cwd: workspaceDir() },
		);
	} catch (err) {
		if (err instanceof SettingsError) {
			void showSettingsError(err);
			return;
		}
		throw err;
	}
	output.appendLine(`Starting ${cmd.command} ${cmd.args.join(' ')}`);
	// No transport: the client then talks to the server over its standard
	// input and output and adds no argument. TransportKind.stdio would add
	// "--stdio", a flag that "gocpp lsp" does not have. vscode-go leaves it
	// out for gopls too.
	const serverOptions = {
		command: cmd.command,
		args: cmd.args,
		options: { env: cmd.env },
	};
	const clientOptions = {
		// The server reads files from disk, so it takes file URIs only.
		documentSelector: [{ scheme: 'file', language: 'gocpp' }],
		outputChannel: output,
	};
	// The id "gocpp" makes the client read the setting gocpp.trace.server.
	client = new LanguageClient('gocpp', 'gocpp', serverOptions, clientOptions);
	try {
		await client.start();
	} catch (err) {
		// The client has shown the error and written it to the output.
		output.appendLine(`The language server did not start: ${err?.message ?? err}`);
	}
}

// stop stops the language server, if it runs.
async function stop() {
	const c = client;
	client = undefined;
	if (c === undefined) {
		return;
	}
	try {
		await c.dispose();
	} catch (err) {
		// A client that failed to start cannot be stopped; there is nothing to stop.
		output.appendLine(`Stopping the language server: ${err?.message ?? err}`);
	}
}

// restart stops the language server and starts it again with the current
// settings (the command "gocpp: Restart Language Server").
function restart() {
	if (!vscode.workspace.isTrusted) {
		void vscode.window.showWarningMessage('The gocpp language server does not run in Restricted Mode. Trust the workspace to start it.');
		return Promise.resolve();
	}
	return serialize(async () => {
		await stop();
		await start();
	});
}

function activate(context) {
	output = vscode.window.createOutputChannel('gocpp');
	context.subscriptions.push(
		output,
		vscode.commands.registerCommand('gocpp.restartServer', restart),
		vscode.workspace.onDidChangeConfiguration(async (e) => {
			if (!vscode.workspace.isTrusted || !startSettings.some((s) => e.affectsConfiguration(s))) {
				return;
			}
			const choice = await vscode.window.showInformationMessage(
				'The gocpp language server settings changed. Restart the server to use them.', 'Restart');
			if (choice === 'Restart') {
				await restart();
			}
		}),
	);
	if (!vscode.workspace.isTrusted) {
		// Restricted Mode: the server would run the C++ compiler on the
		// headers of a folder that the user does not trust yet.
		output.appendLine('The workspace is not trusted (Restricted Mode): the language server starts when you trust it.');
		context.subscriptions.push(vscode.workspace.onDidGrantWorkspaceTrust(restart));
		return undefined;
	}
	return serialize(start);
}

function deactivate() {
	return serialize(stop);
}

module.exports = { activate, deactivate };
