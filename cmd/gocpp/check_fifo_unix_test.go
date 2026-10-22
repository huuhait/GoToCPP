//go:build unix

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// runGuarded runs gocpp with args in the current folder like gocppRun, but fails the test after a
// minute instead of hanging when the command waits for a named pipe. The cleanup opens the pipes
// once, so that a stuck read ends and no goroutine outlives the test.
func runGuarded(t *testing.T, pipes []string, args ...string) result {
	t.Helper()
	t.Cleanup(func() {
		for _, pipe := range pipes {
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		}
	})
	done := make(chan result, 1)
	go func() { done <- gocppRun("", false, args...) }()
	select {
	case got := <-done:
		return got
	case <-time.After(time.Minute):
		t.Fatalf("gocpp %s waits for a writer of a named pipe (no result after a minute)", strings.Join(args, " "))
		return result{}
	}
}

// namedPipeProject writes a small module in the current folder and makes the file name in it a
// named pipe, or a link to one (kind "link"), or a unix socket (kind "socket"). It returns the
// pipes to open at the end of the test.
func namedPipeProject(t *testing.T, name, kind string) (dir string, pipes []string) {
	t.Helper()
	// gocpp build reads the sources through the recorder of the up-to-date check (build.FSRecorder),
	// which needs the check on and the object cache (the folder of the cache is the one of TestMain).
	t.Setenv("GOCPP_NOUPTODATE", "")
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	dir = inDir(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nfunc main() {}\n",
	})
	path := filepath.Join(dir, name)
	os.Remove(path)
	switch kind {
	case "pipe":
		if err := syscall.Mkfifo(path, 0o644); err != nil {
			t.Skip(err)
		}
		return dir, []string{path}
	case "link":
		target := filepath.Join(dir, "target")
		if err := syscall.Mkfifo(target, 0o644); err != nil {
			t.Skip(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skip(err)
		}
		return dir, []string{target}
	case "socket":
		// (the path of a socket is limited to about 104 bytes: bind it in a short folder and move it)
		short, err := os.MkdirTemp("", "gs")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(short) })
		lis, err := net.Listen("unix", filepath.Join(short, "s"))
		if err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { lis.Close() })
		if err := os.Symlink(filepath.Join(short, "s"), path); err != nil {
			t.Skip(err)
		}
		return dir, nil
	}
	t.Fatalf("unknown kind %q", kind)
	return "", nil
}

// TestNamedPipeSource: a named pipe with the name of a source or of the module file, or a link to
// one, is an error that says what it is, in every command that loads the program; the command does
// not wait for a writer (gocpp check . hung on `mkfifo pipe.gp`).
func TestNamedPipeSource(t *testing.T) {
	for _, name := range []string{"pipe.gp", "a.c", "a.cpp", "gocpp.mod"} {
		for _, args := range [][]string{{"check", "."}, {"build", "."}, {"run", "."}, {"test", "."}, {"emit", "."}} {
			t.Run(name+"/"+args[0], func(t *testing.T) {
				_, pipes := namedPipeProject(t, name, "pipe")
				got := runGuarded(t, pipes, args...)
				if got.code == 0 || !strings.Contains(got.stderr, "not a regular file: it is a named pipe") {
					t.Errorf("gocpp %v: exit %d, stderr %q; want an error about the named pipe", args, got.code, got.stderr)
				}
			})
		}
	}
}

// TestCheckNamedPipeLinkSource: the same for a link to a named pipe.
func TestCheckNamedPipeLinkSource(t *testing.T) {
	for _, name := range []string{"link.gp", "link.c"} {
		t.Run(name, func(t *testing.T) {
			_, pipes := namedPipeProject(t, name, "link")
			got := runGuarded(t, pipes, "check", ".")
			if got.code == 0 || !strings.Contains(got.stderr, "not a regular file: it is a named pipe") {
				t.Errorf("gocpp check .: exit %d, stderr %q; want an error about the named pipe", got.code, got.stderr)
			}
		})
	}
}

// TestSocketSource: a unix socket with the name of a source is "a socket", not the system's
// "no such device or address" of the open.
func TestSocketSource(t *testing.T) {
	_, pipes := namedPipeProject(t, "sock.gp", "socket")
	got := runGuarded(t, pipes, "check", ".")
	if got.code == 0 || !strings.Contains(got.stderr, "not a regular file: it is a socket") {
		t.Errorf("gocpp check .: exit %d, stderr %q; want an error that it is a socket", got.code, got.stderr)
	}
}

// TestFmtAndDocNamedPipe: gocpp fmt of a folder and gocpp doc (the folder, and the main module by
// its path) report a named pipe with the name of a source.
func TestFmtAndDocNamedPipe(t *testing.T) {
	for _, args := range [][]string{{"fmt", "."}, {"doc", "."}, {"doc", "example.com/m"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, pipes := namedPipeProject(t, "pipe.gp", "pipe")
			got := runGuarded(t, pipes, args...)
			if got.code == 0 || !strings.Contains(got.stderr, "not a regular file: it is a named pipe") {
				t.Errorf("gocpp %v: exit %d, stderr %q; want an error about the named pipe", args, got.code, got.stderr)
			}
		})
	}
}

