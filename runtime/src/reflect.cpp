// The native functions of package reflect (gocpp/reflect.hpp, Phase 09 step 9.3, ADR-035). Standard
// C++ only.
#include <gocpp/rt.hpp>

#include <bit>
#include <cmath>
#include <cstdlib>
#include <cstring>
#include <limits>
#include <memory>
#include <mutex>
#include <new>
#include <string>
#include <string_view>
#include <unordered_map>

namespace gp::reflect {
namespace {

// --- small helpers ---

inline const type_desc* D(std::uint64_t t) noexcept {
    return reinterpret_cast<const type_desc*>(static_cast<std::uintptr_t>(t));
}

inline std::uint64_t U(const type_desc* d) noexcept {
    return static_cast<std::uint64_t>(reinterpret_cast<std::uintptr_t>(d));
}

[[noreturn]] void fail(const std::string& text) {
    gp::panic(gp::string::gp_copy(text));
}

// text(s) is a gp::string over the NUL-terminated static text s (descriptor names live as long as
// the program).
gp::string text(const char* s) {
    if (s == nullptr) {
        return gp::string();
    }
    return gp::string(gp::static_bytes, s, static_cast<std::int64_t>(std::strlen(s)));
}

bool same_str(const char* a, const char* b) noexcept {
    if (a == nullptr || b == nullptr) {
        return a == b;
    }
    return std::strcmp(a, b) == 0;
}

const type_ops* ops_of_desc(const type_desc* d) noexcept {
    return d->gp_ops;
}

// The C++ type name, as the error texts name it.
std::string type_text(const type_desc* d) {
    if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr && d->gp_ops->gp_go_name != nullptr) {
        return d->gp_ops->gp_go_name;
    }
    return d->gp_name != nullptr ? d->gp_name : "?";
}

// unsupported: a write that a C/C++ value can't take through the Go layout of its Go kind (the
// object is a C++ one: std::string, an integer wider than 8 bytes, a class). Every native that
// writes through a descriptor checks it for kind Foreign first (ADR-035).
[[noreturn]] void unsupported(const type_desc* d, const char* what) {
    fail(std::string("reflect: ") + what + " on a C++ value of type " + type_text(d) +
         " is not supported");
}

[[noreturn]] void opaque(const type_desc* d) {
    fail("reflect: C++ type " + type_text(d) + " is opaque (it can't be made or assigned)");
}

// --- kinds ---

std::int64_t sized_kind(std::size_t size, bool is_signed, const char* go_name) noexcept {
    if (is_signed) {
        switch (size) {
        case 1:
            return static_cast<std::int64_t>(kind::Int8);
        case 2:
            return static_cast<std::int64_t>(kind::Int16);
        case 4:
            return static_cast<std::int64_t>(kind::Int32);
        default:
            return static_cast<std::int64_t>(
                go_name != nullptr && std::strcmp(go_name, "int") == 0 ? kind::Int : kind::Int64);
        }
    }
    switch (size) {
    case 1:
        return static_cast<std::int64_t>(kind::Uint8);
    case 2:
        return static_cast<std::int64_t>(kind::Uint16);
    case 4:
        return static_cast<std::int64_t>(kind::Uint32);
    default:
        return static_cast<std::int64_t>(
            go_name != nullptr && std::strcmp(go_name, "uint") == 0 ? kind::Uint : kind::Uint64);
    }
}

// kind_of is the Go kind of a descriptor: a C/C++ type has the kind of the Go type of its shape.
std::int64_t kind_of(const type_desc* d) noexcept {
    if (d == nullptr) {
        return 0;
    }
    if (d->gp_kind == kind::Opaque) {
        return 0;
    }
    if (d->gp_kind != kind::Foreign) {
        return static_cast<std::int64_t>(d->gp_kind);
    }
    const type_ops* o = d->gp_ops;
    if (o == nullptr) {
        return static_cast<std::int64_t>(kind::Struct);
    }
    switch (o->gp_fclass) {
    case fclass::sint:
    case fclass::enum_s:
        return sized_kind(d->gp_size, true, o->gp_go_name);
    case fclass::uint_:
    case fclass::enum_u:
        return sized_kind(d->gp_size, false, o->gp_go_name);
    case fclass::f32:
        return static_cast<std::int64_t>(kind::Float32);
    case fclass::f64:
        return static_cast<std::int64_t>(kind::Float64);
    case fclass::boolean:
        return static_cast<std::int64_t>(kind::Bool);
    case fclass::raw_ptr:
    case fclass::cptr:
    case fclass::null_ptr:
        return static_cast<std::int64_t>(kind::UnsafePointer);
    case fclass::cxx_string:
    case fclass::cxx_string_view:
        return static_cast<std::int64_t>(kind::String);
    case fclass::none:
        break;
    }
    return static_cast<std::int64_t>(kind::Struct);
}

bool signed_type(const type_desc* d) noexcept {
    std::int64_t k = kind_of(d);
    return k >= static_cast<std::int64_t>(kind::Int) && k <= static_cast<std::int64_t>(kind::Int64);
}

bool unsigned_type(const type_desc* d) noexcept {
    std::int64_t k = kind_of(d);
    return k >= static_cast<std::int64_t>(kind::Uint) &&
           k <= static_cast<std::int64_t>(kind::Uintptr);
}

bool float_type(const type_desc* d) noexcept {
    std::int64_t k = kind_of(d);
    return k == static_cast<std::int64_t>(kind::Float32) ||
           k == static_cast<std::int64_t>(kind::Float64);
}

bool complex_type(const type_desc* d) noexcept {
    std::int64_t k = kind_of(d);
    return k == static_cast<std::int64_t>(kind::Complex64) ||
           k == static_cast<std::int64_t>(kind::Complex128);
}

// --- storage: the lifetime of C++ objects of a type ---

struct lifetime {
    bool trivial = false;
    void (*ctor)(void*) = nullptr;
    void (*dtor)(void*) = nullptr;
    bool ok = false; // the object can be made
};

void construct_iface(void* p) {
    ::new (p) gp::iface();
}

void destroy_iface(void* p) noexcept {
    static_cast<gp::iface*>(p)->~iface();
}

lifetime lifetime_of(const type_desc* d) noexcept {
    lifetime l;
    if (d->gp_kind == kind::Interface) {
        // Every interface type is a 32-byte gp::iface (an interface struct has no data of its own).
        l.ctor = &construct_iface;
        l.dtor = &destroy_iface;
        l.ok = true;
        return l;
    }
    const type_ops* o = ops_of_desc(d);
    if (o == nullptr) {
        return l;
    }
    if (o->gp_trivial) {
        l.trivial = true;
        l.ok = true;
        return l;
    }
    l.ctor = o->gp_ctor;
    l.dtor = o->gp_dtor;
    l.ok = o->gp_ctor != nullptr;
    return l;
}

// array_deleter destroys n objects of one type and frees their block.
struct array_deleter {
    void (*dtor)(void*);
    std::size_t size;
    std::size_t align;
    std::int64_t n;
    void operator()(void* p) const noexcept {
        if (dtor != nullptr) {
            for (std::int64_t i = n - 1; i >= 0; i--) {
                dtor(static_cast<char*>(p) + static_cast<std::size_t>(i) * size);
            }
        }
        ::operator delete(p, std::align_val_t(align));
    }
};

// alloc_array makes a block of n zero values of the type d (at least one byte), owned by a
// shared_ptr that destroys them.
std::shared_ptr<void> alloc_array(const type_desc* d, std::int64_t n) {
    const lifetime l = lifetime_of(d);
    if (!l.ok) {
        opaque(d);
    }
    const std::size_t size = d->gp_size;
    const std::size_t align = d->gp_align == 0 ? 1 : d->gp_align;
    if (n < 0 || (size != 0 && static_cast<std::uint64_t>(n) >
                                   static_cast<std::uint64_t>(detail::max_alloc) / size)) {
        fail("reflect: array too large");
    }
    std::size_t bytes = static_cast<std::size_t>(n) * size;
    if (bytes == 0) {
        bytes = 1;
    }
    void* mem = ::operator new(bytes, std::align_val_t(align));
    if (l.trivial) {
        std::memset(mem, 0, bytes);
    } else {
        for (std::int64_t i = 0; i < n; i++) {
            l.ctor(static_cast<char*>(mem) + static_cast<std::size_t>(i) * size);
        }
    }
    return std::shared_ptr<void>(mem, array_deleter{l.dtor, size, align, n});
}

vptr as_vptr(const std::shared_ptr<void>& block) {
    return vptr(block, block.get());
}

// new_storage makes the storage of one zero value of type d.
vptr new_storage(const type_desc* d) {
    return as_vptr(alloc_array(d, 1));
}

// assign does *dst = *src for two objects of type d.
void assign(const type_desc* d, void* dst, const void* src) {
    if (dst == src) {
        return;
    }
    if (d->gp_kind == kind::Interface) {
        *static_cast<gp::iface*>(dst) = *static_cast<const gp::iface*>(src);
        return;
    }
    const type_ops* o = ops_of_desc(d);
    if (o != nullptr && o->gp_trivial) {
        std::memmove(dst, src, d->gp_size);
        return;
    }
    if (d->gp_store == nullptr || o == nullptr) {
        opaque(d);
    }
    std::shared_ptr<void> data;
    if (o->gp_share) {
        // A *T: store reads the pointer as its owner and its target.
        data = std::shared_ptr<void>(o->gp_owner(src), const_cast<void*>(o->gp_target(src)));
    } else {
        data = std::shared_ptr<void>(std::shared_ptr<void>(), const_cast<void*>(src));
    }
    d->gp_store(dst, data);
}

// --- interface values ---

// Go's shared boxes. Go's conversion to an interface does not allocate for some values: a bool or
// a one-byte integer, and a value of 2, 4 or 8 bytes (aligned to its size, without pointers) below
// 256, point into runtime.staticuint64s; "" and a slice with a nil array point to runtime.zeroVal;
// a zero-size value to runtime.zerobase (cmd/compile/internal/walk/convert.go: dataWord;
// runtime/iface.go: convT16, convT32, convT64, convTstring, convTslice). So two reflect.Values of
// such values have the same pointer field, and DeepEqual(ValueOf(1), ValueOf(1)) is true: unpack
// gives a Value of such a value the same storage (ADR-035 item 10e). These boxes are never written:
// a Value that came out of an interface is not addressable.
struct shared_boxes {
    alignas(16) std::uint64_t small[256];
    gp::string empty;
    gp::slice<std::uint8_t> nil_slice; // (every gp::slice<E> has this layout: slice_of)
    alignas(16) unsigned char zero[64];
    std::shared_ptr<void> owner; // never freed (reachable from the static pointer: no leak report)
};

static_assert(std::endian::native == std::endian::little,
              "a small box is read in its first bytes, as Go does on a little-endian machine");

const shared_boxes& boxes() {
    static const shared_boxes* const b = [] {
        auto* s = new shared_boxes();
        for (std::uint64_t i = 0; i < 256; i++) {
            s->small[i] = i;
        }
        s->owner = std::shared_ptr<void>(static_cast<void*>(s), [](void*) {});
        return s;
    }();
    return *b;
}

// go_zero_size: a struct or array type whose Go size is 0 (struct{}, [0]T, a struct of them; the
// C++ object of struct{} has a size of 1).
bool go_zero_size(const type_desc* d) {
    const type_desc* f = full(d);
    switch (d->gp_kind) {
    case kind::Array:
        return f->gp_len == 0 || (f->gp_elem != nullptr && go_zero_size(f->gp_elem));
    case kind::Struct:
        if (f->gp_fields == nullptr && f->gp_nfields != 0) {
            return false;
        }
        for (std::size_t i = 0; i < f->gp_nfields; i++) {
            if (!go_zero_size(f->gp_fields[i].gp_type)) {
                return false;
            }
        }
        return true;
    default:
        return false;
    }
}

// go_plain: a type whose values hold no pointer in Go (a bool, a number, an array or a struct of
// them).
bool go_plain(const type_desc* d) {
    switch (d->gp_kind) {
    case kind::Bool:
    case kind::Int:
    case kind::Int8:
    case kind::Int16:
    case kind::Int32:
    case kind::Int64:
    case kind::Uint:
    case kind::Uint8:
    case kind::Uint16:
    case kind::Uint32:
    case kind::Uint64:
    case kind::Uintptr:
    case kind::Float32:
    case kind::Float64:
    case kind::Complex64:
    case kind::Complex128:
        return true;
    case kind::Array: {
        const type_desc* f = full(d);
        return f->gp_elem != nullptr && go_plain(f->gp_elem);
    }
    case kind::Struct: {
        const type_desc* f = full(d);
        if (f->gp_fields == nullptr && f->gp_nfields != 0) {
            return false;
        }
        for (std::size_t i = 0; i < f->gp_nfields; i++) {
            if (!go_plain(f->gp_fields[i].gp_type)) {
                return false;
            }
        }
        return true;
    }
    default:
        return false; // (a pointer, a string, ...; a C/C++ type: Go has none)
    }
}

// sole_part: the type of the one part of a struct of one field or an array of one element, after
// all such levels (cmd/compile's Type.SoleComponent; d itself for any other type). Its value is at
// offset 0.
const type_desc* sole_part(const type_desc* d) {
    for (;;) {
        const type_desc* f = full(d);
        if (d->gp_kind == kind::Struct && f->gp_nfields == 1 && f->gp_fields != nullptr) {
            d = f->gp_fields[0].gp_type;
        } else if (d->gp_kind == kind::Array && f->gp_len == 1 && f->gp_elem != nullptr) {
            d = f->gp_elem;
        } else {
            return d;
        }
    }
}

// shared_box: Go's shared box for the value at p of type d, or null when Go allocates one.
const void* shared_box(const type_desc* d, const void* p) {
    const type_ops* o = ops_of_desc(d);
    const shared_boxes& b = boxes();
    if (d->gp_kind == kind::Struct || d->gp_kind == kind::Array) {
        // A struct or an array whose one part is a string or a slice: Go's convTstring and
        // convTslice ("" and a nil array share runtime.zeroVal), with the box of that part.
        const type_desc* s = sole_part(d);
        if (s != d && (s->gp_kind == kind::String || s->gp_kind == kind::Slice)) {
            const void* box = shared_box(s, p);
            return box != nullptr && d->gp_size == s->gp_size ? box : nullptr;
        }
    }
    switch (d->gp_kind) {
    case kind::String:
        return gp::len(*static_cast<const gp::string*>(p)) == 0 ? &b.empty : nullptr;
    case kind::Slice:
        return o != nullptr && o->gp_nil != nullptr && o->gp_nil(p) ? &b.nil_slice : nullptr;
    case kind::Bool:
    case kind::Int8:
    case kind::Uint8:
        return &b.small[*static_cast<const std::uint8_t*>(p)];
    case kind::Int16:
    case kind::Uint16:
    case kind::Int32:
    case kind::Uint32:
    case kind::Float32:
    case kind::Int:
    case kind::Int64:
    case kind::Uint:
    case kind::Uint64:
    case kind::Uintptr:
    case kind::Float64:
        break;
    case kind::Struct:
    case kind::Array:
        if (go_zero_size(d)) {
            return d->gp_size <= sizeof(b.zero) ? b.zero : nullptr;
        }
        if (!go_plain(d)) {
            return nullptr;
        }
        break;
    default:
        return nullptr; // (complex64 is 4-aligned: Go allocates; Foreign values are C++ objects)
    }
    std::size_t n = d->gp_size;
    if ((n != 2 && n != 4 && n != 8) || d->gp_align != n) {
        return nullptr;
    }
    std::uint64_t v = 0;
    std::memcpy(&v, p, n);
    return v < 256 ? &b.small[v] : nullptr;
}

// unpack: the dynamic type of x and the storage of its value.
std::tuple<std::uint64_t, vptr> unpack(const gp::iface& x) {
    const type_desc* d = x.gp_type();
    if (d == nullptr) {
        return {0, vptr()};
    }
    const type_ops* o = ops_of_desc(d);
    if (o != nullptr && o->gp_share) {
        // An interface that holds a *T shares the pointer's owner: the storage is a new pointer.
        vptr s = new_storage(d);
        d->gp_store(s.gp_get(), x.gp_data());
        return {U(d), s};
    }
    const std::shared_ptr<void>& data = x.gp_data();
    if (data == nullptr) {
        // (a value with no box: nothing in the runtime makes one, but be safe)
        vptr s = new_storage(d);
        return {U(d), s};
    }
    if (const void* box = shared_box(d, data.get())) {
        return {U(d), vptr(boxes().owner, const_cast<void*>(box))};
    }
    return {U(d), vptr(data, data.get())};
}

// pack is an interface{} that holds a copy of the value at p (type d).
gp::any pack(const type_desc* d, const void* p) {
    if (d->gp_kind == kind::Interface) {
        return gp::any(*static_cast<const gp::iface*>(p));
    }
    const type_ops* o = ops_of_desc(d);
    if (o != nullptr && o->gp_share) {
        auto data = std::shared_ptr<void>(o->gp_owner(p), const_cast<void*>(o->gp_target(p)));
        return gp::any(std::move(data), d, nullptr);
    }
    vptr box = new_storage(d);
    assign(d, box.gp_get(), p);
    return gp::any(box.gp_shared(), d, nullptr);
}

// --- type relations ---

bool same_fields(const field_desc& x, const field_desc& y, bool tags) noexcept {
    return same_str(x.gp_name, y.gp_name) && same_str(x.gp_pkg, y.gp_pkg) &&
           x.gp_type == y.gp_type && (!tags || same_str(x.gp_tag, y.gp_tag)) &&
           x.gp_embedded == y.gp_embedded;
}

// same_underlying: the types have identical underlying types (struct tags compared when tags).
bool same_underlying(const type_desc* a, const type_desc* b, bool tags) noexcept {
    if (a == b) {
        return true;
    }
    const type_desc* x = full(a);
    const type_desc* y = full(b);
    if (kind_of(x) != kind_of(y) || x->gp_kind == kind::Foreign || y->gp_kind == kind::Foreign) {
        return false;
    }
    switch (x->gp_kind) {
    case kind::Array:
        return x->gp_len == y->gp_len && x->gp_elem == y->gp_elem;
    case kind::Chan:
        return x->gp_dir == y->gp_dir && x->gp_elem == y->gp_elem;
    case kind::Func:
        if (x->gp_variadic != y->gp_variadic || x->gp_nin != y->gp_nin ||
            x->gp_nout != y->gp_nout) {
            return false;
        }
        for (std::size_t i = 0; i < x->gp_nin; i++) {
            if (x->gp_in[i] != y->gp_in[i]) {
                return false;
            }
        }
        for (std::size_t i = 0; i < x->gp_nout; i++) {
            if (x->gp_out[i] != y->gp_out[i]) {
                return false;
            }
        }
        return true;
    case kind::Interface:
        if (x->gp_nimethods != y->gp_nimethods) {
            return false;
        }
        for (std::size_t i = 0; i < x->gp_nimethods; i++) {
            const imethod& m = x->gp_imethods[i];
            const imethod& n = y->gp_imethods[i];
            if (!same_str(m.gp_name, n.gp_name) || !same_str(m.gp_pkg, n.gp_pkg) ||
                !same_str(m.gp_sig, n.gp_sig)) {
                return false;
            }
        }
        return true;
    case kind::Map:
        return x->gp_key == y->gp_key && x->gp_elem == y->gp_elem;
    case kind::Pointer:
    case kind::Slice:
        return x->gp_elem == y->gp_elem;
    case kind::Struct:
        if (x->gp_nfields != y->gp_nfields) {
            return false;
        }
        for (std::size_t i = 0; i < x->gp_nfields; i++) {
            if (!same_fields(x->gp_fields[i], y->gp_fields[i], tags)) {
                return false;
            }
        }
        return true;
    default:
        return true; // the basic kinds: equal kinds
    }
}

bool iface_implements(const type_desc* t, const type_desc* u) noexcept {
    const type_desc* ft = full(t);
    const type_desc* fu = full(u);
    for (std::size_t i = 0; i < fu->gp_nimethods; i++) {
        const imethod& m = fu->gp_imethods[i];
        bool found = false;
        for (std::size_t j = 0; j < ft->gp_nimethods; j++) {
            const imethod& n = ft->gp_imethods[j];
            if (same_str(m.gp_name, n.gp_name) && same_str(m.gp_pkg, n.gp_pkg) &&
                same_str(m.gp_sig, n.gp_sig)) {
                found = true;
                break;
            }
        }
        if (!found) {
            return false;
        }
    }
    return true;
}

bool implements_desc(const type_desc* t, const type_desc* u) noexcept {
    if (u->gp_kind != kind::Interface) {
        return false;
    }
    if (u->gp_nimethods == 0) {
        return true;
    }
    if (t->gp_kind == kind::Interface) {
        return iface_implements(t, u);
    }
    return gp::implements(u, t);
}

bool comparable_desc(const type_desc* d, int depth = 0) noexcept {
    if (depth > 64) {
        return true;
    }
    switch (kind_of(d)) {
    case static_cast<std::int64_t>(kind::Slice):
    case static_cast<std::int64_t>(kind::Map):
    case static_cast<std::int64_t>(kind::Func):
        return false;
    case static_cast<std::int64_t>(kind::Array):
        return comparable_desc(full(d)->gp_elem, depth + 1);
    case static_cast<std::int64_t>(kind::Struct): {
        if (d->gp_kind == kind::Foreign) {
            return d->gp_equal != nullptr;
        }
        const type_desc* f = full(d);
        for (std::size_t i = 0; i < f->gp_nfields; i++) {
            if (!comparable_desc(f->gp_fields[i].gp_type, depth + 1)) {
                return false;
            }
        }
        return true;
    }
    default:
        return true;
    }
}

bool assignable_desc(const type_desc* v, const type_desc* t) noexcept {
    if (v == t) {
        return true;
    }
    if (t->gp_kind == kind::Interface) {
        return implements_desc(v, t);
    }
    if (v->gp_kind == kind::Foreign || t->gp_kind == kind::Foreign) {
        return false;
    }
    if (!same_underlying(v, t, true)) {
        // A bidirectional channel is assignable to a channel type of the same element when one of
        // the two is not a named type.
        if (v->gp_kind == kind::Chan && t->gp_kind == kind::Chan &&
            (!v->gp_named || !t->gp_named)) {
            const type_desc* x = full(v);
            const type_desc* y = full(t);
            return x->gp_dir == chan_dir::both && x->gp_elem == y->gp_elem;
        }
        return false;
    }
    return !v->gp_named || !t->gp_named;
}

bool numeric_type(const type_desc* d) noexcept {
    return signed_type(d) || unsigned_type(d) || float_type(d);
}

bool byte_or_rune_slice(const type_desc* d, kind elem) noexcept {
    if (d->gp_kind != kind::Slice) {
        return false;
    }
    const type_desc* e = full(d)->gp_elem;
    return e != nullptr && e->gp_kind == elem;
}

bool convertible_desc(const type_desc* v, const type_desc* t) noexcept {
    if (assignable_desc(v, t)) {
        return true;
    }
    if (v->gp_kind == kind::Foreign || t->gp_kind == kind::Foreign) {
        return numeric_type(v) && numeric_type(t);
    }
    if (same_underlying(v, t, false)) {
        return true;
    }
    if (v->gp_kind == kind::Pointer && t->gp_kind == kind::Pointer && !v->gp_named &&
        !t->gp_named && same_underlying(full(v)->gp_elem, full(t)->gp_elem, false)) {
        return true;
    }
    if (numeric_type(v) && numeric_type(t)) {
        return true;
    }
    if (complex_type(v) && complex_type(t)) {
        return true;
    }
    if (t->gp_kind == kind::String &&
        (signed_type(v) || unsigned_type(v) || byte_or_rune_slice(v, kind::Uint8) ||
         byte_or_rune_slice(v, kind::Int32))) {
        return true;
    }
    if (v->gp_kind == kind::String &&
        (byte_or_rune_slice(t, kind::Uint8) || byte_or_rune_slice(t, kind::Int32))) {
        return true;
    }
    if (v->gp_kind == kind::Slice) {
        // []T to [N]T and to *[N]T
        const type_desc* e = full(v)->gp_elem;
        if (t->gp_kind == kind::Array && full(t)->gp_elem == e) {
            return true;
        }
        if (t->gp_kind == kind::Pointer && !t->gp_named) {
            const type_desc* a = full(t)->gp_elem;
            return a != nullptr && a->gp_kind == kind::Array && full(a)->gp_elem == e;
        }
    }
    return false;
}

// --- dynamic descriptors ---

const type_desc* ptr_to(const type_desc* elem) {
    // The layout of a gp::ptr<E> does not depend on E, nor do its operations (they read the owner
    // and the target, and make a pointer from them), so the one for void serves every E; the
    // interning returns the program's own descriptor when it has one.
    type_desc p = detail::with_cxx<gp::ptr<void>>(shape_pointer(elem));
    p.gp_methods = elem->gp_pmethods;
    p.gp_nmethods = elem->gp_npmethods;
    return detail::intern(p);
}

const type_desc* slice_of(const type_desc* elem) {
    // Likewise a gp::slice<E> (the owner, the data pointer, the length and the capacity): the
    // operations read and write those four words; the elements are made by their own type's
    // lifetime operations (alloc_array).
    return detail::intern(detail::with_cxx<gp::slice<std::uint8_t>>(shape_slice(elem)));
}

// --- storage accessors ---

inline void* addr(const vptr& p) noexcept {
    return p.gp_get();
}

const slice_refl* slice_ops(const type_desc* d) {
    const type_ops* o = ops_of_desc(d);
    if (o == nullptr || o->gp_refl == nullptr) {
        opaque(d);
    }
    return static_cast<const slice_refl*>(o->gp_refl);
}

const map_refl* map_ops(const type_desc* d) {
    const type_ops* o = ops_of_desc(d);
    if (o == nullptr || o->gp_refl == nullptr) {
        opaque(d);
    }
    return static_cast<const map_refl*>(o->gp_refl);
}

std::size_t elem_size(const type_desc* d) noexcept {
    const type_desc* e = full(d)->gp_elem;
    return e == nullptr ? 0 : e->gp_size;
}

// copy_elems assigns n elements of type e from src to dst; the two runs may overlap.
void copy_elems(const type_desc* e, char* dst, const char* src, std::int64_t n) {
    if (n <= 0 || dst == src) {
        return;
    }
    const std::size_t size = e->gp_size;
    const type_ops* o = ops_of_desc(e);
    if (e->gp_kind != kind::Interface && o != nullptr && o->gp_trivial) {
        std::memmove(dst, src, static_cast<std::size_t>(n) * size);
        return;
    }
    if (dst < src) {
        for (std::int64_t i = 0; i < n; i++) {
            assign(e, dst + static_cast<std::size_t>(i) * size,
                   src + static_cast<std::size_t>(i) * size);
        }
    } else {
        for (std::int64_t i = n - 1; i >= 0; i--) {
            assign(e, dst + static_cast<std::size_t>(i) * size,
                   src + static_cast<std::size_t>(i) * size);
        }
    }
}

// The parts of a slice or an array object.
struct seq_view {
    char* data = nullptr;
    std::int64_t len = 0;
    std::int64_t cap = 0;
    std::shared_ptr<void> owner;
};

seq_view view_of(const type_desc* d, const vptr& p) {
    seq_view v;
    const type_ops* o = ops_of_desc(d);
    if (d->gp_kind == kind::Array) {
        v.data = static_cast<char*>(const_cast<void*>(o->gp_seq_data(addr(p))));
        v.len = v.cap = d->gp_len;
        v.owner = p.gp_shared();
    } else {
        v.data = static_cast<char*>(const_cast<void*>(o->gp_seq_data(addr(p))));
        v.len = o->gp_seq_len(addr(p));
        v.cap = static_cast<const slice_refl*>(o->gp_refl)->gp_cap(addr(p));
        v.owner = o->gp_hold(addr(p));
    }
    return v;
}

const gp::string& string_at(const vptr& p) noexcept {
    return *static_cast<const gp::string*>(addr(p));
}

} // namespace

