package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompareGate(t *testing.T) {
	passing := []string{"a.go", "b.go", "c.go"}
	rs := &ResultSet{Results: []Result{
		{Path: "a.go", Outcome: Pass},
		{Path: "b.go", Outcome: Fail},
		{Path: "d.go", Outcome: Pass},
		{Path: "c.go", Outcome: Skipped},
	}}
	v := CompareGate(passing, rs)
	if strings.Join(v.Regressions, ",") != "b.go" {
		t.Errorf("regressions %v", v.Regressions)
	}
	if strings.Join(v.Missing, ",") != "c.go" {
		t.Errorf("missing %v", v.Missing)
	}
	if strings.Join(v.NewPasses, ",") != "d.go" {
		t.Errorf("new passes %v", v.NewPasses)
	}
}

func TestPassingFile(t *testing.T) {
	text := FormatPassing([]string{"b.go", "a.go", "ken/c.go"})
	got := ParsePassing(text)
	if strings.Join(got, ",") != "a.go,b.go,ken/c.go" {
		t.Errorf("%v", got)
	}
	if FormatPassing(got) != text {
		t.Error("not a fixed point")
	}
}

func TestSelection(t *testing.T) {
	ts := []Test{
		{Path: "a.go", Directive: "run", Status: "applicable"},
		{Path: "b.go", Directive: "errorcheck", Status: "applicable"},
		{Path: "ken/c.go", Directive: "run", Status: "applicable"},
		{Path: "ken/d.go", Directive: "run", Status: "skip:cgo"},
		{Path: "fixedbugs/e.go", Directive: "run", Status: "applicable"},
	}
	pick := func(s selection) string {
		out, err := s.pick(ts)
		if err != nil {
			t.Fatal(err)
		}
		var ps []string
		for _, x := range out {
			ps = append(ps, x.Path)
		}
		return strings.Join(ps, ",")
	}
	if got := pick(selection{}); got != "a.go,b.go,ken/c.go,fixedbugs/e.go" {
		t.Errorf("all applicable: %s", got)
	}
	if got := pick(selection{Dir: "."}); got != "a.go,b.go" {
		t.Errorf("top level: %s", got)
	}
	if got := pick(selection{Dir: "ken"}); got != "ken/c.go" {
		t.Errorf("ken: %s", got)
	}
	if got := pick(selection{Directive: "errorcheck"}); got != "b.go" {
		t.Errorf("directive: %s", got)
	}
	if got := pick(selection{Limit: 2}); got != "a.go,b.go" {
		t.Errorf("limit: %s", got)
	}
	// the shards partition the tests
	seen := map[string]int{}
	for i := 0; i < 3; i++ {
		for _, p := range strings.Split(pick(selection{Shard: strings.Join([]string{string(rune('0' + i)), "3"}, "/")}), ",") {
			if p != "" {
				seen[p]++
			}
		}
	}
	if len(seen) != 4 {
		t.Errorf("shards cover %v", seen)
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("%s is in %d shards", p, n)
		}
	}
	for _, bad := range []string{"3/3", "x", "-1/2", "1/0", "1"} {
		if _, err := (selection{Shard: bad}).pick(ts); err == nil {
			t.Errorf("shard %q accepted", bad)
		}
	}
}

// A shard applies with every kind of folder filter, also with -dir . (the
// first version let the top-level case skip the shard test).
func TestShardWithDirFilters(t *testing.T) {
	var ts []Test
	var shard0, shard1 []string
	for i := 0; len(shard0) < 2 || len(shard1) < 2; i++ {
		p := fmt.Sprintf("t%d.go", i)
		if inShard(p, 0, 2) && len(shard0) < 2 {
			shard0 = append(shard0, p)
		} else if !inShard(p, 0, 2) && len(shard1) < 2 {
			shard1 = append(shard1, p)
		} else {
			continue
		}
		ts = append(ts, Test{Path: p, Directive: "run", Status: "applicable"})
	}
	for _, c := range []struct {
		sel  selection
		want int
	}{{selection{Dir: ".", Shard: "0/2"}, 2}, {selection{Dir: ".", Shard: "1/2"}, 2}, {selection{Shard: "0/2"}, 2}, {selection{Shard: "1/2"}, 2}} {
		got, err := c.sel.pick(ts)
		if err != nil || len(got) != c.want {
			t.Errorf("%+v: %d tests (%v), want %d", c.sel, len(got), err, c.want)
		}
	}
	var sub []Test
	for _, x := range ts {
		sub = append(sub, Test{Path: "ken/" + x.Path, Directive: "run", Status: "applicable"})
	}
	n := 0
	for _, sh := range []string{"0/2", "1/2"} {
		got, _ := selection{Dir: "ken", Shard: sh}.pick(sub)
		n += len(got)
	}
	if n != len(sub) {
		t.Errorf("-dir ken in two shards runs %d of %d tests", n, len(sub))
	}
}
