package cache

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The matrix: statements that use what the gates of a phase report (go, chan, defer, pointers,
// maps, complex numbers, the forms of Phase 10) and what the gates do not know, put in the body of
// Add. At the phase of the product's default the gates report them; the focused check must report
// what the full check does for the edited file, and leave the other files' diagnostics as they
// were.

// matrixFiles are focusFiles with a file whose init function calls a function of the package.
func matrixFiles() map[string]string {
	files := focusFiles()
	files["lib/d.gp"] = "package lib\n\nfunc secret() int { return 7 }\n\nfunc init() {\n\t_ = secret()\n}\n"
	return files
}

// matrixEdits are the new texts of the line "c.n += k // EDIT" of a.gp.
var matrixEdits = []string{
	"go func() {}()",
	"ch := make(chan int, 1)\n\tch <- 1\n\t_ = <-ch",
	"defer func() {}()",
	"var p *int = &c.n\n\t_ = p",
	"m := map[string]int{\"a\": 1}\n\t_ = m",
	"s := []int{1, 2}\n\t_ = s",
	"var e error\n\t_ = e",
	"var i interface{} = k\n\t_ = i",
	"var x any = k\n\t_ = x",
	"select {}",
	"for range 3 {\n\t}",
	"type local struct{ a int }\n\t_ = local{}",
	"var f func(int) int = Helper\n\t_ = f",
	"_ = strings.ToUpper(\"x\")",
	"fmt.Println(k)",
	"os.Exit(1)",
	"_ = #func()",
	"_ = #file()",
	"_ = #line()",
	"panic(\"x\")",
	"_ = recover()",
	"var z = 3.5 + 2i\n\t_ = z",
	"L:\n\tfor {\n\t}",
	"x := 1\n\tx = 2",
	"_ = c.Missing",
	"c.n = \"s\"",
	"Helper = nil",
	"_ = (*Counter).Add",
	"_ = secret()",
	"return",
	"c.n += k\n\treturn 1",
	"var wg sync.WaitGroup\n\twg.Wait()",
	"_ = len(Names) + cap(Names)",
	"var sq Square\n\t_ = sq.Area()\n\t_ = sq.side",
	"_ = lib.Limit",
	"k := 2\n\t_ = k",
	"_ = func() int { return k }",
	"switch v := any(k).(type) {\n\tcase int:\n\t\t_ = v\n\t}",
	"var arr [3]int\n\t_ = arr",
	"x, y := 1\n\t_, _ = x, y",
	"fmt := 1\n\t_ = fmt",
	"var _ Shape = (*Counter)(nil)",
	"var _ Shape = Square{}",
}

func TestFocusedMatrix(t *testing.T) {
	for _, phase := range focusPhases {
		for _, edit := range matrixEdits {
			t.Run(phaseName(phase)+"/"+edit, func(t *testing.T) {
				files := matrixFiles()
				w, base := focusRunAt(t, files, phase)
				w.open("lib/d.gp", files["lib/d.gp"])
				base = w.pkg("lib")
				text := strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", edit+" // EDIT", 1)

				focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)

				if focused == nil {
					t.Skip("declined")
				}
				path := filepath.Join(w.root, "lib", "a.gp")
				if got, want := diagsIn(focused, path), diagsIn(full, path); !reflect.DeepEqual(got, want) {
					t.Errorf("edited file:\nfocused %+v\nfull    %+v", got, want)
				}
				for _, other := range []string{"b.gp", "c.gp", "d.gp"} {
					file := filepath.Join(w.root, "lib", other)
					if got, want := diagsIn(full, file), diagsIn(base, file); !reflect.DeepEqual(got, want) {
						t.Errorf("diagnostics of %s changed:\nbase %+v\nfull %+v", other, want, got)
					}
				}
			})
		}
	}
}

// TestFocusedTestFiles: an edit in the body of a function of a _test.gp file of the package.
func TestFocusedTestFiles(t *testing.T) {
	files := focusFiles()
	files["lib/a_test.gp"] = "package lib\n\nfunc helper() int {\n\treturn 1 // EDIT\n}\n"
	files["lib/x_test.gp"] = "package lib_test\n\nfunc helper2() int {\n\treturn 1 // EDIT\n}\n"
	for _, name := range []string{"lib/a_test.gp", "lib/x_test.gp"} {
		t.Run(name, func(t *testing.T) {
			w, base := focusRun(t, files)
			w.open(name, files[name])
			text := strings.Replace(files[name], "return 1 // EDIT", "return undefinedX // EDIT", 1)

			focused, full := focusedAndFull(t, w, base, "lib", name, text)

			if focused == nil {
				return // declined: the full check answers
			}
			path := filepath.Join(w.root, filepath.FromSlash(name))
			if got, want := diagsIn(focused, path), diagsIn(full, path); !reflect.DeepEqual(got, want) {
				t.Errorf("focused %+v\nfull    %+v", got, want)
			}
		})
	}
}

// TestFocusedLineDirectiveEarlier requires a decline for mappings already present in either the
// edited file or another package file. Checking only the edited physical path would miss errors
// whose logical filename differs, so the full-check assertion uses the mapped filename.
func TestFocusedLineDirectiveEarlier(t *testing.T) {
	for _, phase := range focusPhases {
		for _, block := range []bool{false, true} {
			for _, otherFile := range []bool{false, true} {
				name := phaseName(phase)
				if block {
					name += "/block"
				} else {
					name += "/line"
				}
				if otherFile {
					name += "/other file"
				} else {
					name += "/edited file"
				}
				t.Run(name, func(t *testing.T) {
					files := focusFiles()
					logical := "other.gp"
					if otherFile {
						logical = "a.gp"
					}
					directive := "//line " + logical + ":500"
					if block {
						directive = "/*line " + logical + ":500*/"
					}
					if otherFile {
						files["lib/b.gp"] = strings.Replace(files["lib/b.gp"], "func Join()",
							directive+"\nfunc Join()", 1)
						files["lib/b.gp"] = strings.Replace(files["lib/b.gp"], "return strings.Join",
							"_ = missingInOther\n\treturn strings.Join", 1)
					} else {
						files["lib/a.gp"] = strings.Replace(files["lib/a.gp"], "type Counter",
							directive+"\ntype Counter", 1)
					}
					w, base := focusRunAt(t, files, phase)
					if base.focusEligible() {
						t.Fatal("a base with line mappings is eligible")
					}
					text := strings.Replace(files["lib/a.gp"], "c.n += k // EDIT",
						"c.n += undefinedName // EDIT", 1)

					focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)

					if focused != nil {
						t.Fatal("a line-mapped package received a focused check")
					}
					path := filepath.Join(w.root, "lib", logical)
					if len(diagsIn(full, path)) == 0 {
						t.Fatalf("the full check has no diagnostic at mapped file %s", logical)
					}
				})
			}
		}
	}
}