// --- types ---

std::uint64_t type_of(const gp::iface& x) noexcept {
    return U(x.gp_type());
}

std::int64_t t_kind(std::uint64_t t) noexcept {
    return kind_of(D(t));
}

gp::string t_name(std::uint64_t t) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign) {
        return text(d->gp_name);
    }
    if (!d->gp_named) {
        return gp::string();
    }
    const char* name = d->gp_name;
    if (d->gp_kind == kind::UnsafePointer && d->gp_pkg == nullptr) {
        return text("Pointer"); // unsafe.Pointer: the type Pointer of package unsafe
    }
    if (d->gp_pkg == nullptr) {
        return text(name); // a predeclared type: int, error
    }
    // pkg.Name or pkg.Name[args]: the part after the package name's dot.
    const char* bracket = std::strchr(name, '[');
    const char* dot = std::strchr(name, '.');
    if (dot != nullptr && (bracket == nullptr || dot < bracket)) {
        return text(dot + 1);
    }
    return text(name);
}

gp::string t_pkgpath(std::uint64_t t) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign || !d->gp_named) {
        return gp::string();
    }
    if (d->gp_kind == kind::UnsafePointer && d->gp_pkg == nullptr) {
        return text("unsafe");
    }
    return text(d->gp_pkg);
}

