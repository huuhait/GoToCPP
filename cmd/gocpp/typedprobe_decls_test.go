package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const diffHeader = `#pragma once
#include <cstddef>
namespace phys {
enum class Mode : unsigned char { Off, On = 3, Max = 200 };
enum Level { Low = -1, Mid, High = 5, Big = 1000000 };
struct Vec { double x, y; };
struct Body { Vec pos; long id; Mode mode; char tag[3]; };
inline long size_body() { return (long)sizeof(Body); }
inline long align_body() { return (long)alignof(Body); }
inline long size_vec() { return (long)sizeof(Vec); }
inline long size_mode() { return (long)sizeof(Mode); }
inline long size_level() { return (long)sizeof(Level); }
}
`

const diffProgram = `package main

import (
	"fmt"
	lib "./lib.hpp"
)

func main() {
	var b phys.Body
	_ = b
	var l phys.Level = phys.Mid
	var m phys.Mode = phys.Mode.On
	_, _ = l, m
	fmt.Println("levels", int(phys.Low), int(phys.Mid), int(phys.High), int(phys.Big))
	fmt.Println("modes", int(phys.Mode.Off), int(phys.Mode.On), int(phys.Mode.Max))
	fmt.Println("sizes", phys.size_body(), phys.align_body(), phys.size_vec(), phys.size_mode(), phys.size_level())
}
`

// The typed view and the opaque build agree: the program is compiled and run by
// the opaque pipeline (with the C++ compiler of the build, g++ and clang++), and what the
// real C++ prints (enumerator values, sizes) is what the extraction reported.
func TestTypedProbeDeclsAgreeWithBuild(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_MODCACHE", "off")
		t.Setenv("GOCPP_OFFLINE", "1")
		t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
		t.Setenv("XDG_CACHE_HOME", t.TempDir()) // never the real cache
		inDir(t, map[string]string{"main.gp": diffProgram, "lib.hpp": diffHeader})
		r := gocppRun("", false, "run", ".")
		if r.code != 0 {
			t.Fatalf("gocpp run: %+v", r)
		}
		var levels, modes, sizes []int64
		for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
			f := strings.Fields(line)
			target := &levels
			switch f[0] {
			case "modes":
				target = &modes
			case "sizes":
				target = &sizes
			}
			for _, s := range f[1:] {
				var v int64
				if _, err := fmt.Sscan(s, &v); err != nil {
					t.Fatalf("bad number in %q", line)
				}
				*target = append(*target, v)
			}
		}
		pr := gocppRun("", false, "__typed-probe", "-decls", ".")
		if pr.code != 0 {
			if pr.code == 3 {
				t.Skip("no clang++ for the probe")
			}
			t.Fatalf("__typed-probe: %+v", pr)
		}
		var rep struct {
			Decls struct {
				Records map[string]struct {
					Size, Align int64
					Complete    bool
				}
				Enums map[string]struct {
					Scoped     bool
					Underlying struct{ Text string }
					Size       int64
					Values     []struct{ Name, Value string }
				}
				Errors map[string]string
			}
			Refs []struct {
				Mapped string
			}
		}
		if err := json.Unmarshal([]byte(pr.stdout), &rep); err != nil {
			t.Fatalf("%v\n%s", err, pr.stdout)
		}
		d := rep.Decls
		get := func(name string) []string {
			var out []string
			for _, v := range d.Enums[name].Values {
				out = append(out, v.Name+"="+v.Value)
			}
			return out
		}
		want := func(names []string, vals []int64) string {
			var out []string
			for i, n := range names {
				out = append(out, fmt.Sprintf("%s=%d", n, vals[i]))
			}
			return strings.Join(out, " ")
		}
		if got, w := strings.Join(get("phys::Level"), " "), want([]string{"Low", "Mid", "High", "Big"}, levels); got != w {
			t.Errorf("Level: extracted %s, the build printed %s", got, w)
		}
		if got, w := strings.Join(get("phys::Mode"), " "), want([]string{"Off", "On", "Max"}, modes); got != w {
			t.Errorf("Mode: extracted %s, the build printed %s", got, w)
		}
		if len(sizes) != 5 {
			t.Fatalf("sizes %v", sizes)
		}
		body, vec := d.Records["phys::Body"], d.Records["phys::Vec"]
		if body.Size != sizes[0] || body.Align != sizes[1] || vec.Size != sizes[2] || d.Enums["phys::Mode"].Size != sizes[3] || d.Enums["phys::Level"].Size != sizes[4] {
			t.Errorf("sizes: extracted Body %d/%d Vec %d Mode %d Level %d, the build printed %v",
				body.Size, body.Align, vec.Size, d.Enums["phys::Mode"].Size, d.Enums["phys::Level"].Size, sizes)
		}
		if !d.Enums["phys::Mode"].Scoped || d.Enums["phys::Mode"].Underlying.Text != "unsigned char" || d.Enums["phys::Level"].Scoped {
			t.Errorf("enum flags: %+v", d.Enums)
		}
		for k, e := range d.Errors {
			// The candidates of a name in a scope (phys::Vec and Vec) are all asked; the wrong one fails.
			if !strings.Contains(e, "unknown type name") && !strings.Contains(e, "no type named") {
				t.Errorf("no answer for %s: %s", k, e)
			}
		}
		var mapped []string
		for _, ref := range rep.Refs {
			mapped = append(mapped, ref.Mapped)
		}
		if !strings.Contains(strings.Join(mapped, "\n"), "phys.Body") {
			t.Errorf("no result mapped to phys.Body: %v", mapped)
		}
	})
}
