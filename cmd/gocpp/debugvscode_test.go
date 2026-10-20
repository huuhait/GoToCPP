package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// launchDoc is the part of a launch.json that the tests read.
type launchDoc struct {
	Version        string
	Configurations []map[string]any
}

// parseLaunch decodes the output of gocpp debug -vscode and returns its one configuration.
func parseLaunch(t *testing.T, out string) map[string]any {
	t.Helper()
	var doc launchDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("the launch.json is not JSON: %v\n%s", err, out)
	}
	if doc.Version != "0.2.0" || len(doc.Configurations) != 1 {
		t.Fatalf("the launch.json has version %q and %d configurations:\n%s", doc.Version, len(doc.Configurations), out)
	}
	return doc.Configurations[0]
}

func TestVSCodeLaunchGDB(t *testing.T) {
	noTargetEnv(t)
	old := findTool
	findTool = func(string) (string, string, bool) { return "", "", false }
	t.Cleanup(func() { findTool = old })
	dir := t.TempDir()
	t.Chdir(dir)
	for _, c := range []struct {
		goos, program, gdb string
	}{
		{"linux", "${workspaceFolder}/bin/hello", "gdb"},
		{"windows", "${workspaceFolder}/bin/hello.exe", "gdb.exe"},
	} {
		t.Run(c.goos, func(t *testing.T) {
			r := gocppRun("", false, "debug", "-vscode", "-os", c.goos, "hello.gp")
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("gocpp debug -vscode = %+v", r)
			}
			cfg := parseLaunch(t, r.stdout)
			if cfg["type"] != "cppdbg" || cfg["MIMode"] != "gdb" || cfg["request"] != "launch" {
				t.Errorf("not a cppdbg gdb launch: %v", cfg)
			}
			if cfg["program"] != c.program || cfg["cwd"] != "${workspaceFolder}" || cfg["preLaunchTask"] != "gocpp: build" {
				t.Errorf("program, cwd or task wrong: %v", cfg)
			}
			if cfg["miDebuggerPath"] != c.gdb {
				t.Errorf("miDebuggerPath = %v, want %s", cfg["miDebuggerPath"], c.gdb)
			}
			// The safe path of gdb is the program, in double quotes (the workspace may have spaces).
			want := `-iex "add-auto-load-safe-path ` + c.program + `"`
			if cfg["miDebuggerArgs"] != want {
				t.Errorf("miDebuggerArgs = %v, want %s", cfg["miDebuggerArgs"], want)
			}
			if _, ok := cfg["sourceFileMap"]; ok {
				t.Error("sourceFileMap without -map")
			}
			setup := cfg["setupCommands"].([]any)
			if len(setup) != 2 || setup[1].(map[string]any)["text"] != "set gocpp print-style tree" {
				t.Errorf("setupCommands = %v", setup)
			}
		})
	}

	t.Run("the debugger found on this machine is miDebuggerPath", func(t *testing.T) {
		findTool = func(name string) (string, string, bool) {
			return `C:\msys64\ucrt64\bin\gdb.exe`, "MSYS2 ucrt64", name == "gdb"
		}
		defer func() { findTool = func(string) (string, string, bool) { return "", "", false } }()
		goos := "linux"
		if runtime.GOOS == "windows" {
			goos = "windows"
		}
		r := gocppRun("", false, "debug", "-vscode", "-os", goos, "hello.gp")
		cfg := parseLaunch(t, r.stdout)
		want := `C:\msys64\ucrt64\bin\gdb.exe`
		if runtime.GOOS == "windows" {
			want = "C:/msys64/ucrt64/bin/gdb.exe"
		}
		if cfg["miDebuggerPath"] != want {
			t.Errorf("miDebuggerPath = %v, want %s", cfg["miDebuggerPath"], want)
		}
	})

	t.Run("-map becomes sourceFileMap", func(t *testing.T) {
		r := gocppRun("", false, "debug", "-vscode", "-os", "windows", "-map", `C:\trim=${workspaceFolder}`, "hello.gp")
		cfg := parseLaunch(t, r.stdout)
		files, _ := cfg["sourceFileMap"].(map[string]any)
		if files[`C:\trim`] != "${workspaceFolder}" || len(files) != 1 {
			t.Errorf("sourceFileMap = %v", cfg["sourceFileMap"])
		}
	})

	t.Run("the program is named after the package or the folder", func(t *testing.T) {
		for args, want := range map[string]string{"./cmd/tool": "tool", "app.gp": "app", ".": filepath.Base(dir)} {
			r := gocppRun("", false, "debug", "-vscode", "-os", "linux", args)
			if got := parseLaunch(t, r.stdout)["program"]; got != "${workspaceFolder}/bin/"+want {
				t.Errorf("gocpp debug -vscode %s: program = %v, want bin/%s", args, got, want)
			}
		}
	})
}