gp::string t_string(std::uint64_t t) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr && d->gp_ops->gp_go_name != nullptr) {
        return text(d->gp_ops->gp_go_name);
    }
    return text(d->gp_name);
}

gp::string t_cppname(std::uint64_t t) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign) {
        return text(d->gp_name);
    }
    return gp::string();
}

std::uint64_t t_size(std::uint64_t t) noexcept {
    return D(t)->gp_size;
}

std::uint64_t t_align(std::uint64_t t) noexcept {
    return D(t)->gp_align;
}

std::uint64_t t_elem(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    switch (d->gp_kind) {
    case kind::Array:
    case kind::Chan:
    case kind::Map:
    case kind::Pointer:
    case kind::Slice:
        return U(full(d)->gp_elem);
    default:
        return 0;
    }
}

std::uint64_t t_key(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Map ? U(full(d)->gp_key) : 0;
}

std::int64_t t_len(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Array ? d->gp_len : 0;
}

std::int64_t t_chandir(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Chan ? static_cast<std::int64_t>(full(d)->gp_dir) : 0;
}

std::int64_t t_numfield(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Struct ? static_cast<std::int64_t>(full(d)->gp_nfields) : 0;
}

gp::string f_name(std::uint64_t t, std::int64_t i) {
    return text(full(D(t))->gp_fields[i].gp_name);
}

