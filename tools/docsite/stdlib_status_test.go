package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/stdcount"
)

// Non-portstd packages need a reviewed classification. A new package must either appear in
// PORTED.txt or get a classification here; a removed package leaves an error, not a stale row.
var manualStdlibKinds = map[string]string{
	"embed":                           "stub",
	"fmt":                             "native shim",
	"internal/abi":                    "shim",
	"internal/bytealg":                "Go copy + native",
	"internal/byteorder":              "Go copy",
	"internal/cpu":                    "shim",
	"internal/filepathlite":           "Go copy",
	"internal/goarch":                 "shim",
	"internal/godebug":                "shim",
	"internal/goos":                   "shim",
	"internal/itoa":                   "Go copy",
	"internal/oserror":                "Go copy",
	"internal/poll":                   "Go copy + native",
	"internal/race":                   "shim",
	"internal/reflectlite":            "shim",
	"internal/stringslite":            "Go copy",
	"internal/syscall/windows":        "Go copy + native",
	"internal/syscall/windows/sysdll": "Go copy",
	"internal/testlog":                "shim",
	"os":                              "Go copy + native",
	"reflect":                         "native shim",
	"runtime":                         "Go copy + native",
	"runtime/cgo":                     "native shim",
	"runtime/debug":                   "Go copy + native",
	"sync":                            "Go copy + native",
	"sync/atomic":                     "Go copy + native",
	"syscall":                         "Go copy + native",
	"testing":                         "Go copy + native",
	"testing/internal/testdeps":       "shim",
	"time":                            "Go copy + native",
	"weak":                            "intrinsic shim",
}