func TestVSCodeLaunchLLDB(t *testing.T) {
	noTargetEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	r := gocppRun("", false, "debug", "-vscode", "-debugger", "lldb", "-os", "windows",
		"-map", "/trim=${workspaceFolder}", "hello.gp")
	if r.code != 0 || !strings.Contains(r.stderr, "gocpp_lldb.py") {
		t.Fatalf("gocpp debug -vscode -debugger lldb = %+v", r)
	}
	cfg := parseLaunch(t, r.stdout)
	if cfg["type"] != "lldb-dap" || cfg["program"] != "${workspaceFolder}/bin/hello.exe" {
		t.Errorf("not an lldb-dap launch of hello.exe: %v", cfg)
	}
	init := cfg["initCommands"].([]any)
	if len(init) != 1 || init[0] != `command script import "${workspaceFolder}/.vscode/gocpp_lldb.py"` {
		t.Errorf("initCommands = %v", init)
	}
	maps := cfg["sourceMap"].([]any)
	if len(maps) != 1 || maps[0].([]any)[0] != "/trim" || maps[0].([]any)[1] != "${workspaceFolder}" {
		t.Errorf("sourceMap = %v", maps)
	}
	// The summaries that the configuration loads are written to the workspace.
	script, err := os.ReadFile(filepath.Join(dir, ".vscode", "gocpp_lldb.py"))
	if err != nil || !strings.Contains(string(script), "gp_locals") {
		t.Errorf(".vscode/gocpp_lldb.py: %v, %d bytes", err, len(script))
	}
}

// The configuration of docs/guides/debugging.md for Windows is the one that -vscode prints: the
// guide shows the file, this test keeps the two together.
func TestVSCodeLaunchInGuide(t *testing.T) {
	noTargetEnv(t)
	findTool = func(string) (string, string, bool) { return "", "", false }
	t.Cleanup(func() { findTool = defaultFindTool })
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "guides", "debugging.md"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"-os", "windows", "app.gp"},
		{"-os", "linux", "app.gp"},
		{"-debugger", "lldb", "-os", "linux", "app.gp"},
	} {
		r := gocppRun("", false, append([]string{"debug", "-vscode"}, args...)...)
		if r.code != 0 {
			t.Fatalf("gocpp debug -vscode %v = %+v", args, r)
		}
		if !strings.Contains(string(guide), strings.TrimSpace(r.stdout)) {
			t.Errorf("docs/guides/debugging.md does not hold the launch.json that "+
				"gocpp debug -vscode %v prints:\n%s", args, r.stdout)
		}
	}
}

// A .vscode that cannot be written is an error, not a launch.json that loads a missing script.
func TestVSCodeLLDBScriptNotWritable(t *testing.T) {
	noTargetEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".vscode"), []byte("a file, not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gocppRun("", false, "debug", "-vscode", "-debugger", "lldb", "app.gp")
	if r.code != exitErrors || r.stdout != "" || !strings.Contains(r.stderr, "gocpp debug:") {
		t.Errorf("gocpp debug -vscode -debugger lldb with .vscode a file = %+v", r)
	}
}