gp::string f_pkgpath(std::uint64_t t, std::int64_t i) {
    return text(full(D(t))->gp_fields[i].gp_pkg);
}

gp::string f_tag(std::uint64_t t, std::int64_t i) {
    return text(full(D(t))->gp_fields[i].gp_tag);
}

std::uint64_t f_type(std::uint64_t t, std::int64_t i) noexcept {
    return U(full(D(t))->gp_fields[i].gp_type);
}

bool f_embedded(std::uint64_t t, std::int64_t i) noexcept {
    return full(D(t))->gp_fields[i].gp_embedded;
}

std::uint64_t f_offset(std::uint64_t t, std::int64_t i) noexcept {
    return full(D(t))->gp_fields[i].gp_offset;
}

std::int64_t t_numin(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Func ? static_cast<std::int64_t>(full(d)->gp_nin) : 0;
}

std::int64_t t_numout(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Func ? static_cast<std::int64_t>(full(d)->gp_nout) : 0;
}

std::uint64_t t_in(std::uint64_t t, std::int64_t i) noexcept {
    return U(full(D(t))->gp_in[i]);
}

std::uint64_t t_out(std::uint64_t t, std::int64_t i) noexcept {
    return U(full(D(t))->gp_out[i]);
}

bool t_variadic(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Func && full(d)->gp_variadic;
}

bool t_comparable(std::uint64_t t) noexcept {
    return comparable_desc(D(t));
}

std::int64_t t_nmethod(std::uint64_t t) noexcept {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Interface) {
        return static_cast<std::int64_t>(full(d)->gp_nimethods);
    }
    std::int64_t n = 0;
    for (std::size_t i = 0; i < d->gp_nmethods; i++) {
        if (d->gp_methods[i].gp_pkg == nullptr) {
            n++;
        }
    }
    return n;
}