// TestHeaderNamedPipe: a local header that is a named pipe is an error, not a wait of the C
// compiler for a writer (the probe and the parse-as-C check both read it), whichever import it is.
func TestHeaderNamedPipe(t *testing.T) {
	headers := []string{"a.h", "b.h", "c.h"}
	for position := range headers {
		for _, args := range [][]string{{"check", "."}, {"build", "."}} {
			t.Run(fmt.Sprintf("%s/import %d", args[0], position+1), func(t *testing.T) {
				pipe := headers[position]
				dir, pipes := namedPipeProject(t, pipe, "pipe")
				var imports strings.Builder
				for _, header := range headers {
					fmt.Fprintf(&imports, "import \"./%s\"\n", header)
					if header != pipe {
						text := "int " + strings.TrimSuffix(header, ".h") + "_f(void);\n"
						if err := os.WriteFile(filepath.Join(dir, header), []byte(text), 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
				main := "package main\n\n" + imports.String() + "\nfunc main() {}\n"
				if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(main), 0o644); err != nil {
					t.Fatal(err)
				}
				got := runGuarded(t, pipes, args...)
				if got.code == 0 || !strings.Contains(got.stderr, pipe+": not a regular file: it is a named pipe") {
					t.Errorf("gocpp %v: exit %d, stderr %q; want an error about the named pipe %s", args, got.code, got.stderr, pipe)
				}
			})
		}
	}
}

// TestHeaderNamedPipeOneError: the named pipe is reported once at its import, not by the probe and
// by the parse-as-C check both.
func TestHeaderNamedPipeOneError(t *testing.T) {
	for _, command := range []string{"check", "build"} {
		t.Run(command, func(t *testing.T) {
			dir, pipes := namedPipeProject(t, "pipe.h", "pipe")
			main := "package main\n\nimport \"./pipe.h\"\n\nfunc main() {}\n"
			if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(main), 0o644); err != nil {
				t.Fatal(err)
			}
			got := runGuarded(t, pipes, command, ".")
			if got.code == 0 || strings.Count(got.stderr, "./main.gp:3:8:") != 1 {
				t.Errorf("gocpp %s .: exit %d, stderr %q; want one error at the import", command, got.code, got.stderr)
			}
			if strings.Contains(got.stderr, "cannot check whether header") {
				t.Errorf("gocpp %s .: stderr %q; want no parse-as-C error for a header that is no file", command, got.stderr)
			}
		})
	}
}

// TestDocNamedPipeInADependency: the root of a required module is read like the main module.
func TestDocNamedPipeInADependency(t *testing.T) {
	dir, _ := namedPipeProject(t, "main.gp", "pipe")
	os.Remove(filepath.Join(dir, "main.gp"))
	files := map[string]string{
		"gocpp.mod":     "module example.com/m\n\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ./dep\n",
		"main.gp":       "package main\n\nfunc main() {}\n",
		"dep/gocpp.mod": "module example.com/dep\n",
		"dep/ok.gp":     "package dep\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pipe := filepath.Join(dir, "dep", "pipe.gp")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	got := runGuarded(t, []string{pipe}, "doc", "example.com/dep")
	if got.code == 0 || !strings.Contains(got.stderr, "not a regular file: it is a named pipe") {
		t.Errorf("gocpp doc example.com/dep: exit %d, stderr %q; want an error about the named pipe", got.code, got.stderr)
	}
}

// pipeWith makes a named pipe in a folder of the test and a writer that gives it text (when
// something opens the pipe to read; the cleanup lets a writer that nobody read go), and returns
// the pipe.
func pipeWith(t *testing.T, name, text string) string {
	t.Helper()
	pipe := filepath.Join(t.TempDir(), name)
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { // lets a stuck open go on: no goroutine outlives the test
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	go func() {
		f, err := os.OpenFile(pipe, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		f.WriteString(text)
		f.Close()
	}()
	return pipe
}

// TestFmtExplicitPipe: a named pipe named on the command line is read as gofmt reads it (it
// works for gocpp fmt /dev/stdin and gocpp fmt <(...)): it waits for a writer, formats what it
// gets, and -l and -d work. -w refuses it. A pipe without a writer blocks, like gofmt (not tested).
func TestFmtExplicitPipe(t *testing.T) {
	const messy = "package main\nfunc main(){\n}\n"
	const neat = "package main\n\nfunc main() {\n}\n"
	inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	for _, c := range []struct {
		name  string
		flags []string
		want  func(r result, pipe string) bool
	}{
		{"print", nil, func(r result, _ string) bool { return r.code == 0 && r.stdout == neat }},
		{"list", []string{"-l"}, func(r result, pipe string) bool { return r.code == 0 && strings.TrimSpace(r.stdout) == pipe }},
		{"diff", []string{"-d"}, func(r result, _ string) bool { return r.code == 0 && strings.Contains(r.stdout, "+func main() {") }},
		{"write", []string{"-w"}, func(r result, _ string) bool {
			return r.code != 0 && strings.Contains(r.stderr, "cannot use -w") && r.stdout == ""
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			pipe := pipeWith(t, "in.gp", messy)
			args := append(append([]string{"fmt"}, c.flags...), pipe)
			var r result
			done := make(chan struct{})
			go func() {
				r = gocppRun("", false, args...)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Minute):
				t.Fatalf("gocpp %v waits although the pipe has a writer", args)
			}
			if !c.want(r, pipe) {
				t.Errorf("gocpp %v = %+v", args, r)
			}
		})
	}
}
