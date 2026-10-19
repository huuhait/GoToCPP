package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// normalizeEnvPaths on any system, with a fake Windows: a path is "safe" when it has no "X", the 8.3 names are a table, the
// private folders are made by a function that records its arguments.
type fakePaths struct {
	env     map[string]string
	shorts  map[string]string
	made    []string // folders made by mkdir
	private [][]string
	privErr error
}

func (f *fakePaths) pathsEnv() pathsEnv {
	return pathsEnv{
		getenv: func(k string) string { return f.env[k] },
		setenv: func(k, v string) { f.env[k] = v },
		mkdir:  func(p string) error { f.made = append(f.made, p); return nil },
		safe:   func(s string) bool { return !strings.Contains(s, "X") },
		short:  func(p string) (string, bool) { s, ok := f.shorts[p]; return s, ok },
		private: func(base string, parts ...string) (string, error) {
			f.private = append(f.private, append([]string{base}, parts...))
			if f.privErr != nil {
				return "", f.privErr
			}
			return filepath.Join(append([]string{base, "gocpp-paths-S-1-5-21-1"}, parts...)...), nil
		},
		program: f.env["ProgramData"],
	}
}

func withCleanOriginals(t *testing.T) {
	t.Helper()
	old, oldProblems := originalEnv, pathProblems
	originalEnv, pathProblems = map[string]string{}, nil
	t.Cleanup(func() { originalEnv, pathProblems = old, oldProblems })
}

