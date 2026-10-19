// server.test.js tests src/server.js: how the settings gocpp.path,
// gocpp.lsp.args and gocpp.lsp.env become the command that starts the
// language server, and the message when the binary is not found. The file
// system is a fake (isExecutable), so the tests do the same on every OS.
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { serverCommand, SettingsError } = require('../src/server');

// fake returns the options for serverCommand with a file system that has
// only the executable files files.
function fake(platform, env, files, extra = {}) {
	const set = new Set(files);
	return { platform, env, homedir: platform === 'win32' ? 'C:\\Users\\me' : '/home/me', isExecutable: (f) => set.has(f), ...extra };
}

test('the default finds gocpp in PATH and runs "gocpp lsp"', () => {
	const opts = fake('linux', { PATH: '/usr/bin:/opt/gocpp/bin', HOME: '/home/me' }, ['/opt/gocpp/bin/gocpp']);
	for (const p of [undefined, '', 'gocpp', '  gocpp ']) {
		const cmd = serverCommand({ path: p }, opts);
		assert.strictEqual(cmd.command, '/opt/gocpp/bin/gocpp');
		assert.deepStrictEqual(cmd.args, ['lsp']);
		assert.deepStrictEqual(cmd.env, { PATH: '/usr/bin:/opt/gocpp/bin', HOME: '/home/me' });
	}
});

test('the first gocpp in PATH wins; empty PATH entries are skipped', () => {
	const opts = fake('linux', { PATH: ':/a::/b' }, ['/a/gocpp', '/b/gocpp', 'gocpp']);
	assert.strictEqual(serverCommand({}, opts).command, '/a/gocpp');
	const opts2 = fake('linux', { PATH: '::/b' }, ['gocpp', '/b/gocpp']);
	assert.strictEqual(serverCommand({}, opts2).command, '/b/gocpp');
});

test('gocpp.lsp.args come after "lsp"', () => {
	const opts = fake('linux', { PATH: '/bin' }, ['/bin/gocpp']);
	const cmd = serverCommand({ args: ['-logfile', '/tmp/x.log'] }, opts);
	assert.deepStrictEqual(cmd.args, ['lsp', '-logfile', '/tmp/x.log']);
});

test('gocpp.path as a path: absolute, ~ and relative to the workspace folder', () => {
	const files = ['/usr/local/bin/gocpp', '/home/me/go/bin/gocpp', '/work/proj/bin/gocpp'];
	const opts = fake('linux', { PATH: '/bin' }, files, { cwd: '/work/proj' });
	assert.strictEqual(serverCommand({ path: '/usr/local/bin/gocpp' }, opts).command, '/usr/local/bin/gocpp');
	assert.strictEqual(serverCommand({ path: '~/go/bin/gocpp' }, opts).command, '/home/me/go/bin/gocpp');
	assert.strictEqual(serverCommand({ path: './bin/gocpp' }, opts).command, '/work/proj/bin/gocpp');
	assert.strictEqual(serverCommand({ path: 'bin/gocpp' }, opts).command, '/work/proj/bin/gocpp');
	// A path is not looked up in PATH.
	const opts2 = fake('linux', { PATH: '/usr/local/bin' }, files);
	assert.throws(() => serverCommand({ path: 'bin/gocpp' }, opts2), SettingsError);
});

test('another name in gocpp.path is looked up in PATH', () => {
	const opts = fake('linux', { PATH: '/bin:/opt/bin' }, ['/opt/bin/gocpp-dev']);
	assert.strictEqual(serverCommand({ path: 'gocpp-dev' }, opts).command, '/opt/bin/gocpp-dev');
});

