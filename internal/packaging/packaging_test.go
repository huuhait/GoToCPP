package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/toolchain"
)

// repoRoot is the folder of go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no go.mod in %s: %v", root, err)
	}
	return root
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// trackedFiles returns the files that git tracks, or skips the test outside a git checkout (an
// unpacked tarball).
func trackedFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--stage")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files: %v (not a git checkout?)", err)
	}
	var files []string
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		// "<mode> <object> <stage>\t<path>"
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			t.Fatalf("git ls-files: unexpected record %q", rec)
		}
		if strings.HasPrefix(meta, "120000") {
			t.Errorf("%s is a symbolic link: a Go module zip has no symbolic links, so go install of a tagged version would not have it", path)
			continue
		}
		if strings.HasPrefix(meta, "160000") {
			t.Errorf("%s is a submodule: a Go module zip has no submodules", path)
			continue
		}
		files = append(files, path)
	}
	return files
}

// TestRepoFilesDoNotCollide: two names that differ only in case can't be in a Go module zip (the proxy
// refuses the module, and the files could not be checked out on Windows or macOS either), the
// first thing that stopped go install in step 15.13: tests/run/fmt_bytes_X.gp and
// fmt_bytes_x.gp.
func TestRepoFilesDoNotCollide(t *testing.T) {
	seen := map[string]string{}
	for _, f := range trackedFiles(t) {
		key := strings.ToLower(f)
		if prev, ok := seen[key]; ok {
			t.Errorf("%q and %q differ only in case: a Go module zip (go install) and a checkout on Windows can't hold both", prev, f)
		}
		seen[key] = f
	}
}

// TestGoModIsInstallable: go install pkg@version refuses a module whose go.mod has replace or
// exclude directives, and the module path must be the one that the README and the flake name.
func TestGoModIsInstallable(t *testing.T) {
	gomod := readRepoFile(t, "go.mod")
	if !strings.HasPrefix(gomod, "module github.com/huuhait/gocpp\n") {
		t.Errorf("go.mod does not start with module github.com/huuhait/gocpp:\n%.200s", gomod)
	}
	for _, line := range strings.Split(gomod, "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (f[0] == "replace" || f[0] == "exclude") {
			t.Errorf("go.mod has a %s directive (%q): go install github.com/huuhait/gocpp/cmd/gocpp@version fails with it", f[0], line)
		}
	}
	src := readRepoFile(t, "cmd/gocpp/main.go")
	if !regexp.MustCompile(`(?m)^package main$`).MatchString(src) {
		t.Errorf("cmd/gocpp/main.go is not package main")
	}
}

// TestGoReleaserConfig checks what the plan promises of .goreleaser.yaml (phase-15 README §2.4):
// the three targets, a static binary with the version set, the Windows zip, checksums, and the
// llvm-mingw release in the notes. There is no YAML package in the module, so it looks at the
// lines; TestGoReleaserCheck lets GoReleaser itself check the file when it is installed.
func TestGoReleaserConfig(t *testing.T) {
	cfg := readRepoFile(t, ".goreleaser.yaml")
	for _, want := range []string{
		"version: 2\n",
		"main: ./cmd/gocpp\n",
		"- CGO_ENABLED=0\n",
		"- -trimpath\n",
		"goos:\n      - linux\n      - windows\n",
		"goarch:\n      - amd64\n      - arm64\n",
		"- goos: windows\n        goarch: arm64\n", // no Windows arm64 build
		"format_overrides:\n      - goos: windows\n        formats:\n          - zip\n",
		"files:\n      - LICENSE\n      - THIRD_PARTY_NOTICES.txt\n      - README.md\n",
		"algorithm: sha256\n",
		"draft: true\n", // nothing is public until someone publishes the release
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".goreleaser.yaml lacks %q", want)
		}
	}
	if want := "llvm-mingw release " + toolchain.Release + " (" + toolchain.Variant + ")"; !strings.Contains(cfg, want) {
		t.Errorf(".goreleaser.yaml: the release notes must name the pinned toolchain (%q): internal/toolchain.Release changed", want)
	}
	if !strings.Contains(cfg, "-X github.com/huuhait/gocpp/internal/version.Version=v{{ .Version }}") {
		t.Errorf(".goreleaser.yaml: ldflags must set internal/version.Version to v{{ .Version }}")
	}
}