gp::string m_name(std::uint64_t t, std::int64_t i) {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Interface ? text(full(d)->gp_imethods[i].gp_name)
                                         : text(d->gp_methods[i].gp_name);
}

gp::string m_pkgpath(std::uint64_t t, std::int64_t i) {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Interface ? text(full(d)->gp_imethods[i].gp_pkg)
                                         : text(d->gp_methods[i].gp_pkg);
}

gp::string m_sig(std::uint64_t t, std::int64_t i) {
    const type_desc* d = D(t);
    return d->gp_kind == kind::Interface ? text(full(d)->gp_imethods[i].gp_sig)
                                         : text(d->gp_methods[i].gp_sig);
}

std::uint64_t m_type(std::uint64_t t, std::int64_t i) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Interface) {
        const imethod& m = full(d)->gp_imethods[i];
        return m.gp_ftype != nullptr ? U(m.gp_ftype()) : 0;
    }
    const method& m = d->gp_methods[i];
    return m.gp_ftype != nullptr ? U(sig_with_receiver(d, m.gp_ftype())) : 0;
}

bool t_implements(std::uint64_t t, std::uint64_t u) noexcept {
    return implements_desc(D(t), D(u));
}

bool t_assignable(std::uint64_t t, std::uint64_t u) noexcept {
    return assignable_desc(D(t), D(u));
}

bool t_convertible(std::uint64_t t, std::uint64_t u) noexcept {
    return convertible_desc(D(t), D(u));
}

std::uint64_t t_ptrto(std::uint64_t t) {
    return U(ptr_to(D(t)));
}

std::uint64_t t_sliceof(std::uint64_t t) {
    return U(slice_of(D(t)));
}

vptr rtype_of(std::uint64_t t) {
    // One block for each type, never freed (the table is reachable, so no leak report).
    static std::mutex mu;
    static auto* const table =
        new std::unordered_map<std::uint64_t, std::shared_ptr<std::uint64_t>>();
    std::lock_guard<std::mutex> lock(mu);
    auto it = table->find(t);
    if (it == table->end()) {
        it = table->emplace(t, std::make_shared<std::uint64_t>(t)).first;
    }
    const std::shared_ptr<std::uint64_t>& sp = it->second;
    return vptr(sp, static_cast<void*>(sp.get()));
}

// --- values ---

std::tuple<std::uint64_t, vptr> v_unpack(const gp::iface& x) {
    return unpack(x);
}

std::tuple<std::uint64_t, vptr> v_ifaceelem(std::uint64_t, const vptr& p) {
    return unpack(*static_cast<const gp::iface*>(addr(p)));
}

gp::any v_interface(std::uint64_t t, const vptr& p) {
    return pack(D(t), addr(p));
}

std::int64_t v_int(std::uint64_t t, const vptr& p) noexcept {
    const void* a = addr(p);
    switch (D(t)->gp_size) {
    case 1:
        return *static_cast<const std::int8_t*>(a);
    case 2:
        return *static_cast<const std::int16_t*>(a);
    case 4:
        return *static_cast<const std::int32_t*>(a);
    default:
        return *static_cast<const std::int64_t*>(a);
    }
}

std::uint64_t v_uint(std::uint64_t t, const vptr& p) noexcept {
    const void* a = addr(p);
    switch (D(t)->gp_size) {
    case 1:
        return *static_cast<const std::uint8_t*>(a);
    case 2:
        return *static_cast<const std::uint16_t*>(a);
    case 4:
        return *static_cast<const std::uint32_t*>(a);
    default:
        return *static_cast<const std::uint64_t*>(a);
    }
}

double v_float(std::uint64_t t, const vptr& p) noexcept {
    return D(t)->gp_size == 4 ? static_cast<double>(*static_cast<const float*>(addr(p)))
                              : *static_cast<const double*>(addr(p));
}

gp::complex128 v_complex(std::uint64_t t, const vptr& p) noexcept {
    if (D(t)->gp_size == 8) {
        const auto* c = static_cast<const gp::complex64*>(addr(p));
        return gp::complex128(static_cast<double>(c->gp_re), static_cast<double>(c->gp_im));
    }
    return *static_cast<const gp::complex128*>(addr(p));
}

gp::string v_string(std::uint64_t t, const vptr& p) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr) {
        if (d->gp_ops->gp_fclass == fclass::cxx_string) {
            return gp::string::gp_copy(*static_cast<const std::string*>(addr(p)));
        }
        if (d->gp_ops->gp_fclass == fclass::cxx_string_view) {
            return gp::string::gp_copy(*static_cast<const std::string_view*>(addr(p)));
        }
    }
    return string_at(p);
}

bool v_bool(std::uint64_t, const vptr& p) noexcept {
    return *static_cast<const bool*>(addr(p));
}

gp::slice<std::uint8_t> v_bytes(std::uint64_t, const vptr& p) {
    return *static_cast<const gp::slice<std::uint8_t>*>(addr(p));
}

// check_num_store: a foreign number can be stored as a Go number of its size (1, 2, 4 or 8 bytes).
void check_num_store(const type_desc* d, const char* what) {
    if (d->gp_kind == kind::Foreign &&
        (d->gp_ops == nullptr ||
         (d->gp_size != 1 && d->gp_size != 2 && d->gp_size != 4 && d->gp_size != 8))) {
        unsupported(d, what);
    }
}

void v_setint(std::uint64_t t, const vptr& p, std::int64_t x) {
    check_num_store(D(t), "SetInt");
    void* a = addr(p);
    switch (D(t)->gp_size) {
    case 1:
        *static_cast<std::int8_t*>(a) = static_cast<std::int8_t>(x);
        break;
    case 2:
        *static_cast<std::int16_t*>(a) = static_cast<std::int16_t>(x);
        break;
    case 4:
        *static_cast<std::int32_t*>(a) = static_cast<std::int32_t>(x);
        break;
    default:
        *static_cast<std::int64_t*>(a) = x;
    }
}

void v_setuint(std::uint64_t t, const vptr& p, std::uint64_t x) {
    check_num_store(D(t), "SetUint");
    void* a = addr(p);
    switch (D(t)->gp_size) {
    case 1:
        *static_cast<std::uint8_t*>(a) = static_cast<std::uint8_t>(x);
        break;
    case 2:
        *static_cast<std::uint16_t*>(a) = static_cast<std::uint16_t>(x);
        break;
    case 4:
        *static_cast<std::uint32_t*>(a) = static_cast<std::uint32_t>(x);
        break;
    default:
        *static_cast<std::uint64_t*>(a) = x;
    }
}

void v_setfloat(std::uint64_t t, const vptr& p, double x) {
    check_num_store(D(t), "SetFloat");
    if (D(t)->gp_size == 4) {
        *static_cast<float*>(addr(p)) = static_cast<float>(x);
    } else {
        *static_cast<double*>(addr(p)) = x;
    }
}

void v_setcomplex(std::uint64_t t, const vptr& p, gp::complex128 x) {
    if (D(t)->gp_kind == kind::Foreign) {
        unsupported(D(t), "SetComplex");
    }
    if (D(t)->gp_size == 8) {
        *static_cast<gp::complex64*>(addr(p)) =
            gp::complex64(static_cast<float>(x.gp_re), static_cast<float>(x.gp_im));
    } else {
        *static_cast<gp::complex128*>(addr(p)) = x;
    }
}

void v_setstring(std::uint64_t t, const vptr& p, gp::string x) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign) {
        // A std::string takes a copy of the bytes; a std::string_view would point into x, which
        // nothing keeps alive.
        if (d->gp_ops != nullptr && d->gp_ops->gp_fclass == fclass::cxx_string) {
            static_cast<std::string*>(addr(p))->assign(x.gp_view());
            return;
        }
        unsupported(d, "SetString");
    }
    *static_cast<gp::string*>(addr(p)) = std::move(x);
}

void v_setbool(std::uint64_t t, const vptr& p, bool x) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Foreign && d->gp_size != sizeof(bool)) {
        unsupported(d, "SetBool");
    }
    *static_cast<bool*>(addr(p)) = x;
}