test('gocpp.lsp.env is added to the environment, and its PATH finds the binary', () => {
	const opts = fake('linux', { PATH: '/bin', HOME: '/home/me' }, ['/nix/bin/gocpp']);
	const cmd = serverCommand({ env: { PATH: '/nix/bin', GOCPP_X: 'y', N: 3, B: true } }, opts);
	assert.strictEqual(cmd.command, '/nix/bin/gocpp');
	assert.deepStrictEqual(cmd.env, { PATH: '/nix/bin', HOME: '/home/me', GOCPP_X: 'y', N: '3', B: 'true' });
	// The environment of VS Code is not changed.
	assert.deepStrictEqual(opts.env, { PATH: '/bin', HOME: '/home/me' });
});

test('Windows: .exe and .com, ";" in PATH, names of variables in any case', () => {
	const env = { Path: 'C:\\Windows;C:\\gocpp\\bin', PATHEXT: '.COM;.EXE;.BAT;.CMD' };
	const opts = fake('win32', env, ['C:\\gocpp\\bin\\gocpp.exe', 'D:\\tools\\gocpp.exe', 'C:\\Users\\me\\bin\\gocpp.exe']);
	assert.strictEqual(serverCommand({}, opts).command, 'C:\\gocpp\\bin\\gocpp.exe');
	assert.strictEqual(serverCommand({ path: 'D:\\tools\\gocpp' }, opts).command, 'D:\\tools\\gocpp.exe');
	assert.strictEqual(serverCommand({ path: 'D:\\tools\\gocpp.exe' }, opts).command, 'D:\\tools\\gocpp.exe');
	assert.strictEqual(serverCommand({ path: 'D:/tools/gocpp' }, opts).command, 'D:\\tools\\gocpp.exe');
	assert.strictEqual(serverCommand({ path: '~\\bin\\gocpp' }, opts).command, 'C:\\Users\\me\\bin\\gocpp.exe');
	// gocpp.lsp.env's "PATH" replaces "Path", it does not add a second one.
	const cmd = serverCommand({ env: { PATH: 'D:\\tools' } }, opts);
	assert.strictEqual(cmd.command, 'D:\\tools\\gocpp.exe');
	assert.deepStrictEqual(Object.keys(cmd.env).sort(), ['PATH', 'PATHEXT']);
	// Without PATHEXT, .exe is still tried; .com is tried after it.
	const opts2 = fake('win32', { PATH: 'C:\\gocpp\\bin' }, ['C:\\gocpp\\bin\\gocpp.exe']);
	assert.strictEqual(serverCommand({}, opts2).command, 'C:\\gocpp\\bin\\gocpp.exe');
	const opts3 = fake('win32', { PATH: 'C:\\gocpp\\bin' }, ['C:\\gocpp\\bin\\gocpp.com']);
	assert.strictEqual(serverCommand({}, opts3).command, 'C:\\gocpp\\bin\\gocpp.com');
	// A .bat or .cmd wrapper is not found, even when PATHEXT has it: Node.js
	// does not start one without a shell (CVE-2024-27980).
	const opts4 = fake('win32', env, ['C:\\gocpp\\bin\\gocpp.cmd', 'C:\\gocpp\\bin\\gocpp.bat']);
	assert.throws(() => serverCommand({}, opts4), SettingsError);
});

