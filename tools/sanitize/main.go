// Sanitize runs gocpp programs under the sanitizers and fails on any report: the nightly job of
// step 15.11.
//
// Usage (in the dev shell, from the repository root):
//
//	go run ./tools/sanitize run -mode asan|ubsan|tsan [flags] [program.gp...]
//	go run ./tools/sanitize leaks [-check] [flags]       # the leak audit (tools/sanitize/leaks.txt)
//	go run ./tools/sanitize scan [file...]               # classify the sanitizer reports in files or stdin
//	go run ./tools/sanitize nightly [flags]              # every sanitizer job, canaries and the leak audit
//
// docs/guides/sanitizers.md describes the modes, the corpora, the leak policy and the nightly job.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usageText = `usage: sanitize <command> [flags]

commands:
  run      build and run programs with a sanitizer (-mode asan, ubsan or tsan); any report fails
  leaks    the leak audit: every "// lsan: cycle" program is listed in leaks.txt and leaks no more than its ceiling
  scan     print the sanitizer reports found in files (or stdin); exit 1 when there are any
  nightly  the nightly job: canaries, the sanitizer corpora with both compilers, the leak audit
`

// An exitError ends the tool with a code.
type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return e.msg }

func usageError(format string, args ...any) error {
	return exitError{2, fmt.Sprintf(format, args...)}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("sanitize "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "run":
		err = cmdRun(rest, stdout, stderr)
	case "leaks":
		err = cmdLeaks(rest, stdout, stderr)
	case "scan":
		err = cmdScan(rest, stdout, stderr)
	case "nightly":
		err = cmdNightly(rest, stdout, stderr)
	case "-h", "-help", "help":
		fmt.Fprint(stdout, usageText)
		return 0
	default:
		fmt.Fprintf(stderr, "sanitize: unknown command %q\n%s", cmd, usageText)
		return 2
	}
	if err != nil {
		if e, ok := err.(exitError); ok {
			if e.msg != "" {
				fmt.Fprintln(stderr, "sanitize:", e.msg)
			}
			return e.code
		}
		fmt.Fprintln(stderr, "sanitize:", err)
		return 1
	}
	return 0
}

// cmdScan prints the reports in each file (stdin when there is none).
func cmdScan(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("scan", stderr)
	if err := fs.Parse(args); err != nil {
		return usageError("%v", err)
	}
	found := 0
	scan := func(name string, b []byte) {
		for _, r := range Scan(string(b)) {
			fmt.Fprintf(stdout, "%s: %s\n", name, r)
			found++
		}
	}
	if fs.NArg() == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		scan("stdin", b)
	}
	for _, name := range fs.Args() {
		b, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		scan(name, b)
	}
	if found > 0 {
		return exitError{1, fmt.Sprintf("%d sanitizer reports", found)}
	}
	return nil
}