bool v_isnil(std::uint64_t t, const vptr& p) noexcept {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Interface) {
        return static_cast<const gp::iface*>(addr(p))->gp_type() == nullptr;
    }
    const type_ops* o = ops_of_desc(d);
    if (o != nullptr && o->gp_nil != nullptr) {
        return o->gp_nil(addr(p));
    }
    if (kind_of(d) == static_cast<std::int64_t>(kind::UnsafePointer)) {
        if (o != nullptr && o->gp_target != nullptr) {
            return o->gp_target(addr(p)) == nullptr;
        }
        return *static_cast<void* const*>(addr(p)) == nullptr;
    }
    return false;
}

vptr v_unsafeptr(std::uint64_t t, const vptr& p) {
    const type_desc* d = D(t);
    const type_ops* o = ops_of_desc(d);
    const void* target = nullptr;
    std::shared_ptr<void> owner;
    if (kind_of(d) == static_cast<std::int64_t>(kind::String)) {
        // The bytes of the string, where they are: a C++ string's buffer lives as long as the
        // object; a Go string keeps its own owner.
        if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr &&
            d->gp_ops->gp_fclass == fclass::cxx_string) {
            target = static_cast<const std::string*>(addr(p))->data();
        } else if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr) {
            target = static_cast<const std::string_view*>(addr(p))->data();
        } else {
            const gp::string& str = *static_cast<const gp::string*>(addr(p));
            target = str.gp_data();
            owner = std::const_pointer_cast<void>(str.gp_owner());
        }
    } else if (o != nullptr && o->gp_target != nullptr) {
        target = o->gp_target(addr(p));
        owner = o->gp_owner(addr(p));
    } else if (d->gp_kind == kind::Slice && o != nullptr) {
        target = o->gp_seq_data(addr(p));
        owner = o->gp_hold(addr(p));
    } else if (o != nullptr && o->gp_addr != nullptr) {
        target = o->gp_addr(addr(p));
    } else if (d->gp_kind == kind::Map) {
        // A map is a shared_ptr to its table: its first word is the table's address.
        target = *static_cast<const void* const*>(addr(p));
    } else if (kind_of(d) == static_cast<std::int64_t>(kind::UnsafePointer)) {
        target = *static_cast<const void* const*>(addr(p)); // a raw C pointer
    }
    if (target == nullptr) {
        return vptr();
    }
    if (owner == nullptr) {
        owner = gp::foreign_owner();
    }
    return vptr(owner, const_cast<void*>(target));
}

void v_assign(std::uint64_t t, const vptr& dst, const vptr& src) {
    assign(D(t), addr(dst), addr(src));
}

void v_assign_iface(std::uint64_t dt, const vptr& dst, std::uint64_t st, const vptr& src) {
    const type_desc* it = D(dt);
    gp::any x = pack(D(st), addr(src));
    auto* out = static_cast<gp::iface*>(addr(dst));
    if (x.gp_type() == nullptr) {
        *out = gp::iface();
        return;
    }
    const itab* tab = nullptr;
    if (it->gp_nimethods != 0) {
        tab = gp::getitab(it, x.gp_type());
        if (tab == nullptr) {
            fail(std::string("reflect: value of type ") + x.gp_type()->gp_name +
                 " does not implement " + it->gp_name);
        }
    }
    *out = gp::iface(x.gp_data(), x.gp_type(), tab);
}

vptr v_new(std::uint64_t t) {
    return new_storage(D(t));
}

vptr v_ptrto(std::uint64_t pt, const vptr& target) {
    const type_desc* d = D(pt);
    const type_ops* o = ops_of_desc(d);
    if (o == nullptr || o->gp_refl == nullptr) {
        opaque(d);
    }
    vptr s = new_storage(d);
    static_cast<const ptr_refl*>(o->gp_refl)->gp_make(addr(s), target.gp_shared(), addr(target));
    return s;
}

vptr v_offset(const vptr& p, std::uint64_t off) {
    return vptr(p, static_cast<char*>(addr(p)) + off);
}

vptr v_elem(std::uint64_t t, const vptr& p) {
    const type_ops* o = ops_of_desc(D(t));
    const void* target = o->gp_target(addr(p));
    if (target == nullptr) {
        return vptr();
    }
    return vptr(o->gp_owner(addr(p)), const_cast<void*>(target));
}

std::int64_t v_len(std::uint64_t t, const vptr& p) noexcept {
    const type_desc* d = D(t);
    const type_ops* o = ops_of_desc(d);
    switch (d->gp_kind) {
    case kind::Array:
        return d->gp_len;
    case kind::Slice:
        return o->gp_seq_len(addr(p));
    case kind::Map:
        return o != nullptr && o->gp_refl != nullptr
                   ? static_cast<const map_refl*>(o->gp_refl)->gp_len(addr(p))
                   : 0;
    case kind::Chan:
        return o != nullptr && o->gp_refl != nullptr
                   ? static_cast<const chan_refl*>(o->gp_refl)->gp_len(addr(p))
                   : 0;
    default: // a string (named or not, or a C++ string)
        if (d->gp_kind == kind::Foreign && d->gp_ops != nullptr) {
            if (d->gp_ops->gp_fclass == fclass::cxx_string) {
                return static_cast<std::int64_t>(static_cast<const std::string*>(addr(p))->size());
            }
            if (d->gp_ops->gp_fclass == fclass::cxx_string_view) {
                return static_cast<std::int64_t>(
                    static_cast<const std::string_view*>(addr(p))->size());
            }
        }
        return string_at(p).gp_view().size();
    }
}

std::int64_t v_cap(std::uint64_t t, const vptr& p) noexcept {
    const type_desc* d = D(t);
    const type_ops* o = ops_of_desc(d);
    switch (d->gp_kind) {
    case kind::Array:
        return d->gp_len;
    case kind::Slice:
        return static_cast<const slice_refl*>(o->gp_refl)->gp_cap(addr(p));
    case kind::Chan:
        return o != nullptr && o->gp_refl != nullptr
                   ? static_cast<const chan_refl*>(o->gp_refl)->gp_cap(addr(p))
                   : 0;
    default:
        return 0;
    }
}

vptr v_index(std::uint64_t t, const vptr& p, std::int64_t i) {
    const type_desc* d = D(t);
    seq_view v = view_of(d, p);
    const std::size_t es = elem_size(d);
    return vptr(v.owner, v.data + static_cast<std::size_t>(i) * es);
}

vptr v_mkslice(std::uint64_t t, std::int64_t len, std::int64_t cap) {
    const type_desc* d = D(t);
    const slice_refl* r = slice_ops(d);
    const type_desc* e = full(d)->gp_elem;
    vptr s = new_storage(d);
    std::shared_ptr<void> arr = alloc_array(e, cap);
    r->gp_make(addr(s), arr, arr.get(), len, cap);
    return s;
}

vptr v_reslice(std::uint64_t t, const vptr& p, std::uint64_t st, std::int64_t i, std::int64_t j,
               std::int64_t k) {
    const type_desc* d = D(t);
    const type_desc* rd = D(st);
    vptr res = new_storage(rd);
    if (d->gp_kind == kind::String || (d->gp_kind == kind::Foreign)) {
        gp::string s = v_string(t, p);
        if (rd->gp_kind == kind::Foreign) {
            // The result has the string's own (C++) type: a std::string gets a copy of the bytes.
            if (rd->gp_ops != nullptr && rd->gp_ops->gp_fclass == fclass::cxx_string) {
                static_cast<std::string*>(addr(res))->assign(s.gp_view().substr(
                    static_cast<std::size_t>(i), static_cast<std::size_t>(j - i)));
                return res;
            }
            unsupported(rd, "Slice");
        }
        *static_cast<gp::string*>(addr(res)) = gp::string(s.gp_owner(), s.gp_data() + i, j - i);
        return res;
    }
    seq_view v = view_of(d, p);
    const std::size_t es = elem_size(d);
    slice_ops(rd)->gp_make(addr(res), v.owner, v.data + static_cast<std::size_t>(i) * es, j - i,
                           k - i);
    return res;
}

void v_setlen(std::uint64_t t, const vptr& p, std::int64_t n) {
    const type_desc* d = D(t);
    seq_view v = view_of(d, p);
    slice_ops(d)->gp_make(addr(p), v.owner, v.data, n, v.cap);
}

