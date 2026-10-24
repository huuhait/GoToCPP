package server

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func TestHeaderCompletionRealTinyHeader(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	defer probe.SetRacyWindow(0)()
	const header = `#define API_VALUE 42
struct Widget { private: int secret; };
int Func(int parameter);
double Func(double parameter);
constexpr int Const = 7;
namespace ns {
constexpr int Answer = 3;
namespace inner { struct Thing {}; }
}
namespace alias = ns;
namespace indirect = alias::inner;
namespace imported { using namespace ns; }
namespace narrowed { using namespace alias::inner; }
namespace forwarding { using namespace imported; }
namespace forwarded = forwarding::inner;
namespace forwarded_import { using namespace forwarding::inner; }
`
	const source = `package main
import "./api.hpp"
import "example.com/index/ordinary"
import renamed "::ns"
func main() {
 api.Func(1)
 _ = api.API_VALUE
 _ = ns.Answer
 _ = alias.Answer
 _ = imported.Answer
 _ = forwarding.Answer
 _ = renamed.Answer
 var thing ns.inner.Thing
 var aliasThing alias.inner.Thing
 var indirectThing indirect.Thing
 var importedThing narrowed.Thing
 var forwardedThing forwarded.Thing
 var forwardedImportThing forwarded_import.Thing
 _, _, _ = aliasThing, indirectThing, importedThing
 _, _ = forwardedThing, forwardedImportThing
 var widget api.Widget
 _ = api.Widget.secret
 _, _ = thing, widget
 _ = ordinary.Plain
}
`
	c := startComp(t, map[string]string{
		"gocpp.mod":            "module example.com/index\n",
		"main.gp":              source,
		"api.hpp":              header,
		"ordinary/ordinary.gp": "package ordinary\nconst Plain = 1\n",
	}, plainCompInit)
	defer c.wait()
	c.open("main.gp", source)
	request := func(after string) []protocol.CompletionItem {
		t.Helper()
		bodyStart := strings.Index(source, "func main()")
		offset := strings.Index(source[bodyStart:], after)
		if offset < 0 {
			t.Fatalf("no cursor marker %q", after)
		}
		offset += bodyStart + len(after)
		position := protocol.NewMapper(source).Position(offset)
		params, err := json.Marshal(protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: c.uri("main.gp")},
				Position:     position,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		response := c.call("textDocument/completion", string(params))
		var list protocol.CompletionList
		if response.Error != nil || json.Unmarshal(response.Result, &list) != nil {
			t.Fatalf("completion response = %+v", response)
		}
		return list.Items
	}
	// Wait for data, not an elapsed-time assertion: requests themselves never wait for clang.
	deadline := time.NewTimer(hangLimit)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var items []protocol.CompletionItem
ready:
	for {
		items = request("api.")
		if item(items, "Func") != nil {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("header index did not become ready; server logs: %s", c.logs.String())
			break ready
		}
	}
	want := []string{
		"API_VALUE", "Const", "Func", "Widget", "alias", "forwarded", "forwarded_import",
		"forwarding", "imported", "indirect", "narrowed", "ns",
	}
	if got := labels(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("header names = %v, want %v", got, want)
	}
	if macro := item(items, "API_VALUE"); macro.Kind != protocol.ConstantCompletion {
		t.Fatalf("macro item = %+v", macro)
	}
	if got := labels(request("ns.")); !reflect.DeepEqual(got, []string{"Answer", "inner"}) {
		t.Fatalf("namespace names = %v", got)
	}
	for _, scope := range []string{"alias.", "imported.", "renamed.", "forwarding."} {
		if got := labels(request(scope)); !reflect.DeepEqual(got, []string{"Answer", "inner"}) {
			t.Fatalf("visible namespace %s names = %v", scope, got)
		}
	}
	for _, scope := range []string{
		"ns.inner.", "alias.inner.", "indirect.", "narrowed.", "forwarded.", "forwarded_import.",
	} {
		if got := labels(request(scope)); !reflect.DeepEqual(got, []string{"Thing"}) {
			t.Fatalf("nested namespace %s names = %v", scope, got)
		}
	}
	if got := labels(request("ordinary.")); !reflect.DeepEqual(got, []string{"Plain"}) {
		t.Fatalf("foreign names leaked into ordinary package: %v", got)
	}
	if got := labels(request("api.Widget")); len(got) != 1 || got[0] != "Widget" {
		t.Fatalf("prefix filtering = %v", got)
	}
	if got := labels(request("api.Widget.")); len(got) != 0 {
		t.Fatalf("record/private members leaked from the name index: %v", got)
	}
}