// TestStdlibGuideInventory checks the complete tree, not just public packages. It only reads
// provenance and manifest metadata; it does not claim that tests have run or passed.
func TestStdlibGuideInventory(t *testing.T) {
	root := filepath.Join("..", "..")
	stdlib := os.DirFS(filepath.Join(root, "stdlib"))
	packages := map[string][]string{}
	err := fs.WalkDir(stdlib, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(name, ".gp") &&
			!strings.HasSuffix(name, "_test.gp") {
			pkg := path.Dir(name)
			packages[pkg] = append(packages[pkg], name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ports := map[string][]string{}
	for _, line := range strings.Split(readFile(t, filepath.Join(root, "stdlib", "PORTED.txt")), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) != 8 {
			t.Fatalf("malformed PORTED.txt row: %s", line)
		}
		ports[fields[0]] = fields
	}
	for pkg := range ports {
		if packages[pkg] == nil {
			t.Fatalf("PORTED.txt names absent package %s", pkg)
		}
	}
	for pkg := range manualStdlibKinds {
		if packages[pkg] == nil || ports[pkg] != nil {
			t.Fatalf("stale manual classification for %s", pkg)
		}
	}

	// The plan's tier table is the scheduling policy, not portstd's smaller -tier list.
	tiers := map[string]string{}
	plan := readFile(t, filepath.Join(root, "plans", "2026-10-13-gocpp",
		"phase-09-stdlib-testing", "README.md"))
	tierRows := regexp.MustCompile("(?m)^\\| \\*\\*([123])[^|]+\\| (.+) \\|$")
	for _, row := range tierRows.FindAllStringSubmatch(plan, -1) {
		for _, names := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(row[2], -1) {
			for _, pkg := range strings.Fields(names[1]) {
				tiers[pkg] = row[1]
			}
		}
	}
	if tiers["fmt"] != "1" || tiers["path"] != "2" || tiers["net"] != "3" {
		t.Fatal("cannot read the plan's package tiers")
	}

	var names []string
	for pkg := range packages {
		names = append(names, pkg)
	}
	slices.Sort(names)
	var table strings.Builder
	for _, internal := range []bool{false, true} {
		if internal {
			table.WriteString("\n### Internal support packages\n\n")
		} else {
			table.WriteString("### Public packages\n\n")
		}
		table.WriteString("| Package | Implementation | Source / port selection | Targets | Tier | Manifest selected / all |\n")
		table.WriteString("|---|---|---|---|---|---|\n")
		for _, pkg := range names {
			isInternal := slices.Contains(strings.Split(pkg, "/"), "internal")
			if internal != isInternal {
				continue
			}
			kind := manualStdlibKinds[pkg]
			source := guideSource(t, stdlib, pkg, packages[pkg][0])
			if port := ports[pkg]; port != nil {
				kind = "port"
				if port[6] != "natives=0" {
					kind = "port + native"
				}
				source += " (" + strings.TrimPrefix(port[2], "os=") + ")"
			}
			if kind == "" {
				t.Fatalf("classify new package %s in manualStdlibKinds or PORTED.txt", pkg)
			}
			tier := tiers[pkg]
			if tier == "" {
				tier = "support"
			}
			manifest := "—"
			m, err := stdcount.LoadManifest(filepath.Join(root, "tests", "stdgo"), pkg)
			if err == nil {
				count := stdcount.Tally(m, nil)
				manifest = fmt.Sprintf("[%d / %d](../../tests/stdgo/manifest/%s.json)",
					count.Selected, count.Total, strings.ReplaceAll(pkg, "/", "__"))
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			fmt.Fprintf(&table, "| `%s` | %s | %s | %s | %s | %s |\n", pkg, kind,
				source, guideTargets(t, stdlib, packages[pkg], kind), tier, manifest)
		}
	}

	file := filepath.Join(root, "docs", "guides", "stdlib.md")
	src := readFile(t, file)
	before, rest, ok := strings.Cut(src, "<!-- stdlib inventory start -->\n")
	if !ok {
		t.Fatal("stdlib guide is missing inventory start marker")
	}
	_, after, ok := strings.Cut(rest, "<!-- stdlib inventory end -->")
	if !ok {
		t.Fatal("stdlib guide is missing inventory end marker")
	}
	want := before + "<!-- stdlib inventory start -->\n" + table.String() +
		"<!-- stdlib inventory end -->" + after
	if os.Getenv("UPDATE_STDLIB_GUIDE") == "1" {
		if err := os.WriteFile(file, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	} else if src != want {
		t.Fatal("stdlib inventory is stale: UPDATE_STDLIB_GUIDE=1 go test ./tools/docsite -run '^TestStdlibGuideInventory$'")
	}
}

func TestStdlibGuideMissingPackages(t *testing.T) {
	src := readFile(t, filepath.Join("..", "..", "docs", "guides", "stdlib.md"))
	_, missing, ok := strings.Cut(src, "## Packages that are not shipped\n")
	if !ok {
		t.Fatal("stdlib guide is missing its absent-package section")
	}
	missing, _, _ = strings.Cut(missing, "\n## What differs from Go")
	packages, err := stdlibPackages(embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	names := regexp.MustCompile("`([^`]+)`")
	count := 0
	for _, line := range strings.Split(missing, "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		column := strings.Split(line, "|")[1]
		for _, name := range names.FindAllStringSubmatch(column, -1) {
			count++
			for _, pkg := range packages {
				match, err := path.Match(name[1], pkg)
				if err != nil {
					t.Fatal(err)
				}
				if match {
					t.Errorf("guide calls %s absent, but stdlib/ contains %s", name[1], pkg)
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("stdlib guide lists no missing packages")
	}
}

func guideSource(t *testing.T, stdlib fs.FS, pkg, fallback string) string {
	t.Helper()
	if pkg == "runtime/cgo" {
		return "[PORT](../../stdlib/internal/PORT.md)"
	}
	for dir := pkg; dir != "."; dir = path.Dir(dir) {
		name := path.Join(dir, "PORT.md")
		if _, err := fs.Stat(stdlib, name); err == nil {
			return "[PORT](../../stdlib/" + name + ")"
		}
	}
	return "[source](../../stdlib/" + fallback + ")"
}

func guideTargets(t *testing.T, stdlib fs.FS, files []string, kind string) string {
	t.Helper()
	if kind == "stub" {
		return "—"
	}
	var targets []string
	for _, goos := range []string{"linux", "windows"} {
		for _, file := range files {
			data, err := fs.ReadFile(stdlib, file)
			if err != nil {
				t.Fatal(err)
			}
			ok, msg := load.MatchFile(goos, "amd64", nil, path.Base(file), data)
			if msg != "" {
				t.Fatal(msg)
			}
			if ok {
				targets = append(targets, goos)
				break
			}
		}
	}
	return strings.Join(targets, ", ")
}