// go_layout: Go's size, alignment and pointer-ness of the Go type d (gocpp/golayout.hpp does the
// same from a C++ type): growslice's capacity depends on them, not on the C++ layout.
struct go_layout_info {
    std::uint64_t size;
    std::uint64_t align;
    bool ptrs;
};

static go_layout_info go_layout(const type_desc* d0) {
    const type_desc* d = full(d0);
    switch (d->gp_kind) {
    case kind::Bool:
    case kind::Int8:
    case kind::Uint8:
        return {1, 1, false};
    case kind::Int16:
    case kind::Uint16:
        return {2, 2, false};
    case kind::Int32:
    case kind::Uint32:
    case kind::Float32:
        return {4, 4, false};
    case kind::Int:
    case kind::Int64:
    case kind::Uint:
    case kind::Uint64:
    case kind::Uintptr:
    case kind::Float64:
        return {8, 8, false};
    case kind::Complex64:
        return {8, 4, false};
    case kind::Complex128:
        return {16, 8, false};
    case kind::String:
    case kind::Interface:
        return {16, 8, true};
    case kind::Slice:
        return {24, 8, true};
    case kind::Pointer:
    case kind::UnsafePointer:
    case kind::Map:
    case kind::Chan:
    case kind::Func:
        return {8, 8, true};
    case kind::Array: {
        go_layout_info e = go_layout(d->gp_elem);
        if (d->gp_len == 0) {
            return {0, e.align, false};
        }
        return {e.size * static_cast<std::uint64_t>(d->gp_len), e.align, e.ptrs};
    }
    case kind::Struct: {
        if (d->gp_nfields == 0) {
            if (d->gp_size <= 1) {
                return {0, 1, false}; // struct{}
            }
            return {d->gp_size, d->gp_align, true}; // fields unknown: the C++ layout
        }
        std::uint64_t off = 0, last_off = 0, last_size = 0, align = 1;
        bool ptrs = false;
        for (std::size_t i = 0; i < d->gp_nfields; i++) {
            go_layout_info f = go_layout(d->gp_fields[i].gp_type);
            off = (off + f.align - 1) / f.align * f.align;
            last_off = off;
            last_size = f.size;
            off += f.size;
            align = std::max(align, f.align);
            ptrs = ptrs || f.ptrs;
        }
        std::uint64_t size = last_off + (last_off > 0 && last_size == 0 ? 1 : last_size);
        return {(size + align - 1) / align * align, align, ptrs};
    }
    default:
        return {d->gp_size, d->gp_align, true}; // a C/C++ type: its own layout
    }
}

vptr v_extend(std::uint64_t t, const vptr& p, std::int64_t n) {
    const type_desc* d = D(t);
    const slice_refl* r = slice_ops(d);
    seq_view v = view_of(d, p);
    const type_desc* e = full(d)->gp_elem;
    vptr res = new_storage(d);
    const std::int64_t newlen = v.len + n;
    if (newlen <= v.cap) {
        r->gp_make(addr(res), v.owner, v.data, newlen, v.cap);
        return res;
    }
    const std::int64_t newcap =
        detail::grow_cap(newlen, v.cap, go_layout(e).size, !go_layout(e).ptrs);
    std::shared_ptr<void> arr = alloc_array(e, newcap);
    copy_elems(e, static_cast<char*>(arr.get()), v.data, v.len);
    r->gp_make(addr(res), arr, arr.get(), newlen, newcap);
    return res;
}

std::int64_t v_copy(std::uint64_t dt, const vptr& dp, std::uint64_t st, const vptr& sp) {
    const type_desc* dd = D(dt);
    const type_desc* sd = D(st);
    seq_view dv = view_of(dd, dp);
    const type_desc* e = full(dd)->gp_elem;
    if (sd->gp_kind == kind::String || sd->gp_kind == kind::Foreign) {
        gp::string s = v_string(st, sp);
        std::int64_t n =
            std::min<std::int64_t>(dv.len, static_cast<std::int64_t>(s.gp_view().size()));
        if (n > 0) {
            std::memmove(dv.data, s.gp_data(), static_cast<std::size_t>(n));
        }
        return n;
    }
    seq_view sv = view_of(sd, sp);
    std::int64_t n = std::min(dv.len, sv.len);
    copy_elems(e, dv.data, sv.data, n);
    return n;
}

vptr v_mapnew(std::uint64_t t) {
    const type_desc* d = D(t);
    const map_refl* r = map_ops(d);
    vptr m = new_storage(d);
    r->gp_make(addr(m));
    return m;
}

vptr v_mapindex(std::uint64_t t, const vptr& p, const vptr& kp) {
    const type_desc* d = D(t);
    const map_refl* r = map_ops(d);
    const void* f = r->gp_find(addr(p), addr(kp));
    if (f == nullptr) {
        return vptr();
    }
    const type_desc* e = full(d)->gp_elem;
    vptr res = new_storage(e);
    assign(e, addr(res), f);
    return res;
}

bool v_maphas(std::uint64_t t, const vptr& p, const vptr& kp) {
    return map_ops(D(t))->gp_find(addr(p), addr(kp)) != nullptr;
}

void v_mapset(std::uint64_t t, const vptr& p, const vptr& kp, const vptr& vp) {
    map_ops(D(t))->gp_set(addr(p), addr(kp), addr(vp));
}

void v_mapdel(std::uint64_t t, const vptr& p, const vptr& kp) {
    map_ops(D(t))->gp_erase(addr(p), addr(kp));
}

void v_mapclear(std::uint64_t t, const vptr& p) {
    map_ops(D(t))->gp_clear(addr(p));
}

vptr v_mapsnap(std::uint64_t t, const vptr& p) {
    const type_desc* d = D(t);
    const type_ops* o = ops_of_desc(d);
    if (o == nullptr || o->gp_map_snap == nullptr) {
        opaque(d);
    }
    std::shared_ptr<void> h = o->gp_map_snap(addr(p));
    return vptr(h, h.get());
}

std::int64_t v_snaplen(std::uint64_t t, const vptr& h) {
    return D(t)->gp_ops->gp_snap_len(addr(h));
}

vptr v_snapkey(std::uint64_t t, const vptr& h, std::int64_t i) {
    const void* k = nullptr;
    const void* v = nullptr;
    D(t)->gp_ops->gp_snap_at(addr(h), i, &k, &v);
    return vptr(h, const_cast<void*>(k));
}

vptr v_snapval(std::uint64_t t, const vptr& h, std::int64_t i) {
    const void* k = nullptr;
    const void* v = nullptr;
    D(t)->gp_ops->gp_snap_at(addr(h), i, &k, &v);
    return vptr(h, const_cast<void*>(v));
}

bool v_equal(std::uint64_t t, const vptr& p, const vptr& q) {
    const type_desc* d = D(t);
    if (d->gp_kind == kind::Interface) {
        return *static_cast<const gp::iface*>(addr(p)) == *static_cast<const gp::iface*>(addr(q));
    }
    if (d->gp_equal == nullptr) {
        fail(std::string("reflect: comparing uncomparable type ") + d->gp_name);
    }
    const type_ops* o = ops_of_desc(d);
    if (o != nullptr && o->gp_share) {
        return d->gp_equal(o->gp_target(addr(p)), o->gp_target(addr(q)));
    }
    return d->gp_equal(addr(p), addr(q));
}

namespace {

// to_int64: a float64 converted as Go's amd64 conversion does (out of range: the "integer
// indefinite" value, the minimum).
std::int64_t float_to_int64(double f) noexcept {
    if (!(f >= -9223372036854775808.0 && f < 9223372036854775808.0)) {
        return std::numeric_limits<std::int64_t>::min();
    }
    return static_cast<std::int64_t>(f);
}

std::uint64_t float_to_uint64(double f) noexcept {
    if (f >= 9223372036854775808.0) {
        return static_cast<std::uint64_t>(float_to_int64(f - 9223372036854775808.0)) +
               (std::uint64_t(1) << 63);
    }
    return static_cast<std::uint64_t>(float_to_int64(f));
}

} // namespace

