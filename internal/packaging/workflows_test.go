package packaging

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The workflow files (step 15.12) are checked as text: the module has no YAML package, and a
// new dependency would change the flake's vendorHash. These tests keep the files from drifting
// away from the things they name (Makefile targets, test lists, GoReleaser's archive names);
// they are not a replacement for actionlint, and nothing here runs a workflow.

// workflowFiles returns the names of the files in .github/workflows, sorted.
func workflowFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yml") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	return readRepoFile(t, ".github/workflows/"+name)
}

var commentLinePattern = regexp.MustCompile(`(?m)^[ \t]*#.*$`)

// withoutComments removes the comment lines of a workflow: the header comments talk about the
// things the tests forbid or count.
func withoutComments(text string) string {
	return commentLinePattern.ReplaceAllString(text, "")
}

// readWorkflowCode returns a workflow's text without its comment lines.
func readWorkflowCode(t *testing.T, name string) string {
	t.Helper()
	return withoutComments(readWorkflow(t, name))
}

var jobIDPattern = regexp.MustCompile(`(?m)^  ([A-Za-z0-9_-]+):[ \t]*$`)

// jobIDs returns the ids of the jobs of a workflow: the keys at two spaces of indentation below
// the top-level "jobs:" line.
func jobIDs(t *testing.T, text string) map[string]bool {
	t.Helper()
	_, jobs, ok := strings.Cut(text, "\njobs:\n")
	if !ok {
		t.Fatal("no top-level jobs: section")
	}
	ids := map[string]bool{}
	for _, match := range jobIDPattern.FindAllStringSubmatch(jobs, -1) {
		ids[match[1]] = true
	}
	return ids
}

// TestWorkflowsHaveTheTopLevelKeys: every workflow has a name, triggers, a permissions block that
// is read-only unless a job widens it, and jobs.
func TestWorkflowsHaveTheTopLevelKeys(t *testing.T) {
	for _, name := range workflowFiles(t) {
		text := readWorkflow(t, name)
		for _, key := range []string{"name:", "on:", "permissions:", "jobs:"} {
			if !regexp.MustCompile(`(?m)^` + key).MatchString(text) {
				t.Errorf("%s has no top-level %s", name, key)
			}
		}
		if strings.Contains(text, "\t") {
			t.Errorf("%s contains a tab: YAML does not allow tabs for indentation", name)
		}
		if !regexp.MustCompile(`(?m)^permissions:\n  contents: read\n`).MatchString(text) {
			t.Errorf("%s: the top-level permissions are not contents: read", name)
		}
	}
}

var usesPattern = regexp.MustCompile(`(?m)^\s*(?:- )?uses: (\S+)`)

// TestWorkflowActionsArePinnedAndLocalFilesExist: a "uses:" names a version tag (the nightly
// workflow does the same; commit SHAs are a later step), and a local reusable workflow exists.
func TestWorkflowActionsArePinnedAndLocalFilesExist(t *testing.T) {
	for _, name := range workflowFiles(t) {
		for _, match := range usesPattern.FindAllStringSubmatch(readWorkflow(t, name), -1) {
			ref := match[1]
			if strings.HasPrefix(ref, "./") {
				if _, err := os.Stat(filepath.Join(repoRoot(t), ref)); err != nil {
					t.Errorf("%s uses %s, which does not exist", name, ref)
				}
				continue
			}
			if !regexp.MustCompile(`@v[0-9]+$`).MatchString(ref) {
				t.Errorf("%s uses %s: want a major version tag, as @v4", name, ref)
			}
		}
	}
}

// TestWorkflowJobsHaveALimit: a job that runs on a runner has a timeout, so that a hung job does
// not use the default six hours.
func TestWorkflowJobsHaveALimit(t *testing.T) {
	for _, name := range workflowFiles(t) {
		text := readWorkflow(t, name)
		_, jobs, _ := strings.Cut(text, "\njobs:\n")
		// A job starts at its two-space key; a job that calls a workflow has no runner of its own.
		parts := jobIDPattern.Split(jobs, -1)
		ids := jobIDPattern.FindAllStringSubmatch(jobs, -1)
		for index, id := range ids {
			body := parts[index+1]
			if strings.Contains(body, "\n    uses: ") {
				continue
			}
			if !strings.Contains(body, "\n    timeout-minutes: ") {
				t.Errorf("%s: job %s has no timeout-minutes", name, id[1])
			}
		}
	}
}

