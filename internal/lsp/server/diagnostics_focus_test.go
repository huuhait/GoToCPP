package server

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/version"
)

// The tests of the quick run of the diagnostics (diagnostics.go, run): a change inside the body of
// a function is answered from the focused check of the package, and the full run settles it later.

// quickFiles is a package main of two files: a.gp has the function that the tests edit, b.gp uses
// it.
func quickFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package main\n\nfunc f() int {\n\treturn 1\n}\n",
		"b.gp":      "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t_ = f()\n\tfmt.Println(\"x\")\n}\n",
	}
}

// startQuick starts a server over quickFiles with the settle delay, and opens both files.
func startQuick(t *testing.T, settle time.Duration) (*dclient, *Server) {
	t.Helper()
	files := quickFiles()
	c, srv := startQuickWith(t, settle, files)
	want(t, c.next("a.gp"), 1, "")
	want(t, c.next("b.gp"), 1, "")
	srv.untilIdle(t)
	c.quiet("a.gp", 0)
	return c, srv
}

// startQuickWith starts a server over files with the settle delay, and opens a.gp and b.gp. The
// publishes of the first full run are left for the caller to read.
func startQuickWith(t *testing.T, settle time.Duration, files map[string]string) (*dclient, *Server) {
	t.Helper()
	var srv *Server
	c := startDiag(t, testConfig(), files, func(s *Server) {
		s.diags.settle = settle
		srv = s
	})
	c.open("a.gp", files["a.gp"])
	c.open("b.gp", files["b.gp"])
	return c, srv
}

