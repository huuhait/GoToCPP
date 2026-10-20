package winssh

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The cmd.exe command lines that the runner sends to the machine, built here as plain text so that they
// are tested on any system (the machine's PowerShell is the only place they run).

// RunFolderRegexp matches the names of the run folders of this tool, <pid>_<unix time>, and nothing else:
// the cleanup of old folders never touches a folder of another tool.
const RunFolderRegexp = `^[0-9]+_[0-9]+$`

var runIDRx = regexp.MustCompile(RunFolderRegexp)

// KillRunLine ends every process of the run runID: the processes whose program file is below the run's
// folder (the suite's agent and the programs it started) and the ones whose command line names the run's
// folder followed by a character that cannot continue a run id (the cmd.exe that ssh starts for a
// command: "cd /d gocpp-wintest\<id> && ..."). Another run's processes never match (the id is unique),
// the PowerShell that looks is left out, and it prints "killed N".
func KillRunLine(runID string) string {
	if !runIDRx.MatchString(runID) {
		panic("winssh: bad run id " + runID)
	}
	return `powershell -NoProfile -Command "$id='` + runID + `'; ` +
		`$p = (Join-Path $env:USERPROFILE ('gocpp-wintest\' + $id)) + '\'; ` +
		`$k = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -ne 'powershell.exe' -and $_.ProcessId -ne $PID -and ` +
		`( ($_.ExecutablePath -and $_.ExecutablePath.StartsWith($p, [StringComparison]::OrdinalIgnoreCase)) -or ` +
		`($_.CommandLine -and $_.CommandLine -match ('gocpp-wintest\\' + $id + '(?![0-9A-Za-z_])')) ) }); ` +
		`$k | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }; ` +
		`'killed ' + $k.Count"`
}

// CleanStaleLine removes the run folders of this tool that are older than age: directories of
// %USERPROFILE%\gocpp-wintest whose whole name matches RunFolderRegexp, other than keep (this run's), whose
// last write is older than age (in whole hours, at least one). It touches nothing else and prints "cleaned".
func CleanStaleLine(keep string, age time.Duration) string {
	hours := int(age.Hours())
	if hours < 1 {
		hours = 1
	}
	if keep != "" && !runIDRx.MatchString(keep) {
		panic("winssh: bad run id " + keep)
	}
	return `powershell -NoProfile -Command "$d = Join-Path $env:USERPROFILE 'gocpp-wintest'; ` +
		`if (Test-Path -LiteralPath $d) { Get-ChildItem -Directory -LiteralPath $d | ` +
		`Where-Object { $_.Name -match '` + RunFolderRegexp + `' -and $_.Name -ne '` + keep + `' -and $_.LastWriteTime -lt (Get-Date).AddHours(-` + fmt.Sprint(hours) + `) } | ` +
		`Remove-Item -Recurse -Force -ErrorAction SilentlyContinue }; 'cleaned'"`
}

var plainName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// RemoveLine deletes the named files and folders of the run's folder (the line runs there). A name must be
// a plain file name: letters, digits, '.', '_' and '-', no separator.
func RemoveLine(names ...string) (string, error) {
	var parts []string
	for _, n := range names {
		if !plainName.MatchString(n) || strings.Contains(n, "..") {
			return "", fmt.Errorf("%q is not a plain name", n)
		}
		parts = append(parts, `rmdir /S /Q `+n+` 2>nul & del /Q /F `+n+` 2>nul`)
	}
	return strings.Join(parts, " & ") + ` & echo removed`, nil
}

// FreeBytesLine prints the free bytes of the drive that holds the user's profile.
func FreeBytesLine() string {
	return `powershell -NoProfile -Command "(Get-Item -LiteralPath $env:USERPROFILE).PSDrive.Free"`
}

var (
	stampMu   sync.Mutex
	lastStamp int64
)

// nextStamp is the time part of a run id: the Unix time, made larger than the last one this process used
// (two Remotes of one process in the same second must not share a folder).
func nextStamp() int64 {
	stampMu.Lock()
	defer stampMu.Unlock()
	t := time.Now().Unix()
	if t <= lastStamp {
		t = lastStamp + 1
	}
	lastStamp = t
	return t
}
