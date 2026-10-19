// Command winlspctl is the file-system helper of the language-server test on a real Windows PC (Phase 14 step 14.5,
// tests/windows_lsp_test.go). The test cannot make a file with a non-ASCII name through cmd.exe and ssh (the command
// line goes through the console code page), so it copies a JSON script and this program to the PC and runs it there:
//
//	winlspctl script.json
//
// The script is a list of operations; names and texts are JSON strings, so a character is written \uXXXX and travels
// as ASCII. The operations:
//
//	{"op":"mkdir","path":P}               make the folder and its parents
//	{"op":"write","path":P,"text":T}      write the file (T exactly: write \r\n yourself), making the folder
//	{"op":"writebom","path":P,"text":T}   the same with a UTF-8 byte order mark first
//	{"op":"remove","path":P}              remove a file or an empty folder
//	{"op":"removeall","path":P}           remove a tree
//	{"op":"rename","path":P,"to":Q}       rename (also a change of case only)
//	{"op":"lockfor","path":P,"ms":N}      lock the file's bytes (LockFileEx) for N ms: reads fail with ERROR_LOCK_VIOLATION
//	{"op":"realcase","path":P}            print the path with the case the disk has (JSON string)
//	{"op":"list","path":P}                print the names in the folder, one JSON string per line
//
// It prints one line per operation that prints; an error is printed to standard error and ends the program with status 1.
// It uses Go's file functions (the wide Windows API), so it works for every name Windows allows.
//
//	winlspctl run env.json prog args...
//
// runs prog with its standard streams connected to this program's and the environment of this program plus the variables
// of env.json (a JSON object of names and values, which may hold any character: cmd.exe could not set them), and
// exits with its status. The language-server test starts gocpp.exe lsp so, with a TEMP and a LOCALAPPDATA that the C++
// compiler cannot read (step 14.2's environment rules).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type op struct {
	Op   string `json:"op"`
	Path string `json:"path"`
	To   string `json:"to"`
	Text string `json:"text"`
	Ms   int    `json:"ms"`
}

func main() {
	if len(os.Args) >= 4 && os.Args[1] == "run" {
		os.Exit(runProgram(os.Args[2], os.Args[3], os.Args[4:]))
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: winlspctl script.json | winlspctl run env.json prog args...")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err)
	}
	var ops []op
	if err := json.Unmarshal(data, &ops); err != nil {
		fail(err)
	}
	for i, o := range ops {
		if err := run(o); err != nil {
			fmt.Fprintf(os.Stderr, "operation %d (%s %s): %v\n", i, o.Op, o.Path, err)
			os.Exit(1)
		}
	}
}

// runProgram runs prog with the extra environment of the JSON file envFile and returns its exit status.
func runProgram(envFile, prog string, args []string) int {
	data, err := os.ReadFile(envFile)
	if err != nil {
		fail(err)
	}
	var extra map[string]string
	if err := json.Unmarshal(data, &extra); err != nil {
		fail(err)
	}
	cmd := exec.Command(prog, args...)
	cmd.Env = os.Environ()
	for k, v := range extra {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fail(err)
	}
	return 0
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func run(o op) error {
	switch o.Op {
	case "mkdir":
		return os.MkdirAll(o.Path, 0o755)
	case "write", "writebom":
		if err := os.MkdirAll(filepath.Dir(o.Path), 0o755); err != nil {
			return err
		}
		text := o.Text
		if o.Op == "writebom" {
			text = "\xef\xbb\xbf" + text
		}
		return os.WriteFile(o.Path, []byte(text), 0o644)
	case "remove":
		return os.Remove(o.Path)
	case "lockfor":
		return lockFor(o.Path, o.Ms)
	case "removeall":
		return os.RemoveAll(o.Path)
	case "rename":
		return os.Rename(o.Path, o.To)
	case "realcase":
		p, err := filepath.EvalSymlinks(o.Path)
		if err != nil {
			return err
		}
		return printJSON(p)
	case "list":
		entries, err := os.ReadDir(o.Path)
		if err != nil {
			return err
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			if err := printJSON(n); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown operation %q", o.Op)
}

func printJSON(s string) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	fmt.Println(strings.ReplaceAll(string(b), " ", ` `))
	return nil
}