func TestDiagnosticsQuickBodyEdit(t *testing.T) {
	checkGoroutines(t)
	c, srv := startQuick(t, time.Hour)
	checks := func() int64 {
		_, _, n := srv.pipeline.Stats()
		return n
	}
	before := checks()
	focusedBefore := srv.pipeline.FocusedChecks()

	c.change("a.gp", 2, strings.Replace(quickFiles()["a.gp"], "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")

	t.Run("it is a focused check", func(t *testing.T) {
		if got := srv.pipeline.FocusedChecks() - focusedBefore; got != 1 {
			t.Errorf("%d focused checks, want 1", got)
		}
	})
	t.Run("it is not a full check", func(t *testing.T) {
		if got := checks() - before; got != 0 {
			t.Errorf("%d full checks, want 0", got)
		}
	})
	t.Run("the other file gets no publish", func(t *testing.T) {
		c.quiet("b.gp", 0) // the quick publish sent the edited file only
	})
}

func TestDiagnosticsQuickKeepsFixes(t *testing.T) {
	checkGoroutines(t)
	c, _ := startQuick(t, time.Hour)
	src := quickFiles()["a.gp"]

	c.change("a.gp", 2, strings.Replace(src, "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")

	c.change("a.gp", 3, strings.Replace(src, "return 1", "return 2", 1))
	want(t, c.next("a.gp"), 3, "")
}

// TestDiagnosticsQuickVersionOnlyChange: a didChange that only raises the version of the other open
// file (the same text) is published at that version, as the full run does; the quick publish of the
// edited file alone would leave the client with the old version of the other one.
func TestDiagnosticsQuickVersionOnlyChange(t *testing.T) {
	checkGoroutines(t)
	c, _ := startQuick(t, time.Hour)
	c.change("a.gp", 2, strings.Replace(quickFiles()["a.gp"], "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")

	c.change("b.gp", 2, quickFiles()["b.gp"])

	want(t, c.next("b.gp"), 2, "")
}

// dupFiles are quickFiles with a function that both files declare: the error is in b.gp, and its
// related place is in a.gp.
func dupFiles() map[string]string {
	files := quickFiles()
	files["a.gp"] += "\nfunc dup() {}\n"
	files["b.gp"] += "\nfunc dup() {}\n"
	return files
}

// TestDiagnosticsQuickMovedRelatedPlace: an edit that adds a line to a.gp moves the related place
// of the error in b.gp; the quick publish does not send b.gp, so it must decline and leave the
// correction to the full run.
func TestDiagnosticsQuickMovedRelatedPlace(t *testing.T) {
	checkGoroutines(t)
	files := dupFiles()
	c, srv := startQuickWith(t, time.Hour, files)
	const redeclared = "9:5-9:8 error: dup redeclared in this block\n" +
		"\ta.gp %d:5-%d:8 other declaration of dup\n"
	want(t, c.next("a.gp"), 1, "")
	want(t, c.next("b.gp"), 1, fmt.Sprintf(redeclared, 6, 6))
	srv.untilIdle(t)
	c.quiet("a.gp", 0)

	c.change("a.gp", 2, strings.Replace(files["a.gp"], "return 1", "x := 1\n\treturn x", 1))

	want(t, c.next("a.gp"), 2, "")
	want(t, c.next("b.gp"), 1, fmt.Sprintf(redeclared, 7, 7))
}

// TestDiagnosticsQuickLineMappings compares every published URI with the no-focus server, not
// only the edited physical file. A different physical file can report an error at that URI, and
// the edited file can report at a virtual URI instead. Both need the full check from the start.
func TestDiagnosticsQuickLineMappings(t *testing.T) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		forms := []struct{ name, text string }{
			{"line", "//line %s:100"},
			{"block", "/*line %s:100*/"},
		}
		for _, form := range forms {
			for _, otherFile := range []bool{true, false} {
				name := fmt.Sprintf("phase %d/%s/other file %v", phase, form.name, otherFile)
				t.Run(name, func(t *testing.T) {
					checkGoroutines(t)
					files := quickFiles()
					logical := "virtual.gp"
					message := "missingName"
					replacement := "return missingName"
					if otherFile {
						logical = "a.gp"
						message = "missingInB"
						replacement = "return 2"
						files["b.gp"] = "package main\n\n" + fmt.Sprintf(form.text, logical) +
							"\nfunc main() {\n\t_ = f()\n\t_ = missingInB\n}\n"
					} else {
						files["a.gp"] = "package main\n\n" + fmt.Sprintf(form.text, logical) +
							"\nfunc f() int {\n\treturn 1\n}\n"
					}
					edited := strings.Replace(files["a.gp"], "return 1", replacement, 1)
					gotFirst, gotAll := runLineMappedEdit(
						t, files, edited, logical, message, phase, false,
					)
					wantFirst, wantAll := runLineMappedEdit(
						t, files, edited, logical, message, phase, true,
					)
					if gotFirst != wantFirst {
						t.Errorf("first edited-file publish:\n%s\nwithout focus:\n%s",
							gotFirst, wantFirst)
					}
					if !reflect.DeepEqual(gotAll, wantAll) {
						t.Errorf("all publication URIs:\n%v\nwithout focus:\n%v", gotAll, wantAll)
					}
				})
			}
		}
	}
}

// runLineMappedEdit waits for the first edited version and its logical-file diagnostic. The hour
// settle delay prevents a later full check from concealing a dropped first diagnostic.
func runLineMappedEdit(
	t *testing.T, files map[string]string, edited, logical, message string, phase int, noFocus bool,
) (string, map[string]string) {
	t.Helper()
	cfg := testConfig()
	cfg.Phase = phase
	var srv *Server
	c := startDiag(t, cfg, files, func(s *Server) {
		s.diags.settle = time.Hour
		s.diags.noFocus = noFocus
		srv = s
	})
	c.open("a.gp", files["a.gp"])
	c.open("b.gp", files["b.gp"])
	c.firstPublishAt("a.gp", 1)
	c.firstPublishAt("b.gp", 1)
	srv.untilIdle(t)
	if logical == "a.gp" && !strings.Contains(c.lastPublish(logical), message) {
		t.Fatal("the initial full check lacks the mapped diagnostic")
	}

	c.change("a.gp", 2, edited)
	first := c.firstPublishAt("a.gp", 2)
	if got := srv.pipeline.FocusedChecks(); got != 0 {
		t.Fatalf("a line-mapped package made %d focused checks", got)
	}
	logicalVersion := 0
	if logical == "a.gp" {
		logicalVersion = 2
	}
	if diagnostic := c.firstPublishAt(logical, logicalVersion); !strings.Contains(diagnostic, message) {
		t.Fatalf("first publish at %s lost %s: %s", logical, message, diagnostic)
	}
	srv.untilIdle(t)

	// Record the client's state for every URI, including virtual files outside the physical pair.
	c.pubMu.Lock()
	defer c.pubMu.Unlock()
	all := make(map[string]string)
	for _, publication := range c.publishes {
		name, err := filepath.Rel(c.root, publication.URI.Path())
		if err != nil {
			t.Fatal(err)
		}
		all[filepath.ToSlash(name)] = fmt.Sprintf("v%d:\n%s", publication.Version, format(publication))
	}
	return first, all
}

// TestDiagnosticsQuickBrokenOverlay: after a failed didChange the text is not the editor's, so the
// diagnostics of the edited file are cleared by the full run, as they are without the quick path.
func TestDiagnosticsQuickBrokenOverlay(t *testing.T) {
	checkGoroutines(t)
	c, srv := startQuick(t, time.Hour)
	c.change("a.gp", 2, strings.Replace(quickFiles()["a.gp"], "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")
	_, _, fullBefore := srv.pipeline.Stats()

	// A range that ends before it starts: the server cannot apply the change.
	const backwards = `{"textDocument":{"uri":%q,"version":3},"contentChanges":[` +
		`{"range":{"start":{"line":2,"character":5},"end":{"line":1,"character":0}},"text":"x"}]}`
	c.notify("textDocument/didChange", fmt.Sprintf(backwards, c.uri("a.gp")))

	c.readNotification("window/showMessage")
	want(t, c.next("a.gp"), 2, "")

	// diagnose also clears broken files, so the empty publish alone does not prove that the quick
	// path declined. The retained text has never been fully checked and must be checked now.
	if _, _, fullAfter := srv.pipeline.Stats(); fullAfter-fullBefore != 1 {
		t.Errorf("%d full checks after the broken edit, want 1", fullAfter-fullBefore)
	}
}

// TestDiagnosticsQuickBaseReplaced: when the full run publishes another package for the folder
// while the focused check is being made, the focused diagnostics are not published (they are
// against a base that is gone), and the run goes on with the full check.
func TestDiagnosticsQuickBaseReplaced(t *testing.T) {
	checkGoroutines(t)
	ran := make(chan struct{}, 4)
	files := quickFiles()
	var srv *Server
	c := startDiag(t, testConfig(), files, func(s *Server) {
		s.diags.settle = time.Hour
		srv = s
	})
	c.open("a.gp", files["a.gp"])
	want(t, c.next("a.gp"), 1, "")
	c.open("b.gp", files["b.gp"])
	want(t, c.next("b.gp"), 1, "")
	srv.untilIdle(t)
	c.quiet("a.gp", 0)
	srv.diags.checked = func(_ context.Context, dir string, _ uint64, _ int) {
		ran <- struct{}{}
		if len(ran) == 1 {
			srv.diags.mu.Lock()
			srv.diags.pkgs[dir].pkg = nil // the base is replaced
			srv.diags.mu.Unlock()
		}
	}

	c.change("a.gp", 2, strings.Replace(files["a.gp"], "return 1", "return undefinedName", 1))

	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")
	wait(t, ran) // the quick run
	wait(t, ran) // the full run that followed
}

func TestDiagnosticsDeclarationEditIsFull(t *testing.T) {
	checkGoroutines(t)
	c, srv := startQuick(t, time.Hour)
	_, _, before := srv.pipeline.Stats()
	focusedBefore := srv.pipeline.FocusedChecks()

	// Renaming f changes the declarations: b.gp has an error now, which only the full check finds.
	c.change("a.gp", 2, strings.Replace(quickFiles()["a.gp"], "func f()", "func g()", 1))
	want(t, c.next("b.gp"), 1, "5:5-5:6 error: undefined: f\n")

	_, _, after := srv.pipeline.Stats()
	if after-before != 1 {
		t.Errorf("%d full checks, want 1", after-before)
	}
	if got := srv.pipeline.FocusedChecks() - focusedBefore; got != 0 {
		t.Errorf("%d focused checks, want 0: the edit is not inside a body", got)
	}
}

func TestDiagnosticsSettleRunsFullCheck(t *testing.T) {
	checkGoroutines(t)
	ran := make(chan struct{}, 4)
	files := quickFiles()
	var srv *Server
	c := startDiag(t, testConfig(), files, func(s *Server) {
		s.diags.settle = time.Millisecond
		srv = s
		s.diags.checked = func(context.Context, string, uint64, int) { ran <- struct{}{} }
	})
	c.open("a.gp", files["a.gp"])
	want(t, c.next("a.gp"), 1, "")
	c.open("b.gp", files["b.gp"])
	want(t, c.next("b.gp"), 1, "")
	srv.untilIdle(t)
	c.quiet("a.gp", 0)
	for len(ran) > 0 {
		<-ran
	}
	_, _, before := srv.pipeline.Stats()

	c.change("a.gp", 2, strings.Replace(files["a.gp"], "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")
	wait(t, ran) // the quick run
	wait(t, ran) // the full run that settles it

	t.Run("it checks the package in full", func(t *testing.T) {
		if _, _, after := srv.pipeline.Stats(); after-before != 1 {
			t.Errorf("%d full checks, want 1", after-before)
		}
	})
	t.Run("it publishes nothing that was sent", func(t *testing.T) {
		srv.untilIdle(t)
		c.quiet("a.gp", 0)
		c.quiet("b.gp", 0)
	})
}

// TestDiagnoseOnlyTheEditedFile: a focused package has false errors in the other files (an import
// that only their bodies use is not used), so the diagnostics of a quick run are those of the
// edited file alone.
func TestDiagnoseOnlyTheEditedFile(t *testing.T) {
	checkGoroutines(t)
	c, srv := startQuick(t, time.Hour)
	c.change("a.gp", 2, strings.Replace(quickFiles()["a.gp"], "return 1", "return undefinedName", 1))
	want(t, c.next("a.gp"), 2, "3:8-3:21 error: undefined: undefinedName\n")

	snap, release := srv.session.Snapshot()
	defer release()
	pkg, err := snap.Focused(context.Background(), c.root, srv.diags.quickBase(c.root))
	if err != nil || pkg == nil {
		t.Fatalf("no focused package: %v", err)
	}
	aPath := filepath.Join(c.root, "a.gp")
	bPath := filepath.Join(c.root, "b.gp")
	open := make(map[string]*cache.Overlay)
	for _, o := range snap.Overlays() {
		open[o.URI().Path()] = o
	}

	only := diagnose(context.Background(), snap, c.root, pkg, open, true, aPath)
	all := diagnose(context.Background(), snap, c.root, pkg, open, true, "")

	t.Run("it has the edited file only", func(t *testing.T) {
		if len(only) != 1 || only[aPath] == nil {
			t.Errorf("files %v", only)
		}
	})
	t.Run("they are the diagnostics the file has among all", func(t *testing.T) {
		if !reflect.DeepEqual(only[aPath], all[aPath]) {
			t.Errorf("only %+v\nall  %+v", only[aPath], all[aPath])
		}
	})
	t.Run("the other file has false ones among all", func(t *testing.T) {
		if len(all[bPath]) == 0 {
			t.Error("b.gp has no diagnostic in the focused package: the filter is not needed")
		}
	})
}

// errorOrderCases are the edits of the error-order guard of the focused check (focusErrorFiles in
// internal/lsp/cache): go/types leaves out a later error with "invalid operand" or "invalid type"
// after the start of its message once it has reported one. Case 1: an error in a body of another
// file, which the focused check leaves out, comes first in the full check, so the full check drops
// the edited file's error. Case 2: the edit adds the first error, so the full check drops another
// file's error, which the quick publish would leave on the client. Each in a plain package and with
// methods of generic types; case 1 also with a gate error in the place of an error of go/types.
var errorOrderCases = []struct {
	name     string
	a, b     string
	edit     string // lib/a.gp or lib/b.gp
	old, new string
}{
	{"case 1, plain",
		"package lib\n\nfunc A() {\n\tunusedA := 1\n}\n",
		"package lib\n\nfunc B() error {\n\treturn nil\n}\n",
		"lib/b.gp", "return nil", "return \"invalid type\""},
	{"case 1, a method of a generic type",
		"package lib\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T {\n\tunusedA := 1\n\treturn b.v\n}\n",
		"package lib\n\nfunc B() error {\n\treturn nil\n}\n",
		"lib/b.gp", "return nil", "return \"invalid type\""},
	// errors.Trace is undefined for go/types; at the product's phase a gate error takes the place of
	// that error (diag.Diagnostic.Replaces).
	{"case 1, a gate error in the place of an error of go/types",
		"package lib\n\nimport \"errors\"\n\nfunc A() {\n\t_ = errors.Trace\n}\n",
		"package lib\n\nfunc B() error {\n\treturn nil\n}\n",
		"lib/b.gp", "return nil", "return \"invalid type\""},
	{"case 2, plain",
		"package lib\n\nfunc A() {\n\t_ = 1\n}\n",
		"package lib\n\nfunc B() error {\n\treturn \"invalid type\"\n}\n",
		"lib/a.gp", "_ = 1", "unusedA := 1"},
	{"case 2, a method of a generic type",
		"package lib\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T {\n\t_ = 1\n\treturn b.v\n}\n",
		"package lib\n\ntype Pair[T any] struct{ v T }\n\nfunc (p Pair[T]) B() error {\n\treturn \"invalid type\"\n}\n",
		"lib/a.gp", "_ = 1", "unusedA := 1"},
}

// TestDiagnosticsQuickErrorOrder: for each of errorOrderCases, the first publish of the edited
// version and the last publish of the other file are those of a server without the focused check.
func TestDiagnosticsQuickErrorOrder(t *testing.T) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		for _, tt := range errorOrderCases {
			t.Run(fmt.Sprintf("phase %d/%s", phase, tt.name), func(t *testing.T) {
				checkGoroutines(t)
				files := map[string]string{"gocpp.mod": "module example.com/m\n", "lib/a.gp": tt.a, "lib/b.gp": tt.b}
				other := "lib/a.gp"
				if tt.edit == other {
					other = "lib/b.gp"
				}
				run := func(noFocus bool) (first, last string) {
					cfg := testConfig()
					cfg.Phase = phase
					var srv *Server
					c := startDiag(t, cfg, files, func(s *Server) {
						s.diags.settle = time.Hour // a quick publish stays what the client has
						s.diags.noFocus = noFocus
						srv = s
					})
					c.open("lib/a.gp", files["lib/a.gp"])
					c.open("lib/b.gp", files["lib/b.gp"])
					c.firstPublishAt("lib/a.gp", 1)
					c.firstPublishAt("lib/b.gp", 1)
					srv.untilIdle(t)
					c.change(tt.edit, 2, strings.Replace(files[tt.edit], tt.old, tt.new, 1))
					first = c.firstPublishAt(tt.edit, 2)
					if n := srv.pipeline.FocusedChecks(); n != 0 {
						t.Fatalf("%d focused checks (the first publish:\n%s)", n, first)
					}
					srv.untilIdle(t)
					return first, c.lastPublish(other)
				}
				gotFirst, gotLast := run(false)
				wantFirst, wantLast := run(true)
				if gotFirst != wantFirst {
					t.Errorf("the first publish of %s:\n%s\nwithout the focused check:\n%s", tt.edit, gotFirst, wantFirst)
				}
				if gotLast != wantLast {
					t.Errorf("%s:\n%s\nwithout the focused check:\n%s", other, gotLast, wantLast)
				}
			})
		}
	}
}
