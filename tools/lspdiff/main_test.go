package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

type stubCaller struct {
	raw json.RawMessage
	err error
}

func (s stubCaller) CallRaw(context.Context, string, any) (json.RawMessage, error) {
	return s.raw, s.err
}

func TestCompareRepliesRejectsDrops(t *testing.T) {
	ok := stubCaller{raw: json.RawMessage("null")}
	drop := stubCaller{err: fmt.Errorf("stream: %w", io.EOF)}
	norm := func(b []byte) string { return string(b) }
	canceled := stubCaller{err: protocol.Errorf(protocol.RequestCancelled, "canceled")}
	serverCanceled := stubCaller{err: protocol.Errorf(protocol.ServerCancelled, "canceled")}
	modified := stubCaller{err: protocol.Errorf(protocol.ContentModified, "changed")}
	for _, tc := range []struct {
		name    string
		clients [2]rawCaller
		failed  [2]bool
	}{
		{"both drop", [2]rawCaller{drop, drop}, [2]bool{true, true}},
		{"left drops", [2]rawCaller{drop, ok}, [2]bool{true, false}},
		{"right drops", [2]rawCaller{ok, drop}, [2]bool{false, true}},
		{"missing result", [2]rawCaller{stubCaller{}, stubCaller{}}, [2]bool{true, true}},
		{"timeout", [2]rawCaller{stubCaller{err: context.DeadlineExceeded}, ok}, [2]bool{true, false}},
		{"canceled RPC", [2]rawCaller{canceled, ok}, [2]bool{true, false}},
		{"server canceled RPC", [2]rawCaller{ok, serverCanceled}, [2]bool{false, true}},
		{"content modified RPC", [2]rawCaller{modified, ok}, [2]bool{true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := compareReplies(
				context.Background(), tc.clients, "textDocument/hover", [2]any{}, norm)
			if err == nil {
				t.Fatal("dropped/failed requests could establish equivalence")
			}
			for i := range out {
				if out[i].failed != tc.failed[i] {
					t.Fatalf("side %d failed=%v, want %v", i, out[i].failed, tc.failed[i])
				}
			}
		})
	}
	t.Run("JSON null is an answer", func(t *testing.T) {
		out, err := compareReplies(
			context.Background(), [2]rawCaller{ok, ok}, "hover", [2]any{}, norm)
		if err != nil || out[0].value != "null" || out[1].value != "null" {
			t.Fatal("legitimate JSON null was treated as a missing reply")
		}
	})
	t.Run("explicit refusal is an answer", func(t *testing.T) {
		refusal := stubCaller{
			err: fmt.Errorf("rename: %w", protocol.Errorf(protocol.RequestFailed, "cannot rename")),
		}
		out, err := compareReplies(
			context.Background(), [2]rawCaller{refusal, refusal}, "rename", [2]any{}, norm)
		if err != nil || !out[0].refused || !out[1].refused || out[0].value != out[1].value {
			t.Fatal("explicit JSON-RPC refusals were not compared as answers")
		}
	})
}

func TestFormatDifference(t *testing.T) {
	prefix := strings.Repeat("x", 301)
	a, b := `{"items":["`+prefix+`a"]}`, `{"items":["`+prefix+`b"]}`
	full := formatDifference("completion", a, b, true)
	if want := "completion:\n    a: " + a + "\n    b: " + b; full != want {
		t.Fatalf("verbose evidence was changed or truncated: %q", full)
	}
	short := formatDifference("completion", a, b, false)
	if want := "completion:\n    a: " + a[:300] + "...\n    b: " + b[:300] + "..."; short != want {
		t.Fatalf("nonverbose summary: %q", short)
	}
	for _, full := range []bool{false, true} {
		if got := formatDifference("hover", "null", "{}", full); got != "hover:\n    a: null\n    b: {}" {
			t.Fatalf("short answers changed: %q", got)
		}
		boundary := strings.Repeat("x", 300)
		if got := formatDifference("boundary", boundary, "{}", full); strings.Contains(got, "...") {
			t.Fatal("truncated an answer at the summary boundary")
		}
	}
}
