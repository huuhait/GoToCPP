package check

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/version"
)

// featureMatrix returns docs/spec/feature-matrix.md: the gate tables of
// gates.go as a page for users. TestFeatureMatrix checks that the file is
// up to date (and writes it with -update).
func featureMatrix() string {
	status := fmt.Sprintf("The features of Phases 00 to %02d work (`gocpp version` prints `phase %02d`); the rows with a later phase are rejected.",
		version.Phase, version.Phase)
	var b strings.Builder
	b.WriteString(`# Feature matrix

<!-- Generated from internal/check/gates.go. Don't edit it: change the
tables there and run go test ./internal/check -run TestFeatureMatrix -update-gates -->

This page says which phase of the [plan](../../plans/2026-10-13-gocpp/README.md) brings each
feature of gocpp. ` + status + `
Until a phase is finished, the compiler rejects its features with an error that names the phase,
so you never get broken C++ (a few features of a later phase work earlier, and say so in their row,
for example "06 (works from 05)"):

` + "```" + `
./main.gp:5:12: interfaces are not supported yet (Phase 05)
./main.gp:9:2: package strings is not available yet (Phase 09)
` + "```" + `

Only your own packages are checked: the standard library can use everything. Each file gets one
error for each feature, at its first use, and each use gets one error, for its first missing
feature. The [language](language.md) describes each feature.

## From the start (Phase 02)

Phase 02 builds a small part of the language, and every phase has it: ` + "`package main`" + `,
functions with parameters and several results, the basic types (` + "`bool`" + `, the integer
types, ` + "`float32`" + `, ` + "`float64`" + `, ` + "`string`" + `), constants (with ` + "`iota`" + `), local variables,
` + "`:=`" + ` and assignments, all operators, ` + "`if`" + `, all ` + "`for`" + ` loops (also ` + "`range`" + ` over an
integer), ` + "`switch`" + ` (with ` + "`fallthrough`" + `), labels with ` + "`break`" + `, ` + "`continue`" + ` and
` + "`goto`" + `, string concatenation and comparison, ` + "`len`" + ` of a string, the byte ` + "`s[i]`" + ` of a
string, conversions between number types, the built-in functions
` + "`print`" + `, ` + "`println`" + ` and ` + "`panic`" + `, and ` + "`fmt.Print`" + `, ` + "`fmt.Println`" + ` and
` + "`fmt.Printf`" + ` as statements, with values of basic types (` + "`fmt.Printf`" + ` with the verbs
` + "`%v %d %s %t %f %%`" + `).

`)

	b.WriteString(`## Always available (Phase 10)

The syntax and the meaning of these have no gate and no row below, at every phase and in every command
(` + "`check`, `build`, `run`, `test`, `vet`, `expand`, `fmt`, `lsp`" + `), with no ` + "`GOCPP_GATE_PHASE`" + `
([ADR-095](../adr/095-must-and-macros-are-always-available.md)): ` + "`!`" + ` (must) calls and must functions
(divergence D12), macros with ` + "`lazy`" + ` parameters and the built-in ` + "`#file()`, `#line()`, `#func()`, `#text()`, `#body()`, `#embed()`" + `
(D13, D18), ` + "`#Derive(String, Equal, Clone)`" + `, and ` + "`//go:embed`" + ` on ` + "`string`" + ` and ` + "`[]byte`" + ` variables
(the ` + "`embed`" + ` row below). What such a program uses from the later phases is still gated by those phases
(a must function that imports ` + "`strings`" + ` needs Phase 09).

`)
	b.WriteString("## Language features\n\n| Feature | Phase | Notes |\n|---|---|---|\n")
	rows := slices.Clone(features[:])
	slices.SortStableFunc(rows, func(a, b feature) int { return cmp.Compare(a.Phase, b.Phase) })
	for _, f := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", mdName(f.Name), featurePhaseText(f), f.Notes)
	}

	b.WriteString("\n## Standard packages\n\n" +
		"Most other standard packages come in Phase " + phaseText(otherStdPhase) + " (its README lists them " +
		"by tier). A few come later, with no phase yet: `encoding/csv`, `compress/gzip`, `archive/zip`, " +
		"`os/signal` and `net`.\n\n" +
		"| Package | Phase | Notes |\n|---|---|---|\n")
	pkgs := slices.Clone(stdPackages)
	slices.SortStableFunc(pkgs, func(a, b stdPackage) int { return cmp.Compare(a.Phase, b.Phase) })
	for _, p := range pkgs {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", p.Path, packagePhaseText(p), p.Notes)
	}

	b.WriteString("\n## Parts of packages\n\n" +
		"These come in another phase than their package. A method comes with its type, unless it " +
		"has a row of its own.\n\n| Name | Phase |\n|---|---|\n")
	members := slices.Clone(stdMembers)
	slices.SortStableFunc(members, func(a, b stdMember) int { return cmp.Compare(a.Phase, b.Phase) })
	for _, m := range members {
		fmt.Fprintf(&b, "| `%s.%s` | %s |\n", m.Pkg, m.Name, phaseText(m.Phase))
	}
	for _, p := range stdPackages {
		if p.Rest > p.Phase {
			fmt.Fprintf(&b, "| the rest of `%s` | %s |\n", p.Path, phaseText(p.Rest))
		}
	}

	b.WriteString("\n## Format strings\n\n" +
		"Before Phase 05 the compiler checks the verbs of the format strings of `fmt.Printf`, " +
		"`fmt.Sprintf` and `fmt.Errorf`, and a format string must be a constant. From Phase 05 " +
		"`fmt` checks nothing at compile time: every verb of Go's `fmt` works, and a bad one prints " +
		"Go's error form at run time (`%!z(int=1)`). Width, precision and flags are in the table of " +
		"language features.\n\n" +
		"| Verbs | Phase |\n|---|---|\n")
	byPhase := make(map[int][]string)
	var phases []int
	for _, v := range formatVerbs {
		if byPhase[v.Phase] == nil {
			phases = append(phases, v.Phase)
		}
		byPhase[v.Phase] = append(byPhase[v.Phase], fmt.Sprintf("`%%%c`", v.Verb))
	}
	for _, p := range phases {
		fmt.Fprintf(&b, "| %s | %s |\n", strings.Join(byPhase[p], " "), phaseText(p))
	}
	fmt.Fprintf(&b, "| the other verbs of Go's `fmt` | %s |\n", phaseText(verbPhase('x')))

	b.WriteString("\n## Built-in functions\n\n" +
		"`len` (of a string), `print`, `println` and `panic` work from Phase 02.\n\n" +
		"| Function | Phase |\n|---|---|\n")
	bs := slices.Clone(builtins)
	slices.SortStableFunc(bs, func(a, b stdMember) int { return cmp.Compare(a.Phase, b.Phase) })
	for _, f := range bs {
		fmt.Fprintf(&b, "| `%s` | %s |\n", f.Name, phaseText(f.Phase))
	}
	return b.String()
}