func TestNormalizeEnvPaths(t *testing.T) {
	t.Run("safe values stay (spaces included)", func(t *testing.T) {
		withCleanOriginals(t)
		f := &fakePaths{env: map[string]string{"LOCALAPPDATA": `C:\Users\John Smith\AppData\Local`, "TEMP": `C:\Users\John Smith\AppData\Local\Temp`, "TMP": `C:\Users\John Smith\AppData\Local\Temp`}}
		if got := normalizeEnvPaths(f.pathsEnv()); len(got) != 0 || len(f.made) != 0 || len(originalEnv) != 0 {
			t.Errorf("changed %v, made %v, originals %v", got, f.made, originalEnv)
		}
	})
	t.Run("the short name", func(t *testing.T) {
		withCleanOriginals(t)
		f := &fakePaths{
			env:    map[string]string{"LOCALAPPDATA": `C:\Users\JXrgen\AppData\Local`, "TEMP": `C:\Users\JXrgen\AppData\Local\Temp`, "TMP": `C:\Users\JXrgen\AppData\Local\Temp`},
			shorts: map[string]string{`C:\Users\JXrgen\AppData\Local`: `C:\Users\JRGEN~1\AppData\Local`, `C:\Users\JXrgen\AppData\Local\Temp`: `C:\Users\JRGEN~1\AppData\Local\Temp`},
		}
		got := normalizeEnvPaths(f.pathsEnv())
		if !slices.Equal(got, []string{"LOCALAPPDATA", "TEMP", "TMP"}) {
			t.Fatalf("changed %v", got)
		}
		if f.env["LOCALAPPDATA"] != `C:\Users\JRGEN~1\AppData\Local` || f.env["TEMP"] != `C:\Users\JRGEN~1\AppData\Local\Temp` || f.env["TMP"] != f.env["TEMP"] {
			t.Errorf("env = %v", f.env)
		}
		if originalEnv["TEMP"] != `C:\Users\JXrgen\AppData\Local\Temp` || originalEnv["LOCALAPPDATA"] != `C:\Users\JXrgen\AppData\Local` {
			t.Errorf("the originals were not kept: %v", originalEnv)
		}
		if len(f.private) != 0 {
			t.Errorf("a private folder was made although the short name works: %v", f.private)
		}
	})
	t.Run("no short name: a private folder below ProgramData, the same for TEMP and TMP", func(t *testing.T) {
		withCleanOriginals(t)
		f := &fakePaths{env: map[string]string{"LOCALAPPDATA": `C:\Users\JX\AppData\Local`, "TEMP": `C:\Users\JX\AppData\Local\Temp`, "TMP": `C:\Users\JX\AppData\Local\Temp`, "ProgramData": `C:\ProgramData`}}
		normalizeEnvPaths(f.pathsEnv())
		for _, name := range []string{"LOCALAPPDATA", "TEMP"} {
			if !strings.HasPrefix(strings.ReplaceAll(f.env[name], `\`, "/"), "C:/ProgramData/gocpp-paths-S-1-5-21-1/") || strings.Contains(f.env[name], "X") {
				t.Errorf("%s = %q", name, f.env[name])
			}
		}
		if f.env["TEMP"] != f.env["TMP"] || f.env["TEMP"] == f.env["LOCALAPPDATA"] {
			t.Errorf("TEMP %q TMP %q LOCALAPPDATA %q", f.env["TEMP"], f.env["TMP"], f.env["LOCALAPPDATA"])
		}
		// Only through the private-folder function, one call for each distinct original (TEMP and TMP share), with ProgramData as the base.
		if len(f.private) != 2 || f.private[0][0] != `C:\ProgramData` || f.private[0][2] != "LOCALAPPDATA" || f.private[1][2] != "TEMP" {
			t.Errorf("private folder calls: %v", f.private)
		}
	})
	t.Run("a private folder that is refused: the variable stays", func(t *testing.T) {
		withCleanOriginals(t)
		f := &fakePaths{env: map[string]string{"LOCALAPPDATA": `C:\Users\JX\AppData\Local`, "ProgramData": `C:\ProgramData`}, privErr: errors.New("owned by someone else: refused")}
		if got := normalizeEnvPaths(f.pathsEnv()); len(got) != 0 || f.env["LOCALAPPDATA"] != `C:\Users\JX\AppData\Local` || len(originalEnv) != 0 {
			t.Errorf("changed %v, env %v", got, f.env)
		}
		// The refusal is kept, with the folder's reason, for the commands that need the variable.
		if len(pathProblems) != 1 || pathProblems[0].name != "LOCALAPPDATA" || !pathProblems[0].unreadable || !strings.Contains(pathProblems[0].reason, "owned by someone else: refused") {
			t.Fatalf("problems = %+v", pathProblems)
		}
		var out strings.Builder
		if code := checkPathProblems("build", &out); code != 1 || !strings.Contains(out.String(), `LOCALAPPDATA is C:\Users\JX\AppData\Local`) ||
			!strings.Contains(out.String(), "owned by someone else: refused") || !strings.Contains(out.String(), "Set LOCALAPPDATA to a folder") {
			t.Errorf("checkPathProblems = %d:\n%s", code, out.String())
		}
	})
	t.Run("nothing works: left as it is", func(t *testing.T) {
		withCleanOriginals(t)
		f := &fakePaths{env: map[string]string{"LOCALAPPDATA": `C:\Users\JX\AppData\Local`, "ProgramData": `C:\XData`}}
		if got := normalizeEnvPaths(f.pathsEnv()); len(got) != 0 || f.env["LOCALAPPDATA"] != `C:\Users\JX\AppData\Local` {
			t.Errorf("changed %v, env %v", got, f.env)
		}
	})
	t.Run("a TEMP that is too long for a work folder", func(t *testing.T) {
		withCleanOriginals(t)
		long := `C:\Temp\` + strings.Repeat("a", maxTempLen)
		short := `C:\TEMP\LONGPA~1`
		f := &fakePaths{env: map[string]string{"LOCALAPPDATA": long, "TEMP": long, "TMP": long}, shorts: map[string]string{long: short}}
		got := normalizeEnvPaths(f.pathsEnv())
		// LOCALAPPDATA may be long (cache paths are not a working folder); TEMP and TMP are replaced.
		if !slices.Equal(got, []string{"TEMP", "TMP"}) || f.env["TEMP"] != short || f.env["LOCALAPPDATA"] != long {
			t.Errorf("changed %v, env %v", got, f.env)
		}
		// A short name that is still too long is no help: the private folder.
		withCleanOriginals(t)
		f = &fakePaths{env: map[string]string{"TEMP": long, "ProgramData": `C:\ProgramData`}, shorts: map[string]string{long: strings.Repeat("b", maxTempLen+1)}}
		normalizeEnvPaths(f.pathsEnv())
		if len(f.private) != 1 || len(f.env["TEMP"]) > maxTempLen {
			t.Errorf("env %v, private %v", f.env, f.private)
		}
	})
}

// The program that gocpp run starts sees the user's own TEMP and LOCALAPPDATA, not the ones made for the compilers.
func TestProgramEnv(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "short-lad")
	t.Setenv("TEMP", "short-temp")
	t.Setenv("OTHER_VARIABLE", "kept")
	t.Run("nothing replaced", func(t *testing.T) {
		withCleanOriginals(t)
		env := programEnv()
		if !slices.Contains(env, "TEMP=short-temp") {
			t.Errorf("env lost TEMP: %v", env[:3])
		}
	})
	t.Run("replaced", func(t *testing.T) {
		withCleanOriginals(t)
		originalEnv["TEMP"] = `C:\Users\Jx\Temp`
		originalEnv["LOCALAPPDATA"] = `C:\Users\Jx\Local`
		env := programEnv()
		for _, want := range []string{`TEMP=C:\Users\Jx\Temp`, `LOCALAPPDATA=C:\Users\Jx\Local`, "OTHER_VARIABLE=kept"} {
			if !slices.Contains(env, want) {
				t.Errorf("env lacks %s", want)
			}
		}
		for _, bad := range []string{"TEMP=short-temp", "LOCALAPPDATA=short-lad"} {
			if slices.Contains(env, bad) {
				t.Errorf("env has %s: the program would see the compilers' folder", bad)
			}
		}
		if n := len(env); n != len(os.Environ()) {
			t.Errorf("the environment has %d entries, os.Environ() %d", n, len(os.Environ()))
		}
	})
}

func TestCheckPathProblems(t *testing.T) {
	withCleanOriginals(t)
	var out strings.Builder
	if code := checkPathProblems("build", &out); code != 0 || out.Len() != 0 {
		t.Errorf("no problem: %d %q", code, out.String())
	}
	// A path that is only too long is a warning: the build may work.
	pathProblems = []pathProblem{{name: "TEMP", value: `C:\very\long`, reason: "it has no 8.3 short name that would do", unreadable: false}}
	if code := checkPathProblems("run", &out); code != 0 || !strings.Contains(out.String(), "warning: TEMP is C:\\very\\long") || !strings.Contains(out.String(), "longer than 200 characters") {
		t.Errorf("long TEMP: %d %q", code, out.String())
	}
	// ProgramData that cannot be used says so.
	f := &fakePaths{env: map[string]string{"LOCALAPPDATA": `C:\Users\JX\AppData\Local`, "ProgramData": `C:\XData`}}
	pathProblems = nil
	normalizeEnvPaths(f.pathsEnv())
	if len(pathProblems) != 1 || !strings.Contains(pathProblems[0].reason, "ProgramData (C:\\XData) is not a path the compilers can use") {
		t.Errorf("problems = %+v", pathProblems)
	}
}

// The environment of the program of gocpp run: gocpp's own when nothing was replaced; the original values otherwise, with PWD added on
// Unix only (os/exec gives a nil environment a PWD there, and an explicit one none; on Windows it adds none in either case, so the
// program has the variables it has when started by hand).
func TestRunProgramEnv(t *testing.T) {
	t.Setenv("TEMP", "short")
	t.Setenv("PWD", "/old")
	t.Run("nothing replaced", func(t *testing.T) {
		withCleanOriginals(t)
		for _, goos := range []string{"linux", "windows"} {
			if env := runProgramEnv("/p", goos); env != nil {
				t.Errorf("%s: env = %v, want nil (inherit)", goos, env[:2])
			}
		}
	})
	t.Run("replaced", func(t *testing.T) {
		withCleanOriginals(t)
		originalEnv["TEMP"] = `C:\Users\Jx\Temp`
		count := func(env []string, prefix string) (n int) {
			for _, kv := range env {
				if strings.HasPrefix(kv, prefix) {
					n++
				}
			}
			return
		}
		lin := runProgramEnv("/p", "linux")
		win := runProgramEnv("/p", "windows")
		if !slices.Contains(lin, "PWD=/p") || count(lin, "PWD=") != 2 {
			t.Errorf("linux: PWD entries %d, want the old one and the new one", count(lin, "PWD="))
		}
		if slices.Contains(win, "PWD=/p") || len(win) != len(os.Environ()) {
			t.Errorf("windows: %d entries (os.Environ has %d), PWD added: %v", len(win), len(os.Environ()), slices.Contains(win, "PWD=/p"))
		}
		for _, env := range [][]string{lin, win} {
			if !slices.Contains(env, `TEMP=C:\Users\Jx\Temp`) || slices.Contains(env, "TEMP=short") {
				t.Errorf("TEMP not restored: %v", env)
			}
		}
	})
}
