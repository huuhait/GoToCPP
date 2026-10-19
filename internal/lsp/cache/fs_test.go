package cache

import "testing"

// TestReadSame: two reads are the same only when everything they gave is.
func TestReadSame(t *testing.T) {
	base := func() *read {
		return &read{kind: 'f', path: "/a/b.gp", exists: true, data: []byte("text"), names: []string{"x.gp"}, err: ""}
	}
	if !base().same(base()) {
		t.Fatal("equal reads differ")
	}
	for name, change := range map[string]func(*read){
		"kind":   func(r *read) { r.kind = 'd' },
		"path":   func(r *read) { r.path = "/a/c.gp" },
		"exists": func(r *read) { r.exists = false },
		"isDir":  func(r *read) { r.isDir = true },
		"data":   func(r *read) { r.data = []byte("tex!") },
		"names":  func(r *read) { r.names = []string{"y.gp"} },
		"more":   func(r *read) { r.names = append(r.names, "z.gp") },
		"err":    func(r *read) { r.err = "denied" },
	} {
		o := base()
		change(o)
		if base().same(o) || o.same(base()) {
			t.Errorf("reads that differ in %s are the same", name)
		}
	}
}