// featurePhaseText returns the phase of a feature for the matrix: "03", or
// "06 (works from 05)" when the gate lets it through earlier (Since).
func featurePhaseText(f feature) string {
	if f.Since != 0 && f.Since < f.Phase {
		return fmt.Sprintf("%s (works from %s)", phaseText(f.Phase), phaseText(f.Since))
	}
	return phaseText(f.Phase)
}

// packagePhaseText returns the phase of a standard package for the matrix, like featurePhaseText.
func packagePhaseText(p stdPackage) string {
	if p.Since != 0 && p.Since < p.Phase {
		return fmt.Sprintf("%s (works from %s)", phaseText(p.Phase), phaseText(p.Since))
	}
	return phaseText(p.Phase)
}

// phaseText returns the phase for the matrix: "03", or "never".
func phaseText(phase int) string {
	if phase == Never {
		return "never"
	}
	return fmt.Sprintf("%02d", phase)
}

// mdName returns the name of a feature for a Markdown table, with the
// parts that are code in backquotes: "//gocpp: directives" gives
// "`//gocpp:` directives", "C.wstr and C.GoStringW" gives
// "`C.wstr` and `C.GoStringW`".
func mdName(name string) string {
	if strings.HasPrefix(name, "//") {
		code, rest, _ := strings.Cut(name, " ")
		if rest != "" {
			rest = " " + rest
		}
		return "`" + code + "`" + rest
	}
	return strings.NewReplacer("C.wstr", "`C.wstr`", "C.GoStringW", "`C.GoStringW`").Replace(name)
}
