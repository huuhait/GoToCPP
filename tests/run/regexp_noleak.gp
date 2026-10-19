// phase: 09
//go:build linux

// Compiling a regular expression and dropping it frees everything: 30000 patterns (a parse tree with captures, repeats and classes, a
// one-pass program, a pattern with a character class) are compiled, matched once and dropped, and the resident memory of the process
// grows by less than 8 MB (it grows by 12 to 72 KB in g++ and clang++, with and without -release, and by 3 MB in Go, whose heap has not shrunk; without the two patches of tools/portstd, regexp.rules and regexp__syntax.rules, it grows by 110 MB, 3.7 KB a compile, so a leak of 270 bytes a compile fails). Go's
// code builds a syntax.Regexp whose Sub and Rune slices point into the fields Sub0 and Rune0 of the same Regexp, and makeOnePass's closure
// calls itself through a variable: both are reference cycles in gocpp (ADR-004), so every Compile kept its trees and programs.

package main

import (
	"fmt"
	"os"
	"regexp"
	"regexp/syntax"
	"runtime/debug"
	"strconv"
	"strings"
)

func rssPages() int {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		panic(err)
	}
	f := strings.Fields(string(b))
	n, _ := strconv.Atoi(f[1])
	return n
}

var pats = []string{`(a|b)+c{1,3}[x-z]\d*(?:foo|bar)?`, `^(\d+)-(\d+)-(\d+)$`, `[^a-c\n]+é世\p{Greek}`}

func once(i int) int {
	re := regexp.MustCompile(pats[i%3])
	n := 0
	if re.MatchString("abbcx1foo 12-34-56 zzé世α") {
		n++
	}
	if i%10 == 0 {
		sr, _ := syntax.Parse(pats[i%3], syntax.Perl)
		pr, _ := syntax.Compile(sr.Simplify())
		n += len(pr.Inst)
	}
	return n
}

func main() {
	once(0)
	once(1)
	debug.FreeOSMemory() // Go collects lazily and gocpp's allocator keeps freed blocks: the resident size is measured after returning memory to the OS
	before := rssPages()
	n := 0
	for i := 0; i < 30000; i++ {
		n += once(i)
	}
	debug.FreeOSMemory()
	grown := (rssPages() - before) * 4 // KB
	if strings.Contains(os.Getenv("GOCPP_FLAGS"), "asan") {
		grown = 0 // AddressSanitizer keeps freed blocks (a quarantine of 256 MB), so the resident size says nothing; LeakSanitizer reports a leak at exit
	}
	fmt.Println(n, grown < 8192)
}

// Output:
// 67000 true
