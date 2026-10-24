package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func memberCompletions(t *testing.T, c *compClient, file, source, marker string) memberCompletionList {
	t.Helper()
	offset := strings.Index(source, marker)
	if offset < 0 {
		t.Fatalf("missing completion marker %q", marker)
	}
	offset += len(marker)
	params, err := json.Marshal(protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: c.uri(file)},
			Position:     protocol.NewMapper(source).Position(offset),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := c.call("textDocument/completion", string(params))
	var result memberCompletionList
	result.CompletionList = new(protocol.CompletionList)
	if response.Error != nil || json.Unmarshal(response.Result, &result) != nil {
		t.Fatalf("completion response = %+v", response)
	}
	return result
}

func waitMemberCompletions(t *testing.T, c *compClient, file, source, marker string) memberCompletionList {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(hangLimit)
	defer deadline.Stop()
	for {
		result := memberCompletions(t, c, file, source, marker)
		if result.Members != nil && result.Members.Ready {
			return result
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("member facts did not become ready; result=%+v logs=%s", result, c.logs.String())
		}
	}
}

func TestMemberCompletionFileSnapshotAndRepairProvenance(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	const header = "struct One { int first; }; struct Two { int second; };\n"
	const source = "package main\nimport \"./api.hpp\"\nfunc f() { var x api.One; _ = x.first }\n"
	const helper = "package main\nimport \"./api.hpp\"\nfunc g() { var x api.Two; _ = x.second }\n"
	const other = "package other\nimport \"../api.hpp\"\nfunc f() { var x api.Two; _ = x.second }\n"
	c := startComp(t, map[string]string{
		"gocpp.mod": "module example.com/provenance\n", "api.hpp": header,
		"main.gp": source, "helper.gp": helper, "other/main.gp": other,
	}, plainCompInit)
	defer c.wait()
	c.open("main.gp", source)
	c.open("helper.gp", helper)
	c.open("other/main.gp", other)
	first := waitMemberCompletions(t, c, "main.gp", source, "_ = x.")
	second := waitMemberCompletions(t, c, "other/main.gp", other, "_ = x.")
	sibling := waitMemberCompletions(t, c, "helper.gp", helper, "_ = x.")
	if !reflect.DeepEqual(labels(sibling.Items), []string{"second"}) {
		t.Fatalf("same-named sibling-file local = %v", labels(sibling.Items))
	}
	if !reflect.DeepEqual(labels(first.Items), []string{"first"}) ||
		!reflect.DeepEqual(labels(second.Items), []string{"second"}) {
		t.Fatalf("same-named cross-package locals = %v, %v", labels(first.Items), labels(second.Items))
	}
	changed := strings.Replace(source, "api.One", "api.Two", 1)
	c.change("main.gp", 2, changed)
	current := waitMemberCompletions(t, c, "main.gp", changed, "_ = x.")
	if !reflect.DeepEqual(labels(current.Items), []string{"second"}) {
		t.Fatalf("new snapshot reused old receiver: %v", labels(current.Items))
	}
	repaired := strings.Replace(changed, "x.first", "x. \n", 1)
	c.change("main.gp", 3, repaired)
	result := memberCompletions(t, c, "main.gp", repaired, "_ = x.")
	if item(result.Items, "first") != nil || item(result.Items, "second") != nil ||
		result.Members != nil && result.Members.Ready {
		t.Fatalf("original binding facts crossed into a repaired view: %+v", result)
	}
}

func TestMemberCompletionRealBindings(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	const header = `#include <vector>
struct Widget {
 int field;
 void method();
 void deleted() = delete;
 void unavailable() __attribute__((unavailable));
 private: int hidden;
};
struct Other { int other; };
const std::vector<long>* readonly();
`
	const source = `package main
import "./api.hpp"
type Alias = api.Widget
func main() {
 var typed api.Widget
 _ = typed.field
 constructed := api.Widget{}
 _ = constructed.field
 alias := constructed
 _ = alias.field
 var named Alias
 _ = named.field
 {
  var typed api.Other
  _ = typed.other
 }
 pointer := api.readonly()
 _ = pointer.size()
 ordinary := struct{ Plain int }{}
 _ = ordinary.Plain
 api.Widget{}
}
`
	c := startComp(t, map[string]string{
		"gocpp.mod": "module example.com/members\n",
		"main.gp":   source, "api.hpp": header,
	}, plainCompInit)
	defer c.wait()
	c.open("main.gp", source)
	for _, marker := range []string{"_ = typed.", "_ = constructed.", "_ = alias.", "_ = named."} {
		result := waitMemberCompletions(t, c, "main.gp", source, marker)
		if !reflect.DeepEqual(labels(result.Items), []string{"field", "method"}) || result.Members.Incomplete {
			t.Fatalf("%s members = %v, status = %+v", marker, labels(result.Items), result.Members)
		}
	}
	result := waitMemberCompletions(t, c, "main.gp", source, "var typed api.Other\n  _ = typed.")
	if !reflect.DeepEqual(labels(result.Items), []string{"other"}) {
		t.Fatalf("shadowed receiver = %v", labels(result.Items))
	}
	result = waitMemberCompletions(t, c, "main.gp", source, "_ = pointer.")
	if item(result.Items, "size") == nil || item(result.Items, "push_back") != nil || !result.Members.Incomplete {
		t.Fatalf("const vector = %v, status = %+v", labels(result.Items), result.Members)
	}
	plain := memberCompletions(t, c, "main.gp", source, "_ = ordinary.")
	if !reflect.DeepEqual(labels(plain.Items), []string{"Plain"}) || plain.Members != nil {
		t.Fatalf("ordinary completion changed: %+v", plain)
	}
}
