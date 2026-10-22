package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypedEmitAdaptations pins what the table of argument adaptations (ADR-130) writes for each of
// its rows, and that the arguments it leaves alone are written as they were.
func TestTypedEmitAdaptations(t *testing.T) {
	hdr, err := os.ReadFile(filepath.Join(typedDir, "adapt.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	const src = `package main

import "./adapt.hpp"

func main() {
	var big int64
	n := 1
	adapt.bump(&n)
	adapt.set_big(&big)
	var small int16
	adapt.read_int(&small)
	var f float32
	adapt.halve(&f)
	adapt.wide_length("abc")
	name := "x" + "y"
	adapt.wide_length(name)
	adapt.unit16_length(name)
	adapt.mode_value(2)
	adapt.level_value(200)
	var obj *adapt.Obj
	adapt.make_object(&obj)
	var down float64 = 3.5
	adapt.take_int(down)
	adapt.take_float(down)
	adapt.take_double(f)
	adapt.take_int(7)
	adapt.take_longlong(small)
	adapt.say("%s", name)
	adapt.say("plain")
}
`
	typedEnv(t)
	inDir(t, map[string]string{"adapt.hpp": string(hdr), "a.gp": src})
	needClang(t)
	t.Setenv("GOCPP_CXX", "g++")
	t.Setenv("GOCPP_INTEROP", "typed")
	r := gocppRun("", false, "emit", "a.gp")
	if r.code != 0 {
		t.Fatalf("typed emit: exit %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		`gp::cgo::call_out<int>(n.gp_get(), "n", [&](int* gp_out0) -> decltype(auto) { ` +
			`return ::bump(gp_out0); })`,
		`gp::cgo::call_out<long long>(big.gp_get(), "big", [&](long long* gp_out1) -> decltype(auto) { ` +
			`return ::set_big(gp_out1); })`,
		`::read_int(gp::cgo::scalar_in<int>(small.gp_get(), "small"))`,
		`::halve(gp::cgo::scalar_in<double>(f.gp_get(), "f"))`,
		`::wide_length(gp::wide_cstr<wchar_t>("abc").gp_ptr())`,
		`::wide_length(gp::wide_cstr<wchar_t>(name).gp_ptr())`,
		`::unit16_length(gp::wide_cstr<char16_t>(name).gp_ptr())`,
		`::mode_value(static_cast<::Mode>(2))`,
		`::level_value(static_cast<::Level>(200))`,
		`gp::cgo::call_void_out(obj, [&](void** gp_out2) -> decltype(auto) { ` +
			`return ::make_object(gp_out2); })`,
		`::take_int(gp::cgo::as<int>(down))`,
		`::take_float(gp::cgo::as<float>(down))`,
		`::take_double((*f))`,
		`::take_int(7)`,
		`::take_longlong((*small))`,
		`::say("%s", gp::cstr(name).gp_ptr())`,
		`::say("plain")`,
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("typed emit has no %q", want)
		}
	}
	if t.Failed() {
		t.Logf("emitted:\n%s", r.stdout)
	}
}
