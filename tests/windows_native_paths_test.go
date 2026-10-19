package tests

import (
	"strings"
	"testing"
)

// The folders gocpp works in, with spaces and with characters outside the ANSI code page (Phase 14 step 14.2, ADR-081,
// the review's blocker B1 and finding S1). MinGW-w64's compilers read their command line in the ANSI code page (1252 on
// the PC) and split some paths at spaces; a user called "John Smith" or "日本" is ordinary on Windows. The cases:
//
//   - LOCALAPPDATA (the cache), TEMP (the work folders) and USERPROFILE with spaces, with a Japanese name, with both: a program,
//     a DLL (-o without an extension) with a C caller, and a program with a cxx recipe are built and run;
//   - the program's name from a folder with a Japanese name or an emoji (gocpp build . with no -o), an -o with such a name,
//     a DLL of that name, a package whose folder and whose C file have Japanese names, a cxx recipe below such a folder;
//   - a read-only output and a running program as the output.
//
// The characters are made in PowerShell from their code points, so that no layer between the test and the PC changes them.

const pathsSources = `package main

import "fmt"

func main() { fmt.Println("hello from windows") }
`

func TestWindowsNativePaths(t *testing.T) {
	n := nativeRemote(t)
	n.put(t, "src", map[string]string{
		"hello.gp": pathsSources,
		"dll.gp": `package main

//export Add
func Add(a, b int32) int32 { return a + b }

func main() {}
`,
		"caller.c": `#include <stdio.h>
#include "mylib.h"
int main(void) { printf("Add=%d\n", (int)Add(40, 2)); return 0; }
`,
		"proj/gocpp.mod": "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n\ncxx example.com/clib {\n\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=3\n}\n",
		"proj/main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/clib/include/clib.h\"\n)\n\nfunc main() {\n\tfmt.Println(clib.clib_scaled(14), clib.clib_square(7))\n}\n",
		"proj/clib/include/clib.h": `#pragma once
#ifdef __cplusplus
extern "C" {
#endif
int clib_scaled(int x);
int clib_square(int x);
#ifdef __cplusplus
}
#endif
`,
		"proj/clib/src/clib.c": `#include "clib.h"
int clib_scaled(int x) { return x * CLIB_SCALE; }
int clib_square(int x) { return x * x; }
`,
		"acp/main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"./probe.hpp\"\n)\n\nfunc main() { fmt.Println(\"acp\", int(probe.acp())) }\n",
		"acp/probe.hpp": "#pragma once\n#include <windows.h>\ninline int acp() { return (int)GetACP(); }\n",
		"pkg/gocpp.mod": "module example.com/pkg\n\ngo 1.25\n",
		"pkg/main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"./helper.h\"\n)\n\nfunc main() { fmt.Println(int(helper.twice(21))) }\n",
		"pkg/helper.h":  "#pragma once\nint twice(int);\n",
		"pkg/helper.c":  "#include \"helper.h\"\nint twice(int x) { return 2 * x; }\n",
		"sleeper.gp": `package main

import "time"

func main() { time.Sleep(60 * time.Second) }
`,
	})
	gp := n.abs(n.exe)
	src := n.abs("src")
	prelude := "$ErrorActionPreference = 'Continue'\r\n" +
		"$jp = [string][char]0x65E5 + [string][char]0x672C\r\n" +
		"$emoji = [char]::ConvertFromUtf32(0x1F600)\r\n" +
		"$gp = '" + gp + "'\r\n$src = '" + src + "'\r\n$run = '" + n.abs("") + "'\r\n$runTemp = '" + n.abs("tmp") + "'\r\n" +
		"function Run($what, [scriptblock]$b) {\r\n  $o = (& $b 2>&1 | Out-String)\r\n  $rc = $LASTEXITCODE\r\n  return @{ out = $o; rc = $rc }\r\n}\r\n" +
		"function Check($name, $ok, $detail) { if ($ok) { Write-Output \"OK $name\" } else { Write-Output \"FAIL $name :: $detail\" } }\r\n"

	// Folders of the user's profile with spaces and non-ANSI characters.
	t.Run("cache and temp", func(t *testing.T) {
		for _, c := range []string{"spaces", "japanese", "both"} {
			script := prelude + `
$case = '` + c + `'
$names = @{ spaces = @('lad sp ace', 'tmp sp ace', 'user sp ace'); japanese = @("lad_$jp", "tmp_$jp", "user_$jp"); both = @("lad sp $jp", "tmp sp $jp", "user $jp") }
$L = Join-Path $run $names[$case][0]; $T = Join-Path $run $names[$case][1]; $U = Join-Path $run $names[$case][2]
$work = Join-Path $run "out_$case"
New-Item -ItemType Directory -Force -Path $L, $T, $U, $work | Out-Null
$env:LOCALAPPDATA = $L; $env:TEMP = $T; $env:TMP = $T; $env:USERPROFILE = $U; $env:HOME = $U
$r = Run 'hello' { & $gp build -o "$work\hello" "$src\hello.gp" }
$o = if (Test-Path "$work\hello.exe") { (& "$work\hello.exe") -join '' } else { '' }
Check 'hello' (($r.rc -eq 0) -and ($o -eq 'hello from windows')) ($r.out + $o)
$r = Run 'dll' { & $gp build -buildmode=c-shared -o "$work\mylib" "$src\dll.gp" }
$have = (Test-Path "$work\mylib.dll") -and (Test-Path "$work\mylib.dll.a") -and (Test-Path "$work\mylib.lib") -and (Test-Path "$work\mylib.h")
Check 'dll files' (($r.rc -eq 0) -and $have) ($r.out)
$tmpSaved = $env:TEMP; $env:TEMP = $runTemp; $env:TMP = $runTemp  # the C compiler of the test is not gocpp: it reads TEMP in the ANSI code page
$r = Run 'caller' { gcc "$src\caller.c" "-I$work" "-L$work" -lmylib -o "$work\caller.exe" }
$env:TEMP = $tmpSaved; $env:TMP = $tmpSaved
Push-Location $work
$o = if (Test-Path caller.exe) { (& .\caller.exe) -join '' } else { '' }
Pop-Location
Check 'dll caller' (($r.rc -eq 0) -and ($o -eq 'Add=42')) ($r.out + $o)
Push-Location "$src\proj"
$r = Run 'recipe' { & $gp run . }
Pop-Location
Check 'recipe' (($r.rc -eq 0) -and ($r.out.Trim() -eq '42 49')) ($r.out)
Push-Location "$src\acp"
$r = Run 'acp' { & $gp run . }
Pop-Location
Check 'manifest linked' (($r.rc -eq 0) -and ($r.out.Trim() -eq 'acp 65001')) ($r.out)
$r = Run 'env' { & $gp env GOCPPCACHE }
Write-Output ("CACHE " + $r.out.Trim())
`
			res := n.powershell(t, script)
			out := normNL(res.Stdout)
			for _, want := range []string{"OK hello", "OK dll files", "OK dll caller", "OK recipe", "OK manifest linked"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s: no %q in the output (exit %d):\n%s\n%s", c, want, res.Code, out, res.Stderr)
				}
			}
		}
	})

	// Names of programs, libraries, packages and folders with characters outside the code page.
	t.Run("names", func(t *testing.T) {
		script := prelude + `
$env:LOCALAPPDATA = Join-Path $run 'lad'; $env:TEMP = Join-Path $run 'tmp'; $env:TMP = $env:TEMP
$work = Join-Path $run 'names'
New-Item -ItemType Directory -Force -Path $work | Out-Null
# the program's name comes from the folder (no -o)
foreach ($n in @("n_jp-$jp", "n_emoji-$emoji", ('n_e-' + [char]0xE9))) {
  $d = Join-Path $work $n
  New-Item -ItemType Directory -Force -Path $d | Out-Null
  Copy-Item "$src\hello.gp" "$d\main.gp"
  Push-Location $d
  $r = Run 'build' { & $gp build . }
  $exe = "$n.exe"
  $o = if (Test-Path -LiteralPath $exe) { (& ".\$exe") -join '' } else { '' }
  Pop-Location
  Check "folder name $($n.Substring(0, 4))" (($r.rc -eq 0) -and ($o -eq 'hello from windows')) ($r.out + $o)
}
# -o with a space and a Japanese name
$r = Run 'o' { & $gp build -o "$work\pr$([char]0xF6)g ram $jp" "$src\hello.gp" }
Check '-o name' (($r.rc -eq 0) -and (Test-Path -LiteralPath "$work\pr$([char]0xF6)g ram $jp.exe")) ($r.out)
# a DLL with a Japanese name, and its header
$r = Run 'dll' { & $gp build -buildmode=c-shared -o "$work\$jp" "$src\dll.gp" }
$have = (Test-Path -LiteralPath "$work\$jp.dll") -and (Test-Path -LiteralPath "$work\$jp.dll.a") -and (Test-Path -LiteralPath "$work\$jp.lib") -and (Test-Path -LiteralPath "$work\$jp.h")
Check 'dll name' (($r.rc -eq 0) -and $have) ($r.out)
# a package whose folder and C file have Japanese names, in a folder with a space
$d = Join-Path $work "$jp pkg"
Copy-Item -Recurse "$src\pkg" $d
Rename-Item -LiteralPath "$d\helper.c" "$jp.c"
Push-Location $d
$r = Run 'pkg' { & $gp run . }
Pop-Location
Check 'package folder and C file' (($r.rc -eq 0) -and ($r.out.Trim() -eq '42')) ($r.out)
# a cxx recipe below a folder with a Japanese name
$d = Join-Path $work "$jp lib"
Copy-Item -Recurse "$src\proj" $d
Push-Location $d
$r = Run 'recipe' { & $gp run . }
Pop-Location
Check 'recipe folder' (($r.rc -eq 0) -and ($r.out.Trim() -eq '42 49')) ($r.out)
`
		res := n.powershell(t, script)
		out := normNL(res.Stdout)
		for _, want := range []string{"OK folder name n_jp", "OK folder name n_em", "OK folder name n_e-", "OK -o name", "OK dll name", "OK package folder and C file", "OK recipe folder"} {
			if !strings.Contains(out, want) {
				t.Errorf("no %q in the output (exit %d):\n%s\n%s", want, res.Code, out, res.Stderr)
			}
		}
		if strings.Contains(out, "FAIL") {
			t.Logf("output:\n%s", out)
		}
	})

	// The program that gocpp run starts sees the user's own TEMP and LOCALAPPDATA (S-B), whatever gocpp made for its compilers. The Japanese
	// folders have short names; "tmp_Ł" (L with a stroke: not in code page 1252) is a legal 8.3 name already and has none, so it goes to
	// a private folder below ProgramData, whose ACL is read back with icacls (S-A).
	t.Run("program environment and private folder", func(t *testing.T) {
		n.put(t, "envsrc", map[string]string{"envprog.gp": `package main

import (
	"os"
	"strings"
)

func main() {
	s := strings.Join([]string{os.Getenv("TEMP"), os.Getenv("LOCALAPPDATA"), os.Getenv("TMP")}, "\n")
	os.WriteFile(os.Args[1], []byte(s), 0o666)
}
`})
		for _, c := range []string{"japanese", "stroke"} {
			script := prelude + `
$case = '` + c + `'
$stroke = [string][char]0x141
$names = @{ japanese = @("lad_$jp", "tmp_$jp"); stroke = @("lad_$stroke", "tmp_$stroke") }
$L = Join-Path $run $names[$case][0]; $T = Join-Path $run $names[$case][1]
$work = Join-Path $run "envout_$case"
New-Item -ItemType Directory -Force -Path $L, $T, $work | Out-Null
$env:LOCALAPPDATA = $L; $env:TEMP = $T; $env:TMP = $T
$utf8 = New-Object Text.UTF8Encoding $false
$top = $null
try {
  $r = Run 'build' { & $gp build -o "$work\envprog" "$run\envsrc\envprog.gp" }
  Check 'build' ($r.rc -eq 0) ($r.out)
  & "$work\envprog.exe" "$work\byhand.txt"
  $r = Run 'run' { & $gp run "$run\envsrc\envprog.gp" "$work\viarun.txt" }
  Check 'run' ($r.rc -eq 0) ($r.out)
  $hand = [IO.File]::ReadAllText("$work\byhand.txt", $utf8); $via = [IO.File]::ReadAllText("$work\viarun.txt", $utf8)
  Check 'same by hand and by gocpp run' ($hand -ceq $via) ("hand=[$hand] run=[$via]")
  Check 'the program sees the real TEMP' ($via -ceq "$T` + "`n" + `$L` + "`n" + `$T") ("got [$via]")
  $cache = (& $gp env GOCPPCACHE | Out-String).Trim()
  Write-Output "CACHE $cache"
  if ($cache -match 'gocpp-paths-') {
    $top = ($cache -split 'gocpp-paths-')[0] + 'gocpp-paths-' + ((($cache -split 'gocpp-paths-')[1]) -split '\\')[0]
    $acl = (icacls $top | Out-String)
    Write-Output ("TOP " + $top)
    Check 'private ACL' (($acl -notmatch 'BUILTIN\\Users') -and ($acl -notmatch 'Everyone') -and ($acl -match 'NT AUTHORITY\\SYSTEM')) $acl
  }
} finally {
  if ($top -and (Test-Path -LiteralPath $top)) { Remove-Item -LiteralPath $top -Recurse -Force -ErrorAction SilentlyContinue }
}
`
			res := n.powershell(t, script)
			out := normNL(res.Stdout)
			wants := []string{"OK build", "OK run", "OK same by hand and by gocpp run", "OK the program sees the real TEMP"}
			if c == "stroke" {
				wants = append(wants, "OK private ACL")
				if !strings.Contains(out, "gocpp-paths-S-1-5-") {
					t.Errorf("%s: the cache is not below ProgramData\\gocpp-paths-<SID>:\n%s", c, out)
				}
			}
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s: no %q in the output (exit %d):\n%s\n%s", c, want, res.Code, out, res.Stderr)
				}
			}
		}
	})

	// A private folder that is refused (somebody opened its ACL): the build stops at once and says which variable, which folder and why, instead of
	// a compile error about a file that the compiler cannot open and "this is a bug in gocpp". ProgramData is pointed at a folder of the run, so
	// that nothing is made in the real one.
	t.Run("refused private folder", func(t *testing.T) {
		script := prelude + `
$stroke = [string][char]0x141
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$pd = Join-Path $run 'pd_hostile'
$hostile = Join-Path $pd "gocpp-paths-$sid"
New-Item -ItemType Directory -Force -Path $hostile | Out-Null
icacls $hostile /grant '*S-1-1-0:(OI)(CI)F' | Out-Null
$L = Join-Path $run "lad_$stroke"; $T = Join-Path $run "tmp_$stroke"
New-Item -ItemType Directory -Force -Path $L, $T | Out-Null
$env:LOCALAPPDATA = $L; $env:TEMP = $T; $env:TMP = $T; $env:ProgramData = $pd
$r = Run 'build' { & $gp build -o "$run\refused" "$src\hello.gp" }
Write-Output ("RC " + $r.rc)
Write-Output $r.out
Check 'no program was built' (-not (Test-Path "$run\refused.exe")) ''
`
		res := n.powershell(t, script)
		out := normNL(res.Stdout)
		for _, want := range []string{"RC 1", "OK no program was built", "gocpp build: LOCALAPPDATA is ", "gives access", "Set LOCALAPPDATA to a folder"} {
			if !strings.Contains(out, want) {
				t.Errorf("no %q in the output (exit %d):\n%s\n%s", want, res.Code, out, res.Stderr)
			}
		}
		if strings.Contains(out, "bug in gocpp") {
			t.Errorf("the message says it is a bug in gocpp:\n%s", out)
		}
	})

	// A TEMP of 252 characters: CreateProcess takes a working folder of at most 247. (Much longer, from 260 characters, the TEMP is no path
	// for most of Windows PowerShell either: the script itself stops there.)
	t.Run("long temp", func(t *testing.T) {
		script := prelude + `
$long = Join-Path $run 'longtemp'
while ($long.Length -lt 220) { $long = Join-Path $long ('t' * 28) }
$long = Join-Path $long ('u' * (252 - $long.Length - 1))  # exactly 252 characters
$env:TEMP = $long; $env:TMP = $long
$env:LOCALAPPDATA = Join-Path $run 'lad'
New-Item -ItemType Directory -Force -Path ('\\?\' + $long) | Out-Null
try {
  $work = Join-Path $run 'longtemp_out'
  New-Item -ItemType Directory -Force -Path $work | Out-Null
  $p = Start-Process -FilePath $gp -ArgumentList 'build','-x','-o',"$work\hello","$src\hello.gp" -NoNewWindow -PassThru -RedirectStandardOutput "$work\o.txt" -RedirectStandardError "$work\e.txt"
  $done = $p.WaitForExit(120000)
  if (-not $done) { Stop-Process -Id $p.Id -Force }
  $o = if (Test-Path "$work\hello.exe") { (& "$work\hello.exe") -join '' } else { '' }
  $log = (Get-Content "$work\e.txt" -ErrorAction SilentlyContinue | Select-Object -Last 12 | ForEach-Object { $_.Substring(0, [Math]::Min(300, $_.Length)) }) -join [Environment]::NewLine
  Check 'build with a long TEMP' ($done -and ($o -eq 'hello from windows')) ("len=" + $long.Length + " done=$done " + $log)
} finally {
  Remove-Item -LiteralPath ('\\?\' + (Join-Path $run 'longtemp')) -Recurse -Force -ErrorAction SilentlyContinue
}
`
		res := n.powershell(t, script)
		if out := normNL(res.Stdout); !strings.Contains(out, "OK build with a long TEMP") {
			t.Errorf("a TEMP of 252 characters (exit %d):\n%s\n%s", res.Code, out, res.Stderr)
		}
	})

	// A DLL named without an extension gets .dll; .so is refused.
	t.Run("dll names", func(t *testing.T) {
		n.put(t, "dn", map[string]string{"dll.gp": "package main\n\n//export Add\nfunc Add(a, b int32) int32 { return a + b }\n\nfunc main() {}\n"})
		okExit(t, n.gocpp(t, "dn", nil, "build", "-buildmode=c-shared", "-o", "mylib", "dll.gp"), "gocpp build -buildmode=c-shared -o mylib")
		names := normNL(n.cmd(t, "dn", nil, `dir /b`).Stdout)
		for _, want := range []string{"mylib.dll\n", "mylib.dll.a\n", "mylib.h\n", "mylib.lib\n"} {
			if !strings.Contains(names, want) {
				t.Errorf("-o mylib left:\n%s", names)
			}
		}
		if strings.Contains(string(n.cmd(t, "dn", nil, `type mylib.h`).Stdout), "Link a program with: -l */") {
			t.Error("the header's link line is empty")
		}
		res := n.gocpp(t, "dn", nil, "build", "-buildmode=c-shared", "-o", "mylib2.so", "dll.gp")
		if e := normNL(res.Stderr); res.Code == 0 || !strings.Contains(e, "a shared library for windows is a .dll") {
			t.Errorf("-o mylib2.so: exit %d, stderr %q", res.Code, e)
		}
		// -o lib.dll: the hint is -llib, not -l
		okExit(t, n.gocpp(t, "dn", nil, "build", "-buildmode=c-shared", "-o", "lib.dll", "dll.gp"), "gocpp build -o lib.dll")
		if h := normNL(n.cmd(t, "dn", nil, `type lib.h`).Stdout); !strings.Contains(h, "Link a program with: -llib */") {
			t.Errorf("the header of lib.dll says:\n%s", h)
		}
	})

	// The output exists already: read-only, or a program that runs.
	t.Run("replacing the output", func(t *testing.T) {
		n.put(t, "repl", map[string]string{"hello.gp": pathsSources, "sleeper.gp": "package main\n\nimport \"time\"\n\nfunc main() { time.Sleep(60 * time.Second) }\n"})
		okExit(t, n.gocpp(t, "repl", nil, "build", "-o", "ro", "hello.gp"), "first build")
		okExit(t, n.cmd(t, "repl", nil, `attrib +r ro.exe`), "attrib")
		res := n.gocpp(t, "repl", nil, "build", "-o", "ro", "hello.gp")
		if res.Code != 0 {
			t.Errorf("replacing a read-only output: exit %d\n%s", res.Code, res.Stderr)
		}
		script := prelude + `
Set-Location (Join-Path $run 'repl')
$env:LOCALAPPDATA = Join-Path $run 'lad'; $env:TEMP = Join-Path $run 'tmp'; $env:TMP = $env:TEMP
$r = Run 'first' { & $gp build -o sleeper sleeper.gp }
$p = Start-Process -FilePath .\sleeper.exe -PassThru -WindowStyle Hidden
Start-Sleep -Milliseconds 700
$r = Run 'again' { & $gp build -o sleeper sleeper.gp }
Stop-Process -Id $p.Id -Force
$p.WaitForExit()
Write-Output ("RC " + $r.rc)
Write-Output $r.out
$r = Run 'after' { & $gp build -o sleeper sleeper.gp }
Write-Output ("AFTER " + $r.rc)
`
		res = n.powershell(t, script)
		out := normNL(res.Stdout)
		if !strings.Contains(out, "RC 1") || !strings.Contains(out, "running?") || !strings.Contains(out, "AFTER 0") {
			t.Errorf("replacing a running program:\n%s\n%s", out, res.Stderr)
		}
	})
}