// TestWorkflowNeedsNameJobsThatExist: a "needs:" list names jobs of the same file.
func TestWorkflowNeedsNameJobsThatExist(t *testing.T) {
	needs := regexp.MustCompile(`(?m)^    needs: \[?([A-Za-z0-9_, -]+?)\]?$`)
	for _, name := range workflowFiles(t) {
		text := readWorkflow(t, name)
		ids := jobIDs(t, text)
		for _, match := range needs.FindAllStringSubmatch(text, -1) {
			for _, need := range strings.Split(match[1], ",") {
				if !ids[strings.TrimSpace(need)] {
					t.Errorf("%s: needs: %q is not a job of the file", name, need)
				}
			}
		}
	}
}

// TestWorkflowMakeTargetsExist: every "make target" a workflow runs is a rule of the Makefile.
func TestWorkflowMakeTargetsExist(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	calls := regexp.MustCompile(`\bmake ([a-z][a-z0-9-]*)`)
	for _, name := range workflowFiles(t) {
		// Comment lines talk about make in prose; only the commands count.
		commands := readWorkflowCode(t, name)
		for _, match := range calls.FindAllStringSubmatch(commands, -1) {
			rule := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(match[1]) + `:`)
			if !rule.MatchString(makefile) {
				t.Errorf("%s runs make %s, which is not a Makefile target", name, match[1])
			}
		}
	}
}

// TestCIWorkflowCoversTheThreePlatforms: the exit criterion of Phase 15 names Linux x86-64, Linux
// arm64 and Windows x86-64, and the workflow can be called by the release workflow.
func TestCIWorkflowCoversTheThreePlatforms(t *testing.T) {
	code := readWorkflowCode(t, "ci.yml")
	for _, want := range []string{
		`runner:\s+ubuntu-24\.04\s`,
		`runner:\s+ubuntu-24\.04-arm\s`,
		`runs-on:\s+windows-2022\s`,
		`(?m)^\s+make ci$`,
		`(?m)^\s+workflow_call:`,
	} {
		if !regexp.MustCompile(want).MatchString(code) {
			t.Errorf("ci.yml has nothing that matches %q", want)
		}
	}
}

// TestCIWorkflowUsesNoWine: no CI leg tests with Wine (ADR-111: Windows is tested on the real PC,
// and Wine 11 cannot deliver stack overflows on AVX-512 CPUs). The Linux legs take Wine off the
// PATH and empty GOCPP_REQUIRE_WINE, so the Wine tests of the runtime skip; make ci would
// otherwise require them.
func TestCIWorkflowUsesNoWine(t *testing.T) {
	code := readWorkflowCode(t, "ci.yml")
	if !regexp.MustCompile(`GOCPP_REQUIRE_WINE: ""`).MatchString(code) {
		t.Error(`ci.yml must set GOCPP_REQUIRE_WINE: "" for make ci`)
	}
	if !strings.Contains(code, "grep -v -e '-wine'") {
		t.Error("ci.yml must take Wine off the PATH before make ci")
	}
	forbiddenWords := []string{
		"GOCPP_WINDOWS_WINE",
		"ci-windows",
		"test-windows",
		`GOCPP_REQUIRE_WINE: "1"`,
		"require-wine",
	}
	for _, forbidden := range forbiddenWords {
		if strings.Contains(code, forbidden) {
			t.Errorf("ci.yml contains %q: no CI leg may use Wine", forbidden)
		}
	}
}

// TestFlakeHasAnArm64DevShell: the arm64 job runs "nix develop", so the flake's devShells exist for
// aarch64-linux.
func TestFlakeHasAnArm64DevShell(t *testing.T) {
	flake := readRepoFile(t, "flake.nix")
	devShells := regexp.MustCompile(`(?s)\n      systems = \[(.*?)\];`).FindStringSubmatch(flake)
	if devShells == nil {
		t.Fatal("flake.nix has no systems list for the dev shell")
	}
	for _, system := range []string{`"x86_64-linux"`, `"aarch64-linux"`} {
		if !strings.Contains(devShells[1], system) {
			t.Errorf("flake.nix: the dev shell systems lack %s", system)
		}
	}
}

// TestCIWindowsJobRunsTheSampleList: the Windows job selects its tests from
// tests/windows_sample.txt, and every name in that list is a run test.
func TestCIWindowsJobRunsTheSampleList(t *testing.T) {
	text := readWorkflow(t, "ci.yml")
	if !strings.Contains(text, "tests/windows_sample.txt") {
		t.Error("ci.yml: the Windows job does not read tests/windows_sample.txt")
	}
	for _, line := range strings.Split(readRepoFile(t, "tests/windows_sample.txt"), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(name) {
			t.Errorf("tests/windows_sample.txt: %q is not a plain test name: "+
				"the job joins the names into a -run pattern", name)
		}
		if _, err := os.Stat(filepath.Join(repoRoot(t), "tests", "run", name+".gp")); err != nil {
			t.Errorf("tests/windows_sample.txt names %s, which is not in tests/run", name)
		}
	}
}

