package winssh

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRunFolderRegexp(t *testing.T) {
	rx := regexp.MustCompile(RunFolderRegexp)
	for name, want := range map[string]bool{
		"99999991_1": true, "2161426_1792457085": true, "1_2": true,
		"99999991_2x": false, "x99999991_1": false, "99999991_1x": false, "99999991-1": false, "99999991_1_3": false,
		"99999991": false, "zzold dir_1": false, "_1": false, "99999991_": false, "keep.txt": false, "": false, "99999991_1\n": false,
	} {
		if got := rx.MatchString(name); got != want {
			t.Errorf("%q: %v, want %v", name, got, want)
		}
	}
}

func TestCleanStaleLine(t *testing.T) {
	line := CleanStaleLine("123_456", 30*time.Hour)
	for _, want := range []string{
		"powershell -NoProfile -Command",
		`Join-Path $env:USERPROFILE 'gocpp-wintest'`,
		"Get-ChildItem -Directory -LiteralPath $d", // directories only, never a file
		`$_.Name -match '^[0-9]+_[0-9]+$'`,         // the whole name, digits_digits
		`$_.Name -ne '123_456'`,                    // not this run's
		`$_.LastWriteTime -lt (Get-Date).AddHours(-30)`,
		"Remove-Item -Recurse -Force",
		"'cleaned'",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the line lacks %q:\n%s", want, line)
		}
	}
	if strings.ContainsAny(line, "%&<>") && strings.Contains(line, "&&") {
		t.Errorf("a cmd.exe operator in %s", line)
	}
	if !strings.Contains(CleanStaleLine("", time.Minute), "AddHours(-1)") {
		t.Error("an age under an hour must be one hour")
	}
	if !strings.Contains(CleanStaleLine("", 12*time.Hour), "AddHours(-12)") {
		t.Error("12 hours")
	}
}

func TestKillRunLine(t *testing.T) {
	line := KillRunLine("123_456")
	for _, want := range []string{
		`$id='123_456'`,
		`Join-Path $env:USERPROFILE ('gocpp-wintest\' + $id)`,
		"$_.ExecutablePath.StartsWith($p, [StringComparison]::OrdinalIgnoreCase)", // below the run's folder, with the separator
		`-match ('gocpp-wintest\\' + $id + '(?![0-9A-Za-z_])')`,                   // the exact id, not a longer one
		"$_.Name -ne 'powershell.exe'",
		"Stop-Process -Id $_.ProcessId -Force",
		"'killed ' + $k.Count",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the line lacks %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "winagent") || strings.Contains(line, "*") {
		t.Errorf("the line must not depend on a name or a wildcard: %s", line)
	}
	defer func() {
		if recover() == nil {
			t.Error("a bad run id must not reach a command line")
		}
	}()
	KillRunLine("1_2'; calc; '")
}

func TestRemoveLine(t *testing.T) {
	line, err := RemoveLine("b1.tgz", "b1", "b1-res.tgz")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"rmdir /S /Q b1 2>nul & del /Q /F b1 2>nul", "del /Q /F b1.tgz", "& echo removed"} {
		if !strings.Contains(line, want) {
			t.Errorf("lacks %q: %s", want, line)
		}
	}
	for _, bad := range []string{"", "..", "a/b", `a\b`, "a b", "a&b", "-x", "a..b", `"x`, "%x%", "../x"} {
		if _, err := RemoveLine("ok", bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestFreeBytesLine(t *testing.T) {
	if l := FreeBytesLine(); !strings.Contains(l, "PSDrive.Free") || !strings.Contains(l, "USERPROFILE") {
		t.Error(l)
	}
}

func TestNextStampIsUnique(t *testing.T) {
	seen := map[int64]bool{}
	prev := int64(0)
	for i := 0; i < 50; i++ {
		n := nextStamp()
		if seen[n] || n <= prev {
			t.Fatalf("stamp %d after %d", n, prev)
		}
		seen[n], prev = true, n
	}
}
