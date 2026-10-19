package vet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// TestDeriveFindings: the vet analyzers run on the methods that #Derive writes (step 10.10), but a finding
// inside the generated code is dropped: the finding about the method's signature, which is at the
// #Derive line, says what is wrong with the type, and the lines of the generated body would repeat it.
func TestDeriveFindings(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": `package main

import "sync"

#Derive(Equal, Clone)
type S struct {
	mu   sync.Mutex
	List []int
	P    *S
}

#Derive(String)
type Level int

const A Level = 1

func main() {
	var s S
	_ = s.Equal(s.Clone())
	_ = A.String()
}
`}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := loadDir(t, root, load.Config{Phase10Syntax: true})
	rep := p.vet(t, "copylocks")
	var got []string
	for _, f := range rep.Findings {
		got = append(got, p.short(f.Pos)+" "+f.Message)
	}
	text := strings.Join(got, "\n")
	for _, w := range []string{"Equal passes lock by value", "Clone passes lock by value"} {
		if !strings.Contains(text, w) {
			t.Errorf("no finding %q in:\n%s", w, text)
		}
	}
	if strings.Contains(text, "assignment copies lock value") || strings.Contains(text, "call of s.P.Equal") {
		t.Errorf("a finding in the generated code:\n%s", text)
	}
}