// TestReleaseWorkflowStartsOnlyOnTags: pushes of branches never start it, and its manual run is
// the snapshot that publishes nothing, in a job of its own.
func TestReleaseWorkflowStartsOnlyOnTags(t *testing.T) {
	code := readWorkflowCode(t, "release.yml")
	on, jobs, _ := strings.Cut(code, "\njobs:\n")
	if !regexp.MustCompile(`(?m)^\s+push:\s*\n\s+tags: \[`).MatchString(on) {
		t.Error("release.yml: push must be limited to tags")
	}
	for _, other := range []string{"branches", "pull_request", "schedule"} {
		if strings.Contains(on, other) {
			t.Errorf("release.yml: the triggers mention %s", other)
		}
	}
	snapshot := jobSection(t, jobs, "snapshot")
	if !strings.Contains(snapshot, "args: release --snapshot --clean --skip=publish") {
		t.Error("release.yml: the snapshot job must run goreleaser --snapshot --skip=publish")
	}
	if !strings.Contains(snapshot, "github.event_name != 'push'") {
		t.Error("release.yml: the snapshot job must run only when the event is not a tag push")
	}
	if strings.Contains(snapshot, "contents: write") || strings.Contains(snapshot, "GITHUB_TOKEN") {
		t.Error("release.yml: the snapshot job must not get a write token")
	}
}

// jobSection returns the text of one job of a workflow's jobs section (comments removed).
func jobSection(t *testing.T, jobs string, id string) string {
	t.Helper()
	parts := jobIDPattern.Split(jobs, -1)
	for index, match := range jobIDPattern.FindAllStringSubmatch(jobs, -1) {
		if match[1] == id {
			return parts[index+1]
		}
	}
	t.Fatalf("no job %s", id)
	return ""
}

// TestReleaseWorkflowOnlyCreatesTheDraftGoReleaserConfigures: GoReleaser makes the release (a
// draft, by .goreleaser.yaml), nothing in the workflow publishes, and only the draft job can
// write.
func TestReleaseWorkflowOnlyCreatesTheDraftGoReleaserConfigures(t *testing.T) {
	code := readWorkflowCode(t, "release.yml")
	if !strings.Contains(readRepoFile(t, ".goreleaser.yaml"), "draft: true\n") {
		t.Error(".goreleaser.yaml no longer creates a draft release")
	}
	_, jobs, _ := strings.Cut(code, "\njobs:\n")
	draft := jobSection(t, jobs, "draft")
	if !strings.Contains(draft, "args: release --clean") {
		t.Error("release.yml: the draft job does not run goreleaser release --clean")
	}
	if strings.Count(code, "contents: write") != 1 || !strings.Contains(draft, "contents: write") {
		t.Error("release.yml: contents: write must appear once, in the draft job")
	}
	publishing := []string{"gh release", "draft: false", "--draft=false", "npm publish"}
	for _, forbidden := range publishing {
		if strings.Contains(code, forbidden) {
			t.Errorf("release.yml contains %q: publishing is the owner's decision", forbidden)
		}
	}
	for _, match := range regexp.MustCompile(`secrets\.([A-Z_]+)`).FindAllStringSubmatch(code, -1) {
		if match[1] != "GITHUB_TOKEN" {
			t.Errorf("release.yml uses the secret %s: only GITHUB_TOKEN is expected", match[1])
		}
	}
	for _, job := range []string{"draft", "snapshot"} {
		section := jobSection(t, jobs, job)
		noCredentials := strings.Contains(section, "persist-credentials: false")
		noCache := strings.Contains(section, "cache: false")
		if !noCredentials || !noCache {
			t.Errorf("release.yml: job %s needs persist-credentials: false and cache: false", job)
		}
	}
}

