package server

import (
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func waitNativeHover(t *testing.T, c *dclient, file, source, mark string) string {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(hangLimit)
	defer deadline.Stop()
	for {
		text, _ := c.hover(file, source, mark)
		if strings.Contains(text, "C/C++ native type") {
			return text
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("native hover not ready: %q; logs: %s", text, c.logs.String())
		}
	}
}

const nativeHoverHeader = `#include <cstdio>
#include <string>
#include <vector>
struct Widget { int field; };
using WidgetRef = Widget &;
template<class T> struct Box { T field; };
const std::vector<long>* readonly();
Widget &widgetReference();
std::string makeName();
FILE *openLog();
`

const nativeHoverSource = `package main
import "./api.hpp"

// Global is documented.
var Global api.Widget

func use() { _ = Global.field }

func f() {
 var typed api.Widget
 _ = typed.field
 constructed := api.Widget{}
 _ = constructed.field
 var box api.Box[float64]
 _ = box.field
 pointer := api.readonly()
 _ = pointer.size()
 name := api.makeName()
 _ = name.size()
 log := api.openLog()
 _ = log
 bound := api.widgetReference()
 _ = bound.field
 ordinary := 42
 _ = ordinary
}
func reference(value api.WidgetRef) { _ = value.field }
`

// startNativeHover starts a typed-interop server for the shared fixture above.
func startNativeHover(t *testing.T, init string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/native\n",
		"main.gp":   nativeHoverSource,
		"api.hpp":   nativeHoverHeader,
	}, init)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", nativeHoverSource)
	return c
}

// nativeSignature returns the signature section of a native type hover: the first line of a
// plain hover, or the content of the first fence of a Markdown one.
func nativeSignature(t *testing.T, text string, markdown bool) string {
	t.Helper()
	if markdown {
		body, ok := strings.CutPrefix(text, "```cpp\n")
		if !ok {
			t.Fatalf("signature is not a C++ fence: %q", text)
		}
		signature, _, ok := strings.Cut(body, "\n```")
		if !ok {
			t.Fatalf("signature fence is not closed: %q", text)
		}
		return signature
	}
	signature, _, _ := strings.Cut(text, "\n")
	return signature
}

func TestNativeTypeHoverSignature(t *testing.T) {
	c := startNativeHover(t, plainInit)
	for _, check := range []struct {
		name string
		mark string
		want string
	}{
		{"declared value", "var ty|ped", "Widget"},
		{"use of declared value", "_ = ty|ped.field", "Widget"},
		{"constructed value", "constr|ucted :=", "Widget"},
		{"use of constructed value", "_ = constr|ucted.field", "Widget"},
		{"template instance", "_ = bo|x.field", "Box<double>"},
		{"pointer to const template", "_ = poin|ter.size", "const std::vector<long> *"},
	} {
		t.Run(check.name, func(t *testing.T) {
			text := waitNativeHover(t, c, "main.gp", nativeHoverSource, check.mark)
			if got := nativeSignature(t, text, false); got != check.want {
				t.Fatalf("%s: signature %q, want %q (hover %q)", check.mark, got, check.want, text)
			}
		})
	}
}

func TestNativeTypeHoverCanonicalTypedefs(t *testing.T) {
	c := startNativeHover(t, plainInit)
	for _, check := range []struct {
		name string
		mark string
		want string
	}{
		{"std::string", "_ = na|me.size", "std::basic_string<char>"},
		{"FILE pointer", "_ = lo|g\n", "_IO_FILE *"},
	} {
		t.Run(check.name, func(t *testing.T) {
			text := waitNativeHover(t, c, "main.gp", nativeHoverSource, check.mark)
			if got := nativeSignature(t, text, false); got != check.want {
				t.Fatalf("%s: signature %q, want the canonical %q", check.mark, got, check.want)
			}
		})
	}
}