vptr v_convert(std::uint64_t st, const vptr& sp, std::uint64_t dt) {
    const type_desc* sd = D(st);
    const type_desc* dd = D(dt);
    vptr res = new_storage(dd);
    const std::int64_t sk = kind_of(sd);
    const std::int64_t dk = kind_of(dd);
    auto dst_int = [&](std::int64_t x) { v_setint(dt, res, x); };
    auto dst_uint = [&](std::uint64_t x) { v_setuint(dt, res, x); };
    auto dst_float = [&](double x) { v_setfloat(dt, res, x); };
    if (numeric_type(sd) && numeric_type(dd)) {
        if (signed_type(dd) || unsigned_type(dd)) {
            std::uint64_t bits;
            if (signed_type(sd)) {
                bits = static_cast<std::uint64_t>(v_int(st, sp));
            } else if (unsigned_type(sd)) {
                bits = v_uint(st, sp);
            } else {
                double f = v_float(st, sp);
                bits = signed_type(dd) ? static_cast<std::uint64_t>(float_to_int64(f))
                                       : float_to_uint64(f);
            }
            if (signed_type(dd)) {
                dst_int(static_cast<std::int64_t>(bits));
            } else {
                dst_uint(bits);
            }
        } else {
            double f = signed_type(sd)     ? static_cast<double>(v_int(st, sp))
                       : unsigned_type(sd) ? static_cast<double>(v_uint(st, sp))
                                           : v_float(st, sp);
            dst_float(f);
        }
        return res;
    }
    if (complex_type(sd) && complex_type(dd)) {
        v_setcomplex(dt, res, v_complex(st, sp));
        return res;
    }
    if (dk == static_cast<std::int64_t>(kind::String) && dd->gp_kind != kind::Foreign) {
        gp::string out;
        if (signed_type(sd)) {
            out = gp::rune_to_string(v_int(st, sp));
        } else if (unsigned_type(sd)) {
            out = gp::rune_to_string(v_uint(st, sp));
        } else if (byte_or_rune_slice(sd, kind::Uint8)) {
            out = gp::to_string(*static_cast<const gp::slice<std::uint8_t>*>(addr(sp)));
        } else if (byte_or_rune_slice(sd, kind::Int32)) {
            out = gp::to_string(*static_cast<const gp::slice<std::int32_t>*>(addr(sp)));
        } else {
            out = v_string(st, sp);
        }
        *static_cast<gp::string*>(addr(res)) = std::move(out);
        return res;
    }
    if (sk == static_cast<std::int64_t>(kind::String) && dd->gp_kind == kind::Slice) {
        gp::string s = v_string(st, sp);
        if (byte_or_rune_slice(dd, kind::Uint8)) {
            *static_cast<gp::slice<std::uint8_t>*>(addr(res)) = gp::to_bytes(s);
        } else {
            *static_cast<gp::slice<std::int32_t>*>(addr(res)) = gp::to_runes(s);
        }
        return res;
    }
    if (sd->gp_kind == kind::Slice && dd->gp_kind == kind::Array) {
        seq_view v = view_of(sd, sp);
        if (v.len < dd->gp_len) {
            fail("reflect: cannot convert slice with length " + std::to_string(v.len) +
                 " to array with length " + std::to_string(dd->gp_len));
        }
        char* out = static_cast<char*>(const_cast<void*>(dd->gp_ops->gp_seq_data(addr(res))));
        copy_elems(full(dd)->gp_elem, out, v.data, dd->gp_len);
        return res;
    }
    if (sd->gp_kind == kind::Slice && dd->gp_kind == kind::Pointer) {
        seq_view v = view_of(sd, sp);
        const type_desc* arr = full(dd)->gp_elem;
        if (v.len < arr->gp_len) {
            fail("reflect: cannot convert slice with length " + std::to_string(v.len) +
                 " to array pointer with length " + std::to_string(arr->gp_len));
        }
        const ptr_refl* r = static_cast<const ptr_refl*>(dd->gp_ops->gp_refl);
        r->gp_make(addr(res), v.owner, v.data);
        return res;
    }
    // Types of one underlying type (and unnamed pointers to such): the same object, as the target
    // type's C++ class (identical layout).
    if ((sd->gp_kind == kind::Foreign || dd->gp_kind == kind::Foreign) && sd != dd) {
        unsupported(dd, "Convert");
    }
    if (dk == sk) {
        assign(dd, addr(res), addr(sp));
        return res;
    }
    fail(std::string("reflect.Value.Convert: value of type ") + type_text(sd) +
         " cannot be converted to type " + type_text(dd));
}

namespace {

// go_fast_swap: the element type is one for which Go's Swapper indexes a typed slice (a size of 1,
// 2, 4 or 8 bytes, a pointer, a string), so an index out of range is a runtime error with the index
// and the length; other element types get Go's own text, "reflect: slice index out of range".
bool go_fast_swap(const type_desc* e) noexcept {
    switch (kind_of(e)) {
    case static_cast<std::int64_t>(kind::Array):
    case static_cast<std::int64_t>(kind::Struct):
    case static_cast<std::int64_t>(kind::Interface):
    case static_cast<std::int64_t>(kind::Slice):
    case static_cast<std::int64_t>(kind::Complex128):
        return false;
    default:
        return true;
    }
}

} // namespace

gp::func<void(std::int64_t, std::int64_t)> swapper(const gp::iface& slice) {
    const type_desc* d = slice.gp_type();
    if (d == nullptr || d->gp_kind != kind::Slice) {
        fail("reflect: Swapper of a value that is not a slice");
    }
    vptr p = std::get<1>(unpack(slice));
    seq_view v = view_of(d, p);
    const type_desc* e = full(d)->gp_elem;
    const std::int64_t n = v.len;
    char* data = v.data;
    const std::size_t size = e->gp_size;
    std::shared_ptr<void> keep = v.owner;
    const bool fast = go_fast_swap(e);
    auto oob = [fast, n](std::int64_t i, std::int64_t j) {
        if (fast) {
            gp::check_index(i, n);
            gp::check_index(j, n);
        }
        fail("reflect: slice index out of range");
    };
    if (n == 0) {
        return gp::func<void(std::int64_t, std::int64_t)>(
            [](std::int64_t, std::int64_t) { fail("reflect: slice index out of range"); });
    }
    if (n == 1) {
        return gp::func<void(std::int64_t, std::int64_t)>([](std::int64_t i, std::int64_t j) {
            if (i != 0 || j != 0) {
                fail("reflect: slice index out of range");
            }
        });
    }
    const lifetime l = lifetime_of(e);
    if (e->gp_kind != kind::Interface && l.trivial) {
        // Plain bytes: swap through a buffer on the stack (a big element through the heap).
        return gp::func<void(std::int64_t, std::int64_t)>(
            [keep, data, n, size, oob](std::int64_t i, std::int64_t j) {
                if (static_cast<std::uint64_t>(i) >= static_cast<std::uint64_t>(n) ||
                    static_cast<std::uint64_t>(j) >= static_cast<std::uint64_t>(n)) {
                    oob(i, j);
                }
                if (i == j) {
                    return;
                }
                char* a = data + static_cast<std::size_t>(i) * size;
                char* b = data + static_cast<std::size_t>(j) * size;
                if (size <= 256) {
                    alignas(std::max_align_t) char tmp[256];
                    std::memcpy(tmp, a, size);
                    std::memcpy(a, b, size);
                    std::memcpy(b, tmp, size);
                } else {
                    std::unique_ptr<char[]> tmp(new char[size]);
                    std::memcpy(tmp.get(), a, size);
                    std::memcpy(a, b, size);
                    std::memcpy(b, tmp.get(), size);
                }
            });
    }
    // Other element types: through a temporary object of the type, made with its own operations.
    return gp::func<void(std::int64_t, std::int64_t)>(
        [keep, data, n, size, e, oob](std::int64_t i, std::int64_t j) {
            if (static_cast<std::uint64_t>(i) >= static_cast<std::uint64_t>(n) ||
                static_cast<std::uint64_t>(j) >= static_cast<std::uint64_t>(n)) {
                oob(i, j);
            }
            if (i == j) {
                return;
            }
            char* a = data + static_cast<std::size_t>(i) * size;
            char* b = data + static_cast<std::size_t>(j) * size;
            vptr tmp = new_storage(e);
            assign(e, addr(tmp), a);
            assign(e, a, b);
            assign(e, b, addr(tmp));
        });
}

} // namespace gp::reflect
