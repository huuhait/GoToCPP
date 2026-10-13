// phase: 09

// Grep-lite prints the lines that match a regular expression, like a small
// grep. With no file names, or with the name "-", it reads standard input.
//
//	grep-lite [-i] [-v] [-n] [-c] [-F] pattern [file ...]
//
// The exit status is 0 when a line matched, 1 when no line matched and 2 when
// there was an error.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type options struct {
	ignoreCase  bool // -i
	invert      bool // -v
	lineNumbers bool // -n
	count       bool // -c
	fixed       bool // -F: the pattern is a plain string
}

// A matcher reports whether a line matches the pattern.
type matcher func(line string) bool

func newMatcher(pattern string, opt options) (matcher, error) {
	if opt.fixed {
		if opt.ignoreCase {
			lower := strings.ToLower(pattern)
			return func(line string) bool { return strings.Contains(strings.ToLower(line), lower) }, nil
		}
		return func(line string) bool { return strings.Contains(line, pattern) }, nil
	}
	if opt.ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}

// grep writes the lines of r that match to w, each after prefix, and returns
// how many lines matched.
func grep(r io.Reader, prefix string, match matcher, opt options, w io.Writer) (int, error) {
	sc := bufio.NewScanner(r)
	n, matched := 0, 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if match(line) == opt.invert {
			continue
		}
		matched++
		switch {
		case opt.count:
		case opt.lineNumbers:
			fmt.Fprintf(w, "%s%d:%s\n", prefix, n, line)
		default:
			fmt.Fprintf(w, "%s%s\n", prefix, line)
		}
	}
	if opt.count {
		fmt.Fprintf(w, "%s%d\n", prefix, matched)
	}
	return matched, sc.Err()
}

func grepFile(name, prefix string, match matcher, opt options, stdin io.Reader, w io.Writer) (int, error) {
	if name == "-" {
		return grep(stdin, prefix, match, opt, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return grep(f, prefix, match, opt, w)
}

// run is the whole program; tests call it with their own arguments and
// streams. It returns the exit status.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("grep-lite", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opt options
	fs.BoolVar(&opt.ignoreCase, "i", false, "ignore upper and lower case")
	fs.BoolVar(&opt.invert, "v", false, "print the lines that do not match")
	fs.BoolVar(&opt.lineNumbers, "n", false, "print the line number before each line")
	fs.BoolVar(&opt.count, "c", false, "print only the number of matching lines")
	fs.BoolVar(&opt.fixed, "F", false, "the pattern is a plain string, not a regular expression")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: grep-lite [-i] [-v] [-n] [-c] [-F] pattern [file ...]")
		return 2
	}
	match, err := newMatcher(fs.Arg(0), opt)
	if err != nil {
		fmt.Fprintln(stderr, "grep-lite:", err)
		return 2
	}
	names := fs.Args()[1:]
	if len(names) == 0 {
		names = []string{"-"}
	}
	found, failed := false, false
	for _, name := range names {
		prefix := ""
		if len(names) > 1 {
			prefix = name + ":"
		}
		n, err := grepFile(name, prefix, match, opt, stdin, stdout)
		if err != nil {
			fmt.Fprintln(stderr, "grep-lite:", err)
			failed = true
		}
		found = found || n > 0
	}
	switch {
	case failed:
		return 2
	case found:
		return 0
	default:
		return 1
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
