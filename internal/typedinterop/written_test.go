package typedinterop

import "testing"

// The written type keeps the typedef names that the canonical type loses
// (12.3: the type mapper names FILE, size_t, a typedef of an unnamed struct).
func TestWrittenType(t *testing.T) {
	hdr := `
#include <cstdio>
#include <cstddef>
#include <cstdint>
typedef struct { int w, h; } Rect;
typedef unsigned char Byte;
typedef struct Node Node;
struct Node { Node *next; };
Rect make_rect(int w, int h);
Byte first(const Byte *p);
Node *head(void);
const Node *chead(void);
size_t count(void);
int32_t id32(void);
extern Rect gRect;
`
	rep := analyze(t, `package main

import (
	"fmt"
	cstdio "<cstdio>"
	lib "./lib.hpp"
)

func main() {
	f := cstdio.fopen("x", "r")
	r := lib.make_rect(1, 2)
	b := lib.first(nil)
	h := lib.head()
	c := lib.chead()
	n := lib.count()
	i := lib.id32()
	g := lib.gRect
	fmt.Println(f, r, b, h, c, n, i, g)
}
`, map[string]string{"lib.hpp": hdr}, Options{})
	for site, want := range map[string][2]string{
		`C.fopen("x", "r")`: {"_IO_FILE *", "FILE *"},
		"C.make_rect(1, 2)": {"Rect", ""},
		"C.first(nil)":      {"unsigned char", "Byte"},
		"C.head()":          {"Node *", ""},
		"C.chead()":         {"const Node *", ""},
		"C.count()":         {"unsigned long", "size_t"},
		"C.id32()":          {"int", "int32_t"},
		"C.gRect":           {"Rect", ""},
	} {
		r := find(t, rep, site)
		if !r.OK {
			t.Errorf("%s: %s", site, r.Error)
			continue
		}
		if r.Base != want[0] && r.Type != want[0] {
			t.Errorf("%s: type %q (base %q), want %q", site, r.Type, r.Base, want[0])
		}
		if r.Written != want[1] {
			t.Errorf("%s: written %q, want %q", site, r.Written, want[1])
		}
	}
}
