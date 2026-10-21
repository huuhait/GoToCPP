package load

import (
	"fmt"
	"go/token"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The tags of a cxx recipe's conditions (ADR-129). A condition is a build
// constraint, so a tag that nobody defines is false and not an error; the
// loader warns about it, since it is nearly always a typo (if widnows {).

// knownGOOS and knownGOARCH are the system and architecture names that Go
// knows (go/build's lists): a condition may name any of them, also for a target
// other than this one.
var (
	knownGOOS = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux", "nacl",
		"netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos",
	}
	knownGOARCH = []string{
		"386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "loong64", "mips", "mipsle", "mips64",
		"mips64le", "mips64p32", "mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390", "s390x",
		"sparc", "sparc64", "wasm",
	}

	// otherTags are the tags that are not a system or an architecture and that
	// a build line satisfies or not by itself: unix, the compilers, cgo.
	otherTags = []string{"unix", "gocpp", "gc", "gccgo", "cgo"}

	releaseTagRE = regexp.MustCompile(`^go1\.[0-9]+$`)
)

// warnUnknownTags adds a warning for each tag of a condition in the recipe of
// c that is not a system, an architecture, unix, gocpp, a compiler or cgo tag,
// a release tag go1.N or one of the tags of -tags (Config.Tags). If a tag
// is close to a known one (two edits), the warning says "did you mean".
func (l *loader) warnUnknownTags(c *CxxModule) {
	if c.Recipe == nil {
		return
	}
	r, _ := gocppmod.ParseRecipe(c.RecipeFile, c.Module.Path, c.Recipe)
	known := slices.Concat(knownGOOS, knownGOARCH, otherTags, c.Tags)
	for _, tag := range r.Tags {
		if slices.Contains(known, tag.Name) || releaseTagRE.MatchString(tag.Name) {
			continue
		}
		msg := fmt.Sprintf("cxx %s: the tag %q in a condition is not an operating system, an architecture, unix, "+
			"gocpp, go1.N or a tag of -tags, so it is never satisfied", c.Module.Path, shortTag(tag.Name))
		if near := nearestTag(tag.Name, known); near != "" {
			msg += "; did you mean " + near + "?"
		}
		l.prog.Errors.Add(diag.Diagnostic{
			Pos:      token.Position{Filename: c.RecipeFile, Line: tag.Line, Column: tag.Col},
			Severity: diag.Warning,
			Msg:      msg,
		})
	}
}

// shortTag cuts a long tag for a message.
func shortTag(tag string) string {
	if utf8.RuneCountInString(tag) <= 40 {
		return tag
	}
	return string([]rune(tag)[:40]) + "..."
}

// nearestTag returns the name in names that is at most two edits from tag
// (and fewer edits than tag has letters), the nearest, the first of those; or "".
func nearestTag(tag string, names []string) string {
	best, bestDist := "", 3
	for _, n := range names {
		if d := editDistance(tag, n); d < bestDist && d < utf8.RuneCountInString(tag) {
			best, bestDist = n, d
		}
	}
	return best
}

// editDistance is the number of single-letter insertions, deletions and
// substitutions that turn a into b (the Levenshtein distance). The inputs are
// cut at 64 runes: a tag that long is no typo of a known one.
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	if len(x) > 64 || len(y) > 64 {
		return 3
	}
	prev := make([]int, len(y)+1)
	cur := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(x); i++ {
		cur[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(y)]
}