func TestNativeTypeHoverReferenceReturningCallBindsValueType(t *testing.T) {
	c := startNativeHover(t, plainInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = bou|nd.field")
	if got := nativeSignature(t, text, false); got != "Widget" {
		t.Fatalf("signature %q, want the value type Widget", got)
	}
}

func TestNativeTypeHoverKeepsDocComment(t *testing.T) {
	c := startNativeHover(t, plainInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = Glo|bal.field")
	if got := nativeSignature(t, text, false); got != "Widget" {
		t.Fatalf("signature %q, want Widget", got)
	}
	if !strings.Contains(text, "Global is documented.") {
		t.Fatalf("the doc comment of the variable was lost: %q", text)
	}
}

func TestNativeTypeHoverMarkdownFencesNativeTypeAsCpp(t *testing.T) {
	c := startNativeHover(t, markdownInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = bo|x.field")
	if got := nativeSignature(t, text, true); got != "Box<double>" {
		t.Fatalf("signature %q, want Box<double> in a C++ fence: %q", got, text)
	}
	if strings.Contains(text, "```go") {
		t.Fatalf("native type presented as Go: %q", text)
	}
}

func TestNativeTypeHoverPlainHasNoFence(t *testing.T) {
	c := startNativeHover(t, plainInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = bo|x.field")
	if strings.Contains(text, "```") {
		t.Fatalf("plain hover contains a code fence: %q", text)
	}
}

func TestNativeTypeHoverPartialMemberNote(t *testing.T) {
	c := startNativeHover(t, plainInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = poin|ter.size")
	if !strings.Contains(text, "Member completion is partial") {
		t.Fatalf("partial member status was lost: %q", text)
	}
}

func TestNativeTypeHoverCompleteMembersHaveNoPartialNote(t *testing.T) {
	c := startNativeHover(t, plainInit)
	text := waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = ty|ped.field")
	if strings.Contains(text, "Member completion is partial") {
		t.Fatalf("complete member status reported as partial: %q", text)
	}
}

func TestNativeTypeHoverUnsupportedReferenceAliasKeepsFallback(t *testing.T) {
	c := startNativeHover(t, plainInit)
	waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = ty|ped.field")
	text, _ := c.hover("main.gp", nativeHoverSource, "_ = val|ue.field")
	if strings.Contains(text, "C/C++ native type") {
		t.Fatalf("unsupported reference alias acquired speculative facts: %q", text)
	}
}

func TestNativeTypeHoverLeavesOrdinaryVariableUnchanged(t *testing.T) {
	c := startNativeHover(t, plainInit)
	waitNativeHover(t, c, "main.gp", nativeHoverSource, "_ = ty|ped.field")
	text, _ := c.hover("main.gp", nativeHoverSource, "_ = ord|inary")
	if !strings.Contains(text, "var ordinary int") || strings.Contains(text, "C/C++ native type") {
		t.Fatalf("ordinary hover changed: %q", text)
	}
}

func TestNativeTypeHoverBindingAndSnapshotProvenance(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	const header = "struct One { int first; }; struct Two { int second; };\n"
	const source = `package main
import "./api.hpp"
func f() {
 var x api.One
 _ = x.first
 { var x api.Two; _ = x.second }
}
`
	const sibling = "package main\nimport \"./api.hpp\"\nfunc g() { var x api.Two; _ = x.second }\n"
	const other = "package other\nimport \"../api.hpp\"\nfunc f() { var x api.Two; _ = x.second }\n"
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/native\n", "api.hpp": header,
		"main.gp": source, "sibling.gp": sibling, "other/main.gp": other,
	}, plainInit)
	defer c.wait()
	c.open("main.gp", source)
	c.open("sibling.gp", sibling)
	c.open("other/main.gp", other)
	for _, check := range []struct {
		file, src, mark, typ string
	}{
		{"main.gp", source, "_ = x|.first", "One"},
		{"main.gp", source, "_ = x|.second", "Two"},
		{"sibling.gp", sibling, "_ = x|.second", "Two"},
		{"other/main.gp", other, "_ = x|.second", "Two"},
	} {
		text := waitNativeHover(t, c, check.file, check.src, check.mark)
		if got := nativeSignature(t, text, false); got != check.typ {
			t.Fatalf("binding hover = %q, want %s", text, check.typ)
		}
	}
	changed := strings.Replace(source, "api.One", "api.Two", 1)
	c.change("main.gp", 2, changed)
	text := waitNativeHover(t, c, "main.gp", changed, "_ = x|.first")
	if got := nativeSignature(t, text, false); got != "Two" {
		t.Fatalf("source edit retained the old native type: %q", text)
	}
	broken := strings.Replace(changed, "func f() {", "func f() {\n missing := (", 1)
	c.change("main.gp", 3, broken)
	text, _ = c.hover("main.gp", broken, "_ = x|.first")
	if strings.Contains(text, "C/C++ native type") {
		t.Fatalf("native type crossed a repaired/stale view: %q", text)
	}
}