func TestThirdPartyNotices(t *testing.T) {
	notices := readRepoFile(t, "THIRD_PARTY_NOTICES.txt")

	t.Run("Go license", func(t *testing.T) {
		license := strings.TrimSpace(readRepoFile(t, "stdlib/LICENSE"))
		if !strings.Contains(notices, license) {
			t.Error("THIRD_PARTY_NOTICES.txt must reproduce the full Go license")
		}
	})

	for _, tc := range []struct {
		name  string
		path  string
		start string
		end   string
	}{
		{"SunPro 1993", "stdlib/math/log.gp", "Copyright (C) 1993", "is preserved."},
		{"SunSoft 1993", "stdlib/math/cbrt.gp", "Copyright (C) 1993", "is preserved."},
		{"Sun 2004", "stdlib/math/exp.gp", "Copyright (C) 2004", "is preserved."},
		{"Cephes", "stdlib/math/sin.gp", "Cephes Math Library Release", "moshier@na-net.ornl.gov"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Strip only the Go comment prefix, keeping the upstream notice's text and spacing.
			var lines []string
			for _, line := range strings.Split(readRepoFile(t, tc.path), "\n") {
				line = strings.TrimPrefix(line, "// ")
				lines = append(lines, strings.TrimPrefix(line, "//"))
			}
			source := strings.Join(lines, "\n")
			start := strings.Index(source, tc.start)
			if start < 0 {
				t.Fatalf("%s: missing notice start %q", tc.path, tc.start)
			}
			end := strings.Index(source[start:], tc.end)
			if end < 0 {
				t.Fatalf("%s: missing notice end %q", tc.path, tc.end)
			}
			notice := source[start : start+end+len(tc.end)]
			if !strings.Contains(notices, notice) {
				t.Errorf("THIRD_PARTY_NOTICES.txt must preserve the notice from %s", tc.path)
			}
		})
	}
}

// TestGoReleaserCheck runs goreleaser check, when goreleaser is installed (GOCPP_REQUIRE_PACKAGING=1
// makes a missing one an error).
func TestGoReleaserCheck(t *testing.T) {
	gr, err := exec.LookPath("goreleaser")
	if err != nil {
		if os.Getenv("GOCPP_REQUIRE_PACKAGING") != "" {
			t.Fatal("goreleaser is not installed")
		}
		t.Skip("goreleaser is not installed (nix shell nixpkgs#goreleaser)")
	}
	cmd := exec.Command(gr, "check")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("goreleaser check: %v\n%s", err, out)
	}
}

// vendorStamp checks the vendorHash of flake.nix against go.mod and go.sum: stale is what is wrong
// with the stamp (the hash goes with the two files, so it must be made again when they change),
// bad is what is wrong with the hash itself.
func vendorStamp(flake, gomod, gosum string) (stale, bad string) {
	m := regexp.MustCompile(`go\.mod\+go\.sum sha256: ([0-9a-f]{64})`).FindStringSubmatch(flake)
	if m == nil {
		return "", "flake.nix: no 'go.mod+go.sum sha256: <hex>' comment above vendorHash"
	}
	sum := sha256.Sum256([]byte(gomod + gosum))
	if got := hex.EncodeToString(sum[:]); got != m[1] {
		stale = fmt.Sprintf("go.mod or go.sum changed since the vendorHash of flake.nix was made (stamp %s, now %s): run make flake-hash", m[1], got)
	}
	if !regexp.MustCompile(`vendorHash = "sha256-[A-Za-z0-9+/]{43}=";`).MatchString(flake) {
		bad = `flake.nix: vendorHash is not a sha256 SRI hash (lib.fakeHash left in?)`
	}
	return stale, bad
}

