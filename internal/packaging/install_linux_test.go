package packaging

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// run runs a command with a time limit, in a process group of its own that dies with the test.
func run(t *testing.T, dir string, env []string, name string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	err := cmd.Wait()
	return out.String(), err
}

// moduleZip writes the module zip that a Go module proxy makes of the tracked files of the
// repository, at version, into a proxy folder (file:// GOPROXY layout): the files of the working
// tree that git tracks, except those in a folder with its own go.mod (another module, such as
// spikes/) and in vendor folders, under github.com/huuhait/gocpp@version/.
func moduleZip(t *testing.T, proxy, version string) {
	t.Helper()
	root := repoRoot(t)
	files := trackedFiles(t)
	nested := map[string]bool{} // folders with their own go.mod
	for _, f := range files {
		if filepath.Base(f) == "go.mod" && filepath.Dir(f) != "." {
			nested[filepath.Dir(f)+"/"] = true
		}
	}
	const mod = "github.com/huuhait/gocpp"
	dir := filepath.Join(proxy, filepath.FromSlash(mod), "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	zf, err := os.Create(filepath.Join(dir, version+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
files:
	for _, f := range files {
		for n := range nested {
			if strings.HasPrefix(f, n) {
				continue files
			}
		}
		if strings.HasPrefix(f, "vendor/") || strings.Contains(f, "/vendor/") {
			continue
		}
		src, err := os.Open(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(mod + "@" + version + "/" + f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, src); err != nil {
			t.Fatal(err)
		}
		src.Close()
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		version + ".mod":  readRepoFile(t, "go.mod"),
		version + ".info": `{"Version":"` + version + `","Time":"2026-10-19T00:00:00Z"}`,
		"list":            version + "\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGoInstallOfATaggedVersion is go install github.com/huuhait/gocpp/cmd/gocpp@v1.2.3 without
// the network: the module zip of the repository is put in a file:// proxy, and the go command
// downloads it from there, unpacks it with the zip rules of the real proxy (file names, case
// collisions: the go command checks them), builds the binary, and the binary, run away from the
// source tree, reports the installed version, the embedded runtime, and builds and runs a program
// when a C++ compiler is there. The dependencies (golang.org/x/tools) come from the module cache
// of the machine, so the test needs them there (it skips if they are not).
func TestGoInstallOfATaggedVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the whole program")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command")
	}
	modcache, err := exec.Command(goBin, "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	downloads := filepath.Join(strings.TrimSpace(string(modcache)), "cache", "download")
	if _, err := os.Stat(filepath.Join(downloads, "golang.org", "x", "tools")); err != nil {
		t.Skipf("golang.org/x/tools is not in the module cache (%s): go install would need the network", downloads)
	}

	tmp := t.TempDir()
	// The go command makes its module cache read-only; the folder must be writable to be removed.
	t.Cleanup(func() {
		filepath.WalkDir(tmp, func(p string, d os.DirEntry, err error) error {
			if err == nil {
				os.Chmod(p, 0o777)
			}
			return nil
		})
	})
	proxy := filepath.Join(tmp, "proxy")
	const version = "v0.0.1-rc.1"
	moduleZip(t, proxy, version)

	gobin := filepath.Join(tmp, "bin")
	env := []string{
		"GOPROXY=file://" + proxy + ",file://" + downloads,
		"GOSUMDB=off", "GONOSUMDB=*", "GOFLAGS=", "GOTOOLCHAIN=local",
		"GOMODCACHE=" + filepath.Join(tmp, "modcache"),
		"GOBIN=" + gobin,
		"GOWORK=off",
	}
	if out, err := run(t, tmp, env, goBin, "install", "github.com/huuhait/gocpp/cmd/gocpp@"+version); err != nil {
		t.Fatalf("go install github.com/huuhait/gocpp/cmd/gocpp@%s: %v\n%s", version, err, out)
	}
	gocpp := filepath.Join(gobin, "gocpp")

	out, err := run(t, tmp, nil, gocpp, "version")
	if err != nil {
		t.Fatalf("gocpp version: %v\n%s", err, out)
	}
	first, rest, _ := strings.Cut(out, "\n")
	if !strings.HasPrefix(first, "gocpp "+version+" (phase ") {
		t.Errorf("gocpp version printed %q, want it to start with %q (the version go install recorded)", first, "gocpp "+version+" (phase ")
	}
	if !strings.HasPrefix(rest, "runtime ") || len(strings.TrimSpace(strings.TrimPrefix(rest, "runtime "))) != 16 {
		t.Errorf("gocpp version second line = %q, want runtime and 16 hex digits", rest)
	}

	// The program itself, away from the repository: the runtime and the standard library are
	// in the binary.
	if _, err := exec.LookPath("g++"); err != nil {
		if _, err := exec.LookPath("clang++"); err != nil {
			t.Skip("no C++ compiler: the install and the version are checked, not a build")
		}
	}
	hello := filepath.Join(tmp, "hello")
	if err := os.Mkdir(hello, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello, installed gocpp\") }\n"
	if err := os.WriteFile(filepath.Join(hello, "hello.gp"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = run(t, hello, []string{"XDG_CACHE_HOME=" + filepath.Join(tmp, "cache"), "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1"}, gocpp, "run", "hello.gp")
	if err != nil || out != "hello, installed gocpp\n" {
		t.Errorf("gocpp run hello.gp = %q, %v; want the greeting", out, err)
	}
}

// TestLdflagsSetTheVersion builds the program the way .goreleaser.yaml does (its -X flag, read
// from the file) and checks that gocpp version reports the version: the linker's -X does nothing
// to a variable that a function initializes, so a "clever" version variable would silently stay
// "dev" in every release.
func TestLdflagsSetTheVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the whole program")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command")
	}
	cfg := readRepoFile(t, ".goreleaser.yaml")
	var flag string
	for _, line := range strings.Split(cfg, "\n") {
		if strings.Contains(line, "-X github.com/huuhait/gocpp/internal/version.Version=") {
			flag = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		}
	}
	if flag == "" {
		t.Fatal(".goreleaser.yaml has no -X flag for internal/version.Version")
	}
	flag = strings.ReplaceAll(flag, "{{ .Version }}", "9.8.7")
	exe := filepath.Join(t.TempDir(), "gocpp")
	if out, err := run(t, repoRoot(t), []string{"CGO_ENABLED=0"}, goBin, "build", "-trimpath", "-ldflags", flag, "-o", exe, "./cmd/gocpp"); err != nil {
		t.Fatalf("go build with %q: %v\n%s", flag, err, out)
	}
	out, err := run(t, t.TempDir(), nil, exe, "version")
	if err != nil || !strings.HasPrefix(out, "gocpp v9.8.7 (phase ") {
		t.Errorf("gocpp version = %q, %v; want it to start with %q", out, err, "gocpp v9.8.7 (phase ")
	}
}
