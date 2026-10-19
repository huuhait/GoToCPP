package gen

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// Mode says how a bundle treats the final panic of programs that have one.
type Mode int

const (
	// Recovered: every program runs under recover, a fatal tail is recovered and printed;
	// the process exits 0. This is how hundreds of programs share one build.
	Recovered Mode = iota
	// Standalone: tails are not recovered: the process dies with the panic, exit code 2.
	Standalone
)

// NoRecover (a debugging aid, difftest -norecover) makes the bundle's try not recover, so that
// the panic of a program shows its stack trace.
var NoRecover bool

// FileMarker starts each file of a multi-file bundle: "//// file: NAME" (NAME has no extension).
const FileMarker = "//// file: "

// Bundle puts the programs into one Go package main (valid in gocpp and in gc): their
// declarations, and a main that runs prog_N for each program, printing a marker before.
func Bundle(progs []*Program, mode Mode) (string, error) { return BundleFiles(progs, mode, 1) }

func importBlock(imps map[string]bool) string {
	var names []string
	for k := range imps {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("import (\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\t%q\n", n)
	}
	b.WriteString(")\n\n")
	return b.String()
}

// BundleFiles is Bundle with the programs spread over n files (n > 1: sections that start
// with FileMarker lines, one file "main" with the prelude and main, the others with the
// programs). Several files make gocpp compile the C++ of one bundle in parallel.
func BundleFiles(progs []*Program, mode Mode, n int) (string, error) {
	if n < 1 {
		n = 1
	}
	if n > len(progs) {
		n = max(1, len(progs))
	}
	imps := map[string]bool{"fmt": true}
	for _, p := range progs {
		for _, i := range p.Imports {
			imps[i] = true
		}
	}
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString(importBlock(imps))
	if n > 1 {
		b.Reset()
		b.WriteString(FileMarker + "main\n")
		b.WriteString("package main\n\nimport \"fmt\"\n\n")
	}
	// In a recovered bundle the program markers and scenario names also go to stderr (println),
	// so that what the programs print there is compared per program; a standalone program keeps
	// stderr for its panic.
	errMark := ""
	if mode == Recovered {
		errMark = "\tprintln(\"== \" + name)\n"
	}
	if NoRecover {
		b.WriteString("func try(name string, f func()) {\n\tfmt.Println(\"== \" + name)\n" + errMark + "\tf()\n}\n\n")
	} else {
		b.WriteString(`// try runs one scenario of a program: a panic that no scenario meant to make is reported.
func try(name string, f func()) {
	fmt.Println("== " + name)
` + errMark + `	defer func() {
		if r := recover(); r != nil {
			fmt.Println("UNEXPECTED PANIC:", r)
		}
	}()
	f()
}

`)
	}
	b.WriteString(`// clipS keeps the strings that statements build short.
func clipS(s string) string {
	if len(s) > 256 {
		return s[:256]
	}
	return s
}

`)
	if mode == Recovered {
		b.WriteString(`// tail runs the function that ends a program with a panic, and prints the panic.
func tail(f func()) {
	defer func() { fmt.Println("tail recovered:", recover()) }()
	f()
}

`)
	} else {
		b.WriteString("// tail runs the function that ends a program with a panic.\nfunc tail(f func()) { f() }\n\n")
	}
	if n == 1 {
		for _, p := range progs {
			fmt.Fprintf(&b, "// ---- program %d (%s) ----\n\n", p.ID, strings.Join(p.Used, ","))
			b.WriteString(p.Src)
			b.WriteString("\n")
		}
	}
	b.WriteString("func main() {\n")
	for _, p := range progs {
		fmt.Fprintf(&b, "\tfmt.Println(\"#### %d\")\n", p.ID)
		if mode == Recovered {
			fmt.Fprintf(&b, "\tprintln(\"#### %d\")\n", p.ID)
		}
		if mode == Recovered {
			fmt.Fprintf(&b, "\ttry(\"prog\", %s)\n", p.Entry)
		} else {
			fmt.Fprintf(&b, "\t%s()\n", p.Entry)
		}
	}
	b.WriteString("}\n")
	if n == 1 {
		out, err := format.Source([]byte(b.String()))
		if err != nil {
			return "", fmt.Errorf("bundle does not parse: %v", err)
		}
		return string(out), nil
	}
	out, err := format.Source([]byte(strings.SplitN(b.String(), "\n", 2)[1]))
	if err != nil {
		return "", fmt.Errorf("bundle does not parse: %v", err)
	}
	var res strings.Builder
	res.WriteString(FileMarker + "main\n" + string(out))
	per := (len(progs) + n - 1) / n
	for f := 0; f*per < len(progs); f++ {
		part := progs[f*per : min((f+1)*per, len(progs))]
		fi := map[string]bool{}
		for _, p := range part {
			for _, i := range p.Imports {
				fi[i] = true
			}
		}
		var fb strings.Builder
		fb.WriteString("package main\n\n")
		fb.WriteString(importBlock(fi))
		for _, p := range part {
			fmt.Fprintf(&fb, "// ---- program %d (%s) ----\n\n", p.ID, strings.Join(p.Used, ","))
			fb.WriteString(p.Src)
			fb.WriteString("\n")
		}
		fo, err := format.Source([]byte(fb.String()))
		if err != nil {
			return "", fmt.Errorf("bundle file does not parse: %v", err)
		}
		fmt.Fprintf(&res, "\n%sp%d\n%s", FileMarker, f, fo)
	}
	return res.String(), nil
}
