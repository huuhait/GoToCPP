package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// budgetMillis is the budget of the time from didOpen to each kind of C/C++ data, cold.
const budgetMillis = 2000

// TestCppDataBudget is the budget row of the C/C++ editor data: every kind of data of every
// project is there in under 2 s from didOpen with an empty cache, with the compiler slots that
// a language server has by default (eight on a machine with eight cores or more).
// It needs clang and a quiet machine, so it runs only with GOCPP_CPPDATA_BUDGET=1 (like the
// speed budgets of the editor: it is not part of the ordinary tests).
func TestCppDataBudget(t *testing.T) {
	if os.Getenv("GOCPP_CPPDATA_BUDGET") == "" {
		t.Skip("set GOCPP_CPPDATA_BUDGET=1 on a quiet machine with clang to check the budget")
	}
	binary := filepath.Join(t.TempDir(), "gocpp")
	build := exec.Command("go", "build", "-o", binary, "github.com/huuhait/gocpp/cmd/gocpp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	work := t.TempDir()
	env := "GOCPP_INTEROP=typed"
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			caches := filepath.Join(work, sc.name+"-cache")
			kinds, err := measure(context.Background(), binary, env, work, caches, sc,
				30*time.Second, 5*time.Millisecond, false, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range kinds {
				if strings.HasPrefix(kind.name, "request latency") {
					continue
				}
				if kind.ms < 0 || kind.ms > budgetMillis {
					t.Errorf("%s: %s took %.0f ms (-1: never), budget %d ms", sc.name, kind.name,
						kind.ms, budgetMillis)
				}
			}
		})
	}
}