// TestFlakeVendorHash: the vendorHash of the Nix package belongs to go.mod and go.sum. The
// comment above it holds the sha256 of both files at the time the hash was made; when they
// change, the hash must be made again (make flake-hash), or nix build fails with a hash
// mismatch for everyone. A stale stamp is a failure only under GOCPP_REQUIRE_PACKAGING=1 (make
// package-check, the release steps); otherwise it is logged and the test skips, so that changing
// a dependency does not turn go test red until the hash is made (it needs Nix and the network).
func TestFlakeVendorHash(t *testing.T) {
	stale, bad := vendorStamp(readRepoFile(t, "flake.nix"), readRepoFile(t, "go.mod"), readRepoFile(t, "go.sum"))
	if bad != "" {
		t.Error(bad)
	}
	if stale == "" {
		return
	}
	if os.Getenv("GOCPP_REQUIRE_PACKAGING") != "" {
		t.Error(stale)
		return
	}
	t.Skip(stale + " (a failure under GOCPP_REQUIRE_PACKAGING=1)")
}

// TestVendorStamp: the decisions of TestFlakeVendorHash, with files of the test's own.
func TestVendorStamp(t *testing.T) {
	const hash = `vendorHash = "sha256-yQpnSIS+F8ANLTm1WdUe1Nc8me1LK98G0gjQQ8vaC6o=";`
	sum := sha256.Sum256([]byte("go.mod text" + "go.sum text"))
	stamp := "# go.mod+go.sum sha256: " + hex.EncodeToString(sum[:]) + "\n"
	for _, tc := range []struct {
		name, flake, gomod, gosum string
		stale, bad                bool
	}{
		{"current", stamp + hash, "go.mod text", "go.sum text", false, false},
		{"go.mod changed", stamp + hash, "go.mod text!", "go.sum text", true, false},
		{"go.sum changed", stamp + hash, "go.mod text", "go.sum text!", true, false},
		{"fake hash", stamp + `vendorHash = lib.fakeHash;`, "go.mod text", "go.sum text", false, true},
		{"no stamp", hash, "go.mod text", "go.sum text", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stale, bad := vendorStamp(tc.flake, tc.gomod, tc.gosum)
			if (stale != "") != tc.stale || (bad != "") != tc.bad {
				t.Errorf("vendorStamp = (%q, %q), want stale %v, bad %v", stale, bad, tc.stale, tc.bad)
			}
		})
	}
}

// TestFlakePackage checks what the plan promises of the Nix package: it is the default package
// and app for the systems that have release binaries, and gocpp is wrapped with the compilers.
func TestFlakePackage(t *testing.T) {
	flake := readRepoFile(t, "flake.nix")
	for _, want := range []string{
		"packages = forAllPackageSystems",
		"default = gocpp;",
		"apps = forAllPackageSystems",
		`"x86_64-linux"`, `"aarch64-linux"`,
		"wrapProgram $out/bin/gocpp --prefix PATH",
		"pkgs.gcc", "llvm.clang", "pkgs.binutils",
		"subPackages = [ \"cmd/gocpp\" ]",
		"doInstallCheck = true",
		"-X github.com/huuhait/gocpp/internal/version.Version=",
	} {
		if !strings.Contains(flake, want) {
			t.Errorf("flake.nix lacks %q", want)
		}
	}
	for _, want := range []string{
		"./THIRD_PARTY_NOTICES.txt\n",
		"install -Dm644 LICENSE $out/share/doc/gocpp/LICENSE",
		"install -Dm644 THIRD_PARTY_NOTICES.txt $out/share/doc/gocpp/THIRD_PARTY_NOTICES.txt",
	} {
		if !strings.Contains(flake, want) {
			t.Errorf("flake.nix: missing distribution notice input or install command %q", want)
		}
	}
	// Every file the Go program embeds is in the source of the package.
	for _, want := range []string{"./runtime/include", "./runtime/src", "./runtime/debug", "./runtime/files.go", "./stdlib", "./cmd", "./internal", "./go.sum"} {
		if !strings.Contains(flake, want+"\n") {
			t.Errorf("flake.nix: the source fileset lacks %s", want)
		}
	}
}
