package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/huuhait/gocpp/internal/lsp/server"
)

// lspUsage is the help text of "gocpp lsp". "gocpp lsp -h" prints it,
// followed by the flags.
const lspUsage = `usage: gocpp lsp [-logfile file]

gocpp lsp runs the gocpp language server. It speaks the Language Server
Protocol (LSP 3.17) over standard input and standard output: an editor
starts it and talks to it. Its log goes to standard error, or with
-logfile to the end of that file.

It exits with status 0 after the editor's shutdown request and exit
notification, and with status 1 when the editor exits it without
shutdown or closes the connection first.

For now the server keeps the text of the open files up to date and does
nothing else yet: diagnostics, hover, completion and the other features
come in the later steps of Phase 11.

The flags are:
`

// runLSP is "gocpp lsp" (phase 11 README, step 11.1).
func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	logfile := flags.String("logfile", "", "append the log to `file`, not to standard error")
	flags.Usage = func() {
		fmt.Fprint(stderr, lspUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2 // flag has printed the error and the usage
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: gocpp lsp [-logfile file]")
		return 2
	}
	logOut := stderr
	if *logfile != "" {
		f, err := os.OpenFile(*logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp lsp: %v\n", err)
			return 2
		}
		defer f.Close()
		logOut = f
	}
	logger := log.New(logOut, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return server.Run(context.Background(), stdin, stdout, logger)
}
