package cppgen

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestForwarders calls the forwarding members of the structs of testdata/emit/embedding.gp from
// C++, as the thunks of the method tables (step 5.4) and generic code (Phase 07) do: u.Touch(),
// User::Touch(p), m.Get() with no knowledge of the embedding path. A const member forwarder for a
// method in the method set of T, a static one with the owning pointer for one that only *T has,
// through value fields, an embedded pointer, a named non-struct type and an anonymous struct.
// (gp::println writes to stderr, as Go's println does.) They must change the object that the pointer shares (the embedded field's address aliases the
// owner), copy the receiver where Go does, and panic for a nil embedded pointer.
func TestForwarders(t *testing.T) {
	const driver = `
static void gp_driver() {
    using namespace gpkg_main;
    gp::ptr<User> u = gp::new_<User>();
    User::Touch(u);                       // static: the address of u->Base shares u's owner
    User::Touch(u);
    gp::println(u->Get(), u->Base.ID);    // const forwarder: 2 2
    auto [k, s] = u->Pair(5);
    gp::println(k, s);                    // 7 p
    gp::println(User::Sum(u, gp::slice_of<int64_t>({1, 2, 3})));   // 6 + 2

    User copy = *u;                       // value embedding is a copy
    User::Touch(gp::new_<User>(copy));
    gp::println(copy.Base.ID);            // 2

    Mid m{gp::new_<Base>(Base{5}), 1};    // embedded pointer: pointer methods are in Mid's method set
    m.Touch();
    gp::println(m.Get(), m.Base->ID, m.Sum(gp::slice_of<int64_t>({1, 2})));    // 6 6 9
    Mid shared = m;
    shared.Touch();                       // the pointer is shared
    auto [k2, s2] = m.Pair(1);
    gp::println(m.Base->ID, k2, s2);      // 7 8 p

    Temp t{Celsius(100), "C"_gs};
    gp::println(int64_t(t.F()));          // 212

    Top top{Mid{gp::new_<Base>(Base{9}), 1}, 2};
    top.Touch();                          // const forwarder: through Mid, then the embedded pointer
    gp::println(top.Mid.Base->ID, top.Get(), top.Sum(gp::slice_of<int64_t>({1})));   // 10 10 11

    Deep deep{Mid2{gp::new_<Holder>(Holder{Base{20}})}};
    deep.Touch();                         // const forwarder: through the embedded *Holder, then &Holder::Base
    gp::println(deep.Mid2.Holder->Base.ID, deep.Get());    // 21 21

    Both b{};
    b.Mid.Base = gp::new_<Base>(Base{1});
    gp::println(b.Get());                 // Both's own Get: -1
    Mid zero{};
    bool panicked = false;
    try { zero.Get(); } catch (...) { panicked = true; }
    gp::println(panicked);
}
`
	dir, err := filepath.Abs(filepath.Join("testdata", "emit"))
	if err != nil {
		t.Fatal(err)
	}
	base := emitter(filepath.Join(dir, "embedding.gp"))
	emit := func(t *testing.T, cfg Config) string {
		cfg.AllForwarders = true
		cpp := base(t, cfg)
		cpp = strings.Replace(cpp, "int main(int argc, char** argv) {", driver+"\nint main(int argc, char** argv) {", 1)
		cpp = strings.Replace(cpp, "&gpkg_main::main", "&gp_driver", 1)
		return cpp
	}
	want := &result{stderr: "2 2\n7 p\n8\n2\n6 6 9\n7 8 p\n212\n10 10 11\n21 21\n-1\ntrue\n"}
	checkRuns(t, emit, want)
}