test('Windows: a path with spaces, quotes, a folder, a UNC name, mixed slashes', () => {
	const files = ['C:\\Program Files\\gocpp\\bin\\gocpp.exe', '\\\\srv\\share\\gocpp\\gocpp.exe', 'C:\\Users\\Jos\u00e9 \u65e5\u672c\\gocpp.exe'];
	const opts = fake('win32', { Path: '"C:\\Program Files\\gocpp\\bin";C:\\Windows' }, files);
	// Spaces need no quoting for a program that is started without a shell; a quoted path or PATH entry is read.
	assert.strictEqual(serverCommand({ path: 'C:\\Program Files\\gocpp\\bin\\gocpp.exe' }, opts).command, files[0]);
	assert.strictEqual(serverCommand({ path: '"C:\\Program Files\\gocpp\\bin\\gocpp.exe"' }, opts).command, files[0]);
	assert.strictEqual(serverCommand({ path: 'C:/Program Files/gocpp/bin/gocpp' }, opts).command, files[0]);
	assert.strictEqual(serverCommand({}, opts).command, files[0], 'a quoted entry of PATH');
	// The folder of the binary.
	assert.strictEqual(serverCommand({ path: 'C:\\Program Files\\gocpp\\bin' }, opts).command, files[0]);
	assert.strictEqual(serverCommand({ path: 'C:\\Program Files\\gocpp\\bin\\' }, opts).command, files[0]);
	// A UNC name, and a user folder with a non-ASCII name.
	assert.strictEqual(serverCommand({ path: '\\\\srv\\share\\gocpp\\gocpp' }, opts).command, files[1]);
	assert.strictEqual(serverCommand({ path: '~\\gocpp' }, fake('win32', {}, files, { homedir: 'C:\\Users\\Jos\u00e9 \u65e5\u672c' })).command, files[2]);
	// A folder without the binary is the same error as a missing file.
	assert.throws(() => serverCommand({ path: 'C:\\Windows' }, opts), SettingsError);
});

test('not found: the message names PATH or the setting gocpp.path', () => {
	const opts = fake('linux', { PATH: '/bin' }, []);
	assert.throws(() => serverCommand({}, opts), (err) => {
		assert.ok(err instanceof SettingsError);
		assert.strictEqual(err.setting, 'gocpp.path');
		assert.match(err.message, /"gocpp" is not in PATH/);
		assert.match(err.message, /set "gocpp\.path" to its full path/);
		return true;
	});
	assert.throws(() => serverCommand({ path: '/opt/gocpp/bin/gocpp' }, opts), (err) => {
		assert.strictEqual(err.setting, 'gocpp.path');
		assert.match(err.message, /the setting "gocpp\.path" is "\/opt\/gocpp\/bin\/gocpp", and there is no gocpp binary there/);
		return true;
	});
	// A relative path without a workspace folder is not found either.
	assert.throws(() => serverCommand({ path: './gocpp' }, fake('linux', {}, ['./gocpp'])), SettingsError);
});

test('settings of the wrong type are errors that name the setting', () => {
	const opts = fake('linux', { PATH: '/bin' }, ['/bin/gocpp']);
	const cases = [
		[{ path: 3 }, 'gocpp.path'],
		[{ args: '-logfile x' }, 'gocpp.lsp.args'],
		[{ args: ['-logfile', 1] }, 'gocpp.lsp.args'],
		[{ env: ['A=1'] }, 'gocpp.lsp.env'],
		[{ env: 'A=1' }, 'gocpp.lsp.env'],
		[{ env: { A: { b: 1 } } }, 'gocpp.lsp.env'],
		[{ env: { A: null } }, 'gocpp.lsp.env'],
	];
	for (const [settings, setting] of cases) {
		assert.throws(() => serverCommand(settings, opts), (err) => {
			assert.ok(err instanceof SettingsError, `${JSON.stringify(settings)}: ${err}`);
			assert.strictEqual(err.setting, setting);
			assert.ok(err.message.includes(`"${setting}"`), err.message);
			return true;
		});
	}
});

test('the real file system: an executable file is found, a plain file and a folder are not', { skip: process.platform === 'win32' }, (t) => {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gocpp-vscode-test-'));
	t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
	const bin = path.join(dir, 'gocpp');
	fs.writeFileSync(bin, '#!/bin/sh\n', { mode: 0o755 });
	fs.writeFileSync(path.join(dir, 'plain'), '', { mode: 0o644 });
	fs.mkdirSync(path.join(dir, 'folder'));
	const opts = { env: { PATH: dir } };
	assert.strictEqual(serverCommand({}, opts).command, bin);
	assert.throws(() => serverCommand({ path: 'plain' }, opts), SettingsError);
	assert.throws(() => serverCommand({ path: path.join(dir, 'folder') }, opts), SettingsError);
});
