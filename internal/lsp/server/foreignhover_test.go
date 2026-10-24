package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Hover and definition of the names of a header (step 12.13c2), against the real compiler: the
// answer waits for the background worker of the snapshot, and never blocks a request.

const foreignHoverHeader = `#include <string>
// Adds numbers.
int add(int a, int b);
int add(double a);
struct Window {
  // Pixel size.
  int size() const;
  int w;
};
Window open();
`

const foreignHoverSource = `package main
import "./api.hpp"
import "<string>"

func f() {
	_ = api.add(1, 2)
	var name std.string
	_ = name
	win := api.open()
	_ = win.size()
	_ = win.w
}

func shadow(api struct{ add int }) int { return api.add }

func rebind() {
	api := 3
	_ = api
}
`

func startForeignHover(t *testing.T, init string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/foreign\n",
		"main.gp":   foreignHoverSource,
		"api.hpp":   foreignHoverHeader,
	}, init)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", foreignHoverSource)
	return c
}

// waitHover polls the hover at mark until it contains want.
func waitHover(t *testing.T, c *dclient, source, mark, want string) string {
	t.Helper()
	deadline := time.Now().Add(hangLimit)
	for {
		text, _ := c.hover("main.gp", source, mark)
		if strings.Contains(text, want) {
			return text
		}
		if time.Now().After(deadline) {
			t.Fatalf("hover at %q never had %q: %q; logs: %s", mark, want, text, c.logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitDefinitions polls the definition at mark until it has an answer.
func waitDefinitions(t *testing.T, c *dclient, source, mark string) []string {
	t.Helper()
	deadline := time.Now().Add(hangLimit)
	for {
		if got := c.definitions("textDocument/definition", "main.gp", source, mark); len(got) > 0 {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("definition at %q never had an answer; logs: %s", mark, c.logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// locationsAt returns the locations of a definition request, as the protocol has them.
func (c *dclient) locationsAt(method, name, src, mark string) []protocol.Location {
	c.t.Helper()
	resp := c.call(method, c.posParams(name, src, mark))
	if resp.Error != nil {
		c.t.Fatalf("%s %q: %v", method, mark, resp.Error)
	}
	var locs []protocol.Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		c.t.Fatal(err)
	}
	return locs
}

func TestForeignHoverFunctionOverloads(t *testing.T) {
	c := startForeignHover(t, plainInit)
	text := waitHover(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)", "int add(double a)")
	for _, want := range []string{
		"int add(int a, int b)\nint add(double a)",
		"`api.add` has 2 declarations",
		"`int add(int a, int b)`\nAdds numbers.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hover lacks %q: %q", want, text)
		}
	}
	if strings.Contains(text, "```") {
		t.Errorf("a plain hover has a fence: %q", text)
	}
}

func TestForeignHoverMarkdownFencesCpp(t *testing.T) {
	c := startForeignHover(t, markdownInit)
	text := waitHover(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)", "int add(double a)")
	if !strings.HasPrefix(text, "```cpp\nint add(int a, int b)\nint add(double a)\n```") {
		t.Errorf("hover = %q, want the declarations in a C++ fence", text)
	}
	if strings.Contains(text, "```go") {
		t.Errorf("C++ shown as Go: %q", text)
	}
}

func TestForeignHoverNamespaceName(t *testing.T) {
	c := startForeignHover(t, plainInit)
	text := waitHover(t, c, foreignHoverSource, "var name std.st|ring", "std::basic_string<char>")
	if !strings.HasPrefix(text, "typedef std::basic_string<char> std::string;") ||
		!strings.Contains(text, "declared at") {
		t.Errorf("hover = %q", text)
	}
}

func TestForeignHoverMember(t *testing.T) {
	c := startForeignHover(t, plainInit)
	text := waitHover(t, c, foreignHoverSource, "_ = win.si|ze()", "int Window::size() const")
	if !strings.Contains(text, "Pixel size.") || !strings.Contains(text, "C/C++ method") {
		t.Errorf("hover = %q", text)
	}
	field := waitHover(t, c, foreignHoverSource, "_ = win.|w", "int Window::w")
	if !strings.Contains(field, "C/C++ field") {
		t.Errorf("hover = %q", field)
	}
}

func TestForeignDefinitionJumpsIntoTheHeader(t *testing.T) {
	c := startForeignHover(t, plainInit)
	t.Run("function overloads", func(t *testing.T) {
		got := waitDefinitions(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)")
		want := []string{"api.hpp:3:5-3:8 add", "api.hpp:4:5-4:8 add"}
		if !slices.Equal(got, want) {
			t.Errorf("definitions = %v, want %v", got, want)
		}
	})
	t.Run("member", func(t *testing.T) {
		got := waitDefinitions(t, c, foreignHoverSource, "_ = win.si|ze()")
		if want := []string{"api.hpp:7:7-7:11 size"}; !slices.Equal(got, want) {
			t.Errorf("definitions = %v, want %v", got, want)
		}
	})
	t.Run("a name of the standard library", func(t *testing.T) {
		got := waitDefinitions(t, c, foreignHoverSource, "var name std.st|ring")
		if len(got) != 1 || !strings.HasSuffix(got[0], " string") || strings.HasPrefix(got[0], "api.hpp") {
			t.Errorf("definitions = %v, want one place of the library's header", got)
		}
	})
}

func TestForeignDefinitionUsesTheFileURI(t *testing.T) {
	c := startForeignHover(t, plainInit)
	waitDefinitions(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)")
	locs := c.locationsAt("textDocument/definition", "main.gp", foreignHoverSource, "_ = api.ad|d(1, 2)")
	if len(locs) != 2 {
		t.Fatalf("locations = %v", locs)
	}
	for _, loc := range locs {
		if got, want := loc.URI.Path(), filepath.Join(c.root, "api.hpp"); got != want {
			t.Errorf("uri path = %q, want %q", got, want)
		}
		span := loc.Range.End.Character - loc.Range.Start.Character
		if loc.Range.Start.Line != loc.Range.End.Line || span != 3 {
			t.Errorf("range = %+v, want the three letters of add", loc.Range)
		}
	}
}

func TestForeignShadowedHeaderNameLeaksNoDeclaration(t *testing.T) {
	c := startForeignHover(t, plainInit)
	waitHover(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)", "int add(double a)") // the facts are ready
	text, _ := c.hover("main.gp", foreignHoverSource, "return api.ad|d }")
	if strings.Contains(text, "int add") || strings.Contains(text, "declared at") {
		t.Errorf("hover of a field of a shadowing parameter = %q", text)
	}
	got := c.definitions("textDocument/definition", "main.gp", foreignHoverSource, "return api.ad|d }")
	for _, place := range got {
		if strings.HasPrefix(place, "api.hpp") {
			t.Errorf("definition of a shadowed name went to the header: %v", got)
		}
	}
	rebound, _ := c.hover("main.gp", foreignHoverSource, "_ = ap|i\n}")
	if strings.Contains(rebound, "declared at") || strings.Contains(rebound, "C/C++") {
		t.Errorf("hover of a rebound name = %q", rebound)
	}
}

func TestForeignAnswersFollowTheEditsOfTheDocument(t *testing.T) {
	c := startForeignHover(t, plainInit)
	waitDefinitions(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)")
	edited := strings.Replace(foreignHoverSource, "api.add(1, 2)", "api.nosuch(1, 2)", 1)
	c.change("main.gp", 2, edited)
	got := c.definitions("textDocument/definition", "main.gp", edited, "_ = api.nos|uch(1, 2)")
	if len(got) != 0 {
		t.Errorf("a name that is not in the header went to %v", got)
	}
	text, _ := c.hover("main.gp", edited, "_ = api.nos|uch(1, 2)")
	if strings.Contains(text, "int add") {
		t.Errorf("hover of the new name has the old declaration: %q", text)
	}
}

func TestForeignAnswersFollowTheHeaders(t *testing.T) {
	c := startForeignHover(t, plainInit)
	before := waitDefinitions(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)")
	header := filepath.Join(c.root, "api.hpp")
	if err := os.WriteFile(header, []byte("\n\n"+foreignHoverHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles",
		fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.uri("api.hpp")))
	want := []string{"api.hpp:5:5-5:8 add", "api.hpp:6:5-6:8 add"}
	deadline := time.Now().Add(hangLimit)
	for {
		got := c.definitions("textDocument/definition", "main.gp", foreignHoverSource, "_ = api.ad|d(1, 2)")
		if slices.Equal(got, want) {
			return
		}
		if len(got) > 0 && !slices.Equal(got, want) && !slices.Equal(got, before) {
			t.Fatalf("definitions = %v, want %v or the old %v", got, want, before)
		}
		if time.Now().After(deadline) {
			t.Fatalf("definitions = %v, never %v", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestForeignUnreadyFactAnswersAsBefore(t *testing.T) {
	// Without the header facts (opaque mode has no worker) the hover and definition are those
	// of step 11.5 and the earlier 12 steps.
	t.Setenv("GOCPP_INTEROP", "opaque")
	c := startNav(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/foreign\n",
		"main.gp":   foreignHoverSource,
		"api.hpp":   foreignHoverHeader,
	}, plainInit)
	t.Cleanup(func() { c.wait() })
	c.open("main.gp", foreignHoverSource)
	text, _ := c.hover("main.gp", foreignHoverSource, "_ = api.ad|d(1, 2)")
	if !strings.Contains(text, "opaque to gocpp") || strings.Contains(text, "int add") {
		t.Errorf("hover = %q", text)
	}
	got := c.definitions("textDocument/definition", "main.gp", foreignHoverSource, "_ = api.ad|d(1, 2)")
	if len(got) != 0 {
		t.Errorf("definitions = %v, want none", got)
	}
}

func TestForeignRepairedViewAnswersAsBefore(t *testing.T) {
	c := startForeignHover(t, plainInit)
	waitDefinitions(t, c, foreignHoverSource, "_ = api.ad|d(1, 2)")
	broken := foreignHoverSource + "\nfunc broken( {\n"
	c.change("main.gp", 2, broken)
	text, _ := c.hover("main.gp", broken, "_ = api.ad|d(1, 2)")
	if strings.Contains(text, "int add") || strings.Contains(text, "declared at") {
		t.Errorf("hover in a repaired view = %q, want the answer of a name without a fact", text)
	}
	got := c.definitions("textDocument/definition", "main.gp", broken, "_ = api.ad|d(1, 2)")
	if len(got) != 0 {
		t.Errorf("definitions in a repaired view = %v", got)
	}
}
