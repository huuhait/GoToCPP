package build

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

// compileLimit is the length (bytes, the program and all arguments with their separators) from which
// any command of the compiler goes through a response file (@file): Windows takes at most 32,767
// characters for a command line (CreateProcess), and a compile with a long list of include folders, or a
// link of hundreds of units, can pass that. The link has its own, earlier rule (responseArgs: it keeps
// the driver flags on the line); this is the safety net for every other command. A variable, for the
// tests.
var compileLimit = windowsCompileLimit

// windowsCompileLimit is the limit on Windows: CreateProcess takes 32,767 characters, and the program's own
// quoting and a path that grows after the check (a response file's name) need some room.
const windowsCompileLimit = 30000

func init() {
	if runtime.GOOS != "windows" {
		compileLimit = 120000 // far below the limits of Linux and macOS
	}
}

var rspCounter atomic.Int64

// escapedLen is the length of the argument s in a CreateProcess command line, written by the rules of
// os/exec on Windows (syscall.EscapeArg): an argument with a space or tab, or an empty one, is in double
// quotes; a double quote has a backslash before it, and so do the backslashes in front of a quote (and in front of
// the closing quote of a quoted argument).
func escapedLen(s string) int {
	if s == "" {
		return 2
	}
	needsBackslash, hasSpace := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			needsBackslash = true
		case ' ', '\t':
			hasSpace = true
		}
	}
	switch {
	case !needsBackslash && !hasSpace:
		return len(s)
	case !needsBackslash:
		return len(s) + 2
	}
	n, slashes := len(s), 0
	if hasSpace {
		n += 2
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			slashes++
		case '"':
			n += slashes + 1 // the backslashes in front of a quote are doubled, and the quote gets one
			slashes = 0
		default:
			slashes = 0
		}
	}
	if hasSpace {
		n += slashes // in front of the closing quote
	}
	return n
}

// commandLength is the length of the command line that runs c with args as CreateProcess sees it: the
// program and every argument as quoted for it, with the spaces between them. (On other systems the
// same count is a good measure of the size of the argument list.)
func commandLength(c CXX, args []string) int {
	n := escapedLen(c.Path)
	for _, a := range c.Args {
		n += 1 + escapedLen(a)
	}
	for _, a := range args {
		n += 1 + escapedLen(a)
	}
	return n
}

// fitCommand returns the arguments to run c with: args as they are, or, when the command line would be
// longer than compileLimit, all of them in a response file in the work folder (named cmd-N.rsp,
// removed by the returned function) and "@file". The program's own Args (CXX="ccache g++") stay on
// the line: they name the program. A response file is the compilers' own feature (gcc and clang read
// @file, also under a Nix wrapper). A command that fits comes back unchanged with a function that
// does nothing.
func (b *Builder) fitCommand(c CXX, args []string) ([]string, func(), error) {
	noop := func() {}
	if commandLength(CXX{Path: c.Path, Args: c.Args}, c.compilerArgs(args)) <= compileLimit || b.Work == "" || len(args) == 0 {
		return args, noop, nil
	}
	var text strings.Builder
	for _, a := range c.compilerArgs(args) {
		text.WriteString(rspQuote(a, b.GOOS == "windows"))
		text.WriteByte('\n')
	}
	name := filepath.Join(b.Work, "cmd-"+strconv.FormatInt(rspCounter.Add(1), 10)+".rsp")
	if err := os.WriteFile(name, []byte(text.String()), 0o644); err != nil {
		return nil, noop, err
	}
	return []string{"@" + name}, func() { os.Remove(name) }, nil
}