// TestReleaseWorkflowVerifiesTheArchivesGoReleaserNames: the verify job's matrix and file names are
// the ones .goreleaser.yaml produces: linux amd64 and arm64 as tar.gz, windows amd64 as zip, and
// checksums.txt.
func TestReleaseWorkflowVerifiesTheArchivesGoReleaserNames(t *testing.T) {
	code := readWorkflowCode(t, "release.yml")
	config := readRepoFile(t, ".goreleaser.yaml")
	for _, want := range []string{
		"name_template: \"gocpp_{{ .Version }}_{{ .Os }}_{{ .Arch }}\"",
		"name_template: checksums.txt",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf(".goreleaser.yaml no longer has %q: update release.yml and this test", want)
		}
	}
	for _, want := range []string{
		"{os: linux, arch: amd64, runner: ubuntu-24.04, suffix: tar.gz}",
		"{os: linux, arch: arm64, runner: ubuntu-24.04-arm, suffix: tar.gz}",
		"{os: windows, arch: amd64, runner: windows-2022, suffix: zip}",
		"gocpp_*_${{ matrix.os }}_${{ matrix.arch }}.${{ matrix.suffix }}",
		"checksums.txt",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("release.yml lacks %q", want)
		}
	}
	if strings.Contains(code, "windows, arch: arm64") {
		t.Error("release.yml verifies windows/arm64, which .goreleaser.yaml does not build")
	}
}

// TestReleaseWorkflowUsesOneArtifactName: what the draft and snapshot jobs upload (one of them
// runs) is what the verify job downloads.
func TestReleaseWorkflowUsesOneArtifactName(t *testing.T) {
	code := readWorkflowCode(t, "release.yml")
	names := regexp.MustCompile(`(?m)^\s+name: gocpp-dist$`).FindAllString(code, -1)
	if len(names) != 3 {
		t.Errorf("release.yml: %d uses of gocpp-dist, want 2 uploads and 1 download", len(names))
	}
	uploads := strings.Count(code, "actions/upload-artifact@")
	downloads := strings.Count(code, "actions/download-artifact@")
	if uploads != 2 || downloads != 1 {
		t.Error("release.yml needs an upload in the draft and the snapshot job and one download")
	}
}

// TestReleaseWorkflowPreflightRunsTheWorkflowChecks: the release's preflight runs the same tests as
// make workflow-check.
func TestReleaseWorkflowPreflightRunsTheWorkflowChecks(t *testing.T) {
	pattern := regexp.MustCompile(`-run '([A-Za-z0-9|]+)'`)
	makefile := readRepoFile(t, "Makefile")
	rule := makefile[strings.Index(makefile, "workflow-check:\n"):]
	fromMakefile := pattern.FindStringSubmatch(rule)
	if fromMakefile == nil {
		t.Fatal("make workflow-check has no -run pattern")
	}
	preflight := readWorkflowCode(t, "release.yml")
	for _, name := range strings.Split(fromMakefile[1], "|") {
		if !strings.Contains(preflight, name) {
			t.Errorf("release.yml's preflight lacks %s, which make workflow-check runs", name)
		}
	}
	// A stale vendorHash must stop a tag: the stamp test fails, not skips, with this set on the step
	// that runs it (make ci skips it).
	if !strings.Contains(fromMakefile[1], "TestFlakeVendorHash") {
		t.Error("make workflow-check does not run TestFlakeVendorHash")
	}
	stepStart := strings.LastIndex(preflight[:strings.Index(preflight, "TestFlakeVendorHash")], "- name:")
	stepText := preflight[stepStart:strings.Index(preflight, "TestFlakeVendorHash")]
	if !strings.Contains(stepText, "GOCPP_REQUIRE_PACKAGING:") {
		t.Error("release.yml: the step that runs TestFlakeVendorHash must set GOCPP_REQUIRE_PACKAGING")
	}
}

var customShellPattern = regexp.MustCompile(`(?m)^\s+shell: (nix develop -c bash.*)$`)

// TestWorkflowCustomShellsStopOnErrors: a custom shell ("nix develop -c bash {0}") gets neither -e
// nor pipefail from GitHub (only the named shells bash and sh do), so a failing go vet is lost
// when the next command succeeds. actionlint does not shellcheck such a script either.
func TestWorkflowCustomShellsStopOnErrors(t *testing.T) {
	for _, name := range workflowFiles(t) {
		code := readWorkflowCode(t, name)
		for _, match := range customShellPattern.FindAllStringSubmatch(code, -1) {
			if !strings.Contains(match[1], "bash -eo pipefail {0}") {
				t.Errorf("%s: %q does not use bash -eo pipefail {0}", name, match[1])
			}
		}
	}
}

// TestWorkflowScriptsDoNotPipeIntoHead: `program | head -n 1` makes head close the pipe and the
// program die with SIGPIPE, which fails the step under pipefail (it did in the verify job, where
// gocpp version prints two lines). Capture the output and use sed -n 1p.
func TestWorkflowScriptsDoNotPipeIntoHead(t *testing.T) {
	for _, name := range workflowFiles(t) {
		if regexp.MustCompile(`\|\s*head\b`).MatchString(readWorkflowCode(t, name)) {
			t.Errorf("%s pipes into head: use sed -n 1p, or capture the output first", name)
		}
	}
}
