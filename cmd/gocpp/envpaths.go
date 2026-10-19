package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// The folders gocpp makes files in must be paths the compilers can read (Phase 14 step 14.2, ADR-081): the
// cache (%LocalAppData%\gocpp), where the generated C++ and the objects are, and the temporary folder (%TEMP%),
// where the work folder of a build is. MinGW-w64's g++ reads its command line in the ANSI code page; a user
// called 日本 or Łukasz has a profile folder it cannot name, and then no compile finds its input. And CreateProcess takes a
// working folder of at most 247 characters, so a TEMP much longer than that cannot hold a work folder either. So at the start
// of the process each of LOCALAPPDATA, TEMP and TMP whose value has a character outside the code page (TEMP and TMP: or is
// longer than 200 characters) is replaced, in gocpp's own environment and so in that of the compilers it starts, by an
// equivalent folder that the compilers can name: the 8.3 short name of the same folder when the volume has one (a name that
// is a legal 8.3 name already, such as tmp_Ł, has none), else a PRIVATE folder below
// %ProgramData%\gocpp-paths-<SID>\<hash of the original>\ (winpath.PrivateFolder: a protected ACL for the user, SYSTEM and
// Administrators only, the owner checked when it exists already; %ProgramData% is shared, and a folder made there would be
// readable by every user and could be made first by an attacker). A value the compilers can read is left alone, spaces
// included (a space is no trouble for an argument; the one place where gocpp passed a path inside text that is
// split at spaces, GCC's -B for the manifest, takes a folder without spaces: internal/build/manifest.go).
//
// The user's own programs do not see the change: the original values are kept, and a program that gocpp run or gocpp test
// starts gets them back (programEnv), so os.Getenv("TEMP") is the same whether the program was started by gocpp or by hand.

// maxTempLen is the longest TEMP that is kept: the work folder is below it, and a compiler is started in the work folder.
const maxTempLen = 200

// pathsEnv is what normalizeEnvPaths uses of the system; the tests replace it.
type pathsEnv struct {
	getenv  func(string) string
	setenv  func(k, v string)
	mkdir   func(string) error
	safe    func(string) bool
	short   func(string) (string, bool)
	private func(base string, parts ...string) (string, error) // a private folder below base (winpath.PrivateFolder)
	program string                                             // the value of ProgramData
}

// originalEnv holds the values that normalizeEnvPaths replaced, by variable name.
var originalEnv = map[string]string{}

// A pathProblem is a variable that the compilers cannot use and that no replacement was found for.
type pathProblem struct {
	name, value, reason string
	unreadable          bool // the path has a character outside the code page: the compilers certainly cannot open it (else it is only long)
}

// pathProblems are the problems of normalizeEnvPaths, for checkPathProblems.
var pathProblems []pathProblem

// normalizeEnvPaths makes LOCALAPPDATA, TEMP and TMP usable for the compilers (see above), and returns the variables it changed. A variable
// that needs a replacement and gets none (no short name, no private folder: it was refused, or ProgramData is not usable) is left as it is
// and recorded in pathProblems.
func normalizeEnvPaths(e pathsEnv) (changed []string) {
	type result struct {
		repl, reason string
	}
	done := map[string]result{} // original value -> result, so that TEMP and TMP stay the same folder
	for _, name := range []string{"LOCALAPPDATA", "TEMP", "TMP"} {
		v := e.getenv(name)
		if v == "" || e.safe(v) && (name == "LOCALAPPDATA" || len(v) <= maxTempLen) {
			continue
		}
		r, ok := done[v]
		if !ok {
			r.repl, r.reason = replacementFolder(e, name, v)
			done[v] = r
		}
		if r.repl != "" {
			originalEnv[name] = v
			e.setenv(name, r.repl)
			changed = append(changed, name)
			continue
		}
		pathProblems = append(pathProblems, pathProblem{name: name, value: v, reason: r.reason, unreadable: !e.safe(v)})
	}
	return changed
}

// replacementFolder returns the folder that stands for v, or why there is none.
func replacementFolder(e pathsEnv, name, v string) (folder, reason string) {
	e.mkdir(v) // a folder that is not there yet has no short name
	if s, ok := e.short(v); ok && e.safe(s) && (name == "LOCALAPPDATA" || len(s) <= maxTempLen) {
		return s, ""
	}
	reason = "it has no 8.3 short name that would do"
	switch {
	case e.program == "" || e.private == nil:
		return "", reason + ", and ProgramData is not set"
	case !e.safe(e.program):
		return "", reason + ", and ProgramData (" + e.program + ") is not a path the compilers can use either"
	}
	sum := sha256.Sum256([]byte(v))
	dir, err := e.private(e.program, hex.EncodeToString(sum[:5]), name)
	if err != nil {
		return "", reason + ", and the private folder below ProgramData cannot be used: " + err.Error()
	}
	if !e.safe(dir) {
		return "", reason + ", and the private folder " + dir + " is not a path the compilers can use"
	}
	return dir, ""
}

// checkPathProblems reports the problems of normalizeEnvPaths to the user of a command that starts a C++ compiler. A path with a
// character outside the code page that has no replacement makes every compile fail with a message about a file the compiler
// cannot open and "this is a bug in gocpp"; so the command stops here, with the variable, the path and the reason. A path that is only
// too long is a warning (the build may work).
func checkPathProblems(command string, stderr io.Writer) int {
	code := 0
	for _, p := range pathProblems {
		if p.unreadable {
			fmt.Fprintf(stderr, "gocpp %s: %s is %s, which the C++ compiler cannot read (it has a character outside the system's ANSI code page), and no replacement could be made: %s.\n"+
				"Set %s to a folder whose path is plain ASCII (or fix the reason above) and run gocpp again.\n", command, p.name, p.value, p.reason, p.name)
			code = exitErrors
			continue
		}
		fmt.Fprintf(stderr, "gocpp %s: warning: %s is %s, which is longer than %d characters, and no replacement could be made: %s. The build may fail to start the compiler.\n",
			command, p.name, p.value, maxTempLen, p.reason)
	}
	return code
}

// programEnv returns the environment for a program of the user that gocpp starts (gocpp run, gocpp test): gocpp's own, with the
// values that normalizeEnvPaths replaced put back. It is os.Environ() when nothing was replaced.
func programEnv() []string {
	env := os.Environ()
	if len(originalEnv) == 0 {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		replaced := false
		for k, v := range originalEnv {
			if strings.EqualFold(name, k) {
				out = append(out, name+"="+v)
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, kv)
		}
	}
	return out
}

// runProgramEnv returns the environment for the program of gocpp run in the folder dir when normalizeEnvPaths replaced something, or nil
// (inherit gocpp's own) when it did not. Where an explicit environment is given os/exec does not add PWD for the folder (a nil one gets
// it on Unix), so on Unix PWD=dir is added to keep the program's environment what it was; on Windows os/exec adds no PWD for either, so the program has
// the same variables as when it is started by hand.
func runProgramEnv(dir, goos string) []string {
	if len(originalEnv) == 0 {
		return nil
	}
	env := programEnv()
	if goos != "windows" {
		env = append(env, "PWD="+dir)
	}
	return env
}
