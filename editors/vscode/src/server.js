// server.js works out how the extension starts the language server: the
// gocpp binary, its arguments and its environment, from the settings
// gocpp.path, gocpp.lsp.args and gocpp.lsp.env. It does not use the vscode
// module, so the tests can run it in plain Node.js (test/server.test.js).
'use strict';

const fs = require('node:fs');
const path = require('node:path');

// SettingsError is a problem with one setting: the binary is not found, or
// a value has the wrong type. The extension shows the message with a button
// that opens the setting.
class SettingsError extends Error {
	constructor(setting, message) {
		super(message);
		this.name = 'SettingsError';
		this.setting = setting;
	}
}

// envKey returns the name under which env has the variable name: name itself,
// or on Windows, where names do not depend on case, the name as env spells
// it ("Path" for "PATH"). It returns name when env does not have it.
function envKey(env, name, platform) {
	if (platform !== 'win32') {
		return name;
	}
	const upper = name.toUpperCase();
	return Object.keys(env).find((k) => k.toUpperCase() === upper) ?? name;
}

// mergeEnv returns a copy of base with the variables of extra (the setting
// gocpp.lsp.env) set. Numbers and booleans become strings; any other type is
// a SettingsError.
function mergeEnv(base, extra, platform) {
	const env = { ...base };
	if (extra === undefined || extra === null) {
		return env;
	}
	if (typeof extra !== 'object' || Array.isArray(extra)) {
		throw new SettingsError('gocpp.lsp.env', 'The setting "gocpp.lsp.env" must be an object of environment variables, like { "GOCPP_CACHE": "/tmp/gocpp" }.');
	}
	for (const [name, value] of Object.entries(extra)) {
		if (typeof value !== 'string' && typeof value !== 'number' && typeof value !== 'boolean') {
			throw new SettingsError('gocpp.lsp.env', `The variable "${name}" in the setting "gocpp.lsp.env" must be a string.`);
		}
		const key = envKey(env, name, platform);
		delete env[key];
		env[name] = String(value);
	}
	return env;
}

// isExecutableFile says whether file is a file that can be run.
function isExecutableFile(file, platform) {
	try {
		if (!fs.statSync(file).isFile()) {
			return false;
		}
		if (platform !== 'win32') {
			fs.accessSync(file, fs.constants.X_OK);
		}
		return true;
	} catch {
		return false;
	}
}

// candidates returns the file names to try for file: file itself, and on
// Windows, when file has no extension, file.exe and file.com. PATHEXT is not
// used: its .bat and .cmd files need a shell, and since CVE-2024-27980
// Node.js refuses to spawn them without one (EINVAL), so such a file would
// be found and then fail to start.
function candidates(file, platform) {
	if (platform !== 'win32' || path.win32.extname(file) !== '') {
		return [file];
	}
	return [file + '.exe', file + '.com'];
}

// unquote takes one pair of double quotes off s: a path with spaces that was pasted from a Windows
// shell ("C:\\Program Files\\gocpp\\gocpp.exe", quotes included), or an entry of PATH written that way.
function unquote(s) {
	s = s.trim();
	return s.length >= 2 && s.startsWith('"') && s.endsWith('"') ? s.slice(1, -1) : s;
}

// findBinary returns the full path of the binary bin (the setting
// gocpp.path), or undefined when it is not there. A bin with a path
// separator is a path: "~" at its start is the home directory, and a
// relative path is relative to cwd (the first workspace folder). A bin
// without one is a command name, looked up in the PATH of env, as a shell
// does.
function findBinary(bin, opts) {
	const { env, platform, homedir, cwd } = opts;
	const isExec = opts.isExecutable ?? ((f) => isExecutableFile(f, platform));
	const p = platform === 'win32' ? path.win32 : path.posix;
	if (platform === 'win32') {
		bin = unquote(bin);
	}
	const hasSep = bin.includes('/') || (platform === 'win32' && bin.includes('\\'));
	if (bin === '~' || hasSep || p.isAbsolute(bin)) {
		let file = bin;
		if (homedir && (bin === '~' || bin.startsWith('~/') || (platform === 'win32' && bin.startsWith('~\\')))) {
			file = p.join(homedir, bin.slice(1));
		}
		if (!p.isAbsolute(file)) {
			if (!cwd) {
				return undefined;
			}
			file = p.resolve(cwd, file);
		}
		file = p.normalize(file);
		const found = candidates(file, platform).find(isExec);
		if (found !== undefined || platform !== 'win32') {
			return found;
		}
		// The setting names the folder of gocpp.exe ("C:\\Program Files\\gocpp\\bin").
		return candidates(p.join(file, 'gocpp'), platform).find(isExec);
	}
	const dirs = (env[envKey(env, 'PATH', platform)] || '').split(p.delimiter);
	for (let dir of dirs) {
		if (platform === 'win32') {
			dir = unquote(dir); // PATH entries with spaces are often quoted on Windows
		}
		if (dir === '') {
			continue; // like Go's exec.LookPath, an empty entry is not "."
		}
		const found = candidates(p.join(dir, bin), platform).find(isExec);
		if (found) {
			return found;
		}
	}
	return undefined;
}

// notFoundMessage is the error for a gocpp.path that names no binary.
function notFoundMessage(bin) {
	const fix = 'Install gocpp ("make build" in the gocpp repository writes bin/gocpp), then put it in PATH or set "gocpp.path" to its full path.';
	if (bin === 'gocpp') {
		return `Cannot start the gocpp language server: "gocpp" is not in PATH. ${fix}`;
	}
	return `Cannot start the gocpp language server: the setting "gocpp.path" is "${bin}", and there is no gocpp binary there. ${fix}`;
}

// serverCommand returns how to start the language server, as
// { command, args, env }: command is the full path of the binary, args are
// "lsp" and then the setting gocpp.lsp.args, and env is the environment of
// VS Code with the setting gocpp.lsp.env set. The binary is looked up in
// the PATH of that environment. settings is { path, args, env } with the
// values of the three settings; opts can give platform, env, homedir, cwd
// and isExecutable (for the tests). It throws a SettingsError when a setting
// is wrong or the binary is not found.
function serverCommand(settings, opts = {}) {
	const platform = opts.platform ?? process.platform;
	const homedir = opts.homedir ?? require('node:os').homedir();
	const env = mergeEnv(opts.env ?? process.env, settings.env, platform);

	let bin = settings.path ?? '';
	if (typeof bin !== 'string') {
		throw new SettingsError('gocpp.path', 'The setting "gocpp.path" must be a string: the gocpp binary, like "gocpp" or "/usr/local/bin/gocpp".');
	}
	bin = bin.trim();
	if (bin === '') {
		bin = 'gocpp';
	}
	const extra = settings.args ?? [];
	if (!Array.isArray(extra) || !extra.every((a) => typeof a === 'string')) {
		throw new SettingsError('gocpp.lsp.args', 'The setting "gocpp.lsp.args" must be a list of strings, like ["-logfile", "/tmp/gocpp-lsp.log"].');
	}
	const command = findBinary(bin, { env, platform, homedir, cwd: opts.cwd, isExecutable: opts.isExecutable });
	if (command === undefined) {
		throw new SettingsError('gocpp.path', notFoundMessage(bin));
	}
	return { command, args: ['lsp', ...extra], env };
}

module.exports = { SettingsError, serverCommand, findBinary, mergeEnv, notFoundMessage };
