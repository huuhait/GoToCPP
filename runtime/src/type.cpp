// The parts of type descriptors (gocpp/type.hpp) and interface values (gocpp/iface.hpp) that
// are not templates: the interning table of composite descriptors, the names that Go's %T
// gives to composite types, the itab builder and the itab cache. Standard C++ only.
//
// Both tables are append-only and open-addressed, and are read without a lock, as Go's itab
// table is: an entry is made completely, then published with a release store into its slot;
// a reader loads the slot with acquire and sees the whole entry. Writers take one mutex. A
// table that fills up (3/4) is replaced by one twice as big and the old one is kept (never
// freed), so a reader that is still probing it is safe; nothing is ever removed. Descriptors
// and itabs are never freed, so a pointer to one is valid for the rest of the program.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <atomic>
#include <cstring>
#include <mutex>
#include <new>
#include <string>
#include <string_view>
#include <vector>

namespace gp {
namespace detail {
namespace {

// append_table: pointers to immutable entries, found by their hash and a test.
class append_table {
public:
    // find returns the entry for which eq(entry) holds, among those with hash h, or null.
    // Lock free.
    template <typename Eq>
    const void* find(std::size_t h, Eq&& eq) const noexcept {
        const arr* a = cur_.load(std::memory_order_acquire);
        if (a == nullptr) {
            return nullptr;
        }
        const std::size_t mask = a->cap - 1;
        for (std::size_t i = h & mask;; i = (i + 1) & mask) {
            const void* e = a->slots[i].load(std::memory_order_acquire);
            if (e == nullptr) {
                return nullptr; // there is always an empty slot: the table is at most 3/4 full
            }
            if (eq(e)) {
                return e;
            }
        }
    }

    // insert adds e, which has the hash h and isn't in the table yet. The caller holds the
    // mutex of the table's writers.
    void insert(std::size_t h, const void* e) {
        arr* a = cur_.load(std::memory_order_relaxed);
        if (a == nullptr || (a->count + 1) * 4 > a->cap * 3) {
            a = grow(a);
        }
        put(a, h, e);
        a->count++;
    }

    std::size_t size() const noexcept {
        const arr* a = cur_.load(std::memory_order_acquire);
        return a == nullptr ? 0 : a->count; // racy by nature: for tests
    }

private:
    struct arr {
        std::size_t cap;
        std::size_t count;
        std::atomic<const void*>* slots;
        std::size_t* hashes; // read only by the writers (under the mutex)
    };

    static void put(arr* a, std::size_t h, const void* e) noexcept {
        const std::size_t mask = a->cap - 1;
        std::size_t i = h & mask;
        while (a->slots[i].load(std::memory_order_relaxed) != nullptr) {
            i = (i + 1) & mask;
        }
        a->hashes[i] = h;
        a->slots[i].store(e, std::memory_order_release);
    }

    arr* grow(arr* old) {
        auto* a = new arr;
        a->cap = old == nullptr ? 16 : old->cap * 2;
        a->count = 0;
        a->slots = new std::atomic<const void*>[a->cap];
        a->hashes = new std::size_t[a->cap];
        for (std::size_t i = 0; i < a->cap; i++) {
            a->slots[i].store(nullptr, std::memory_order_relaxed);
            a->hashes[i] = 0;
        }
        if (old != nullptr) {
            for (std::size_t i = 0; i < old->cap; i++) {
                const void* e = old->slots[i].load(std::memory_order_relaxed);
                if (e != nullptr) {
                    put(a, old->hashes[i], e);
                    a->count++;
                }
            }
            retired_.push_back(old); // a reader may still be probing it: never freed
        }
        cur_.store(a, std::memory_order_release);
        return a;
    }

    std::atomic<arr*> cur_{nullptr};
    std::vector<arr*> retired_;
};

// state is everything the tables need, on the heap and never deleted: no static destructor
// can run at exit while a thread uses a table, and LeakSanitizer doesn't report it (the
// static variable in state() still points at it).
struct state {
    std::mutex mu;
    append_table descs;
    append_table itabs;
    std::atomic<std::size_t> itab_builds{0};
};

state& st() {
    static state* const s = new state;
    return *s;
}

// --- strings ---

bool same_str(const char* a, const char* b) noexcept {
    if (a == nullptr || b == nullptr) {
        return a == b;
    }
    return std::strcmp(a, b) == 0;
}

std::size_t hash_str(const char* s) noexcept {
    return s == nullptr ? 0 : hash_bytes(s, std::strlen(s));
}

// copy_str: a copy of s that is never freed (reachable from the descriptor that holds it).
const char* copy_str(const std::string& s) {
    char* p = new char[s.size() + 1];
    std::memcpy(p, s.c_str(), s.size() + 1);
    return p;
}

// quote is a struct tag as Go's reflect writes it: strconv.Quote, for ASCII (other bytes are
// kept as they are).
void quote(std::string& out, const char* s) {
    constexpr const char* hex = "0123456789abcdef";
    out += '"';
    for (const char* p = s; *p != '\0'; p++) {
        auto c = static_cast<unsigned char>(*p);
        switch (c) {
        case '"':
            out += "\\\"";
            break;
        case '\\':
            out += "\\\\";
            break;
        case '\a':
            out += "\\a";
            break;
        case '\b':
            out += "\\b";
            break;
        case '\f':
            out += "\\f";
            break;
        case '\n':
            out += "\\n";
            break;
        case '\r':
            out += "\\r";
            break;
        case '\t':
            out += "\\t";
            break;
        case '\v':
            out += "\\v";
            break;
        default:
            if (c < 0x20 || c == 0x7f) {
                out += "\\x";
                out += hex[c >> 4];
                out += hex[c & 0xf];
            } else {
                out += static_cast<char>(c);
            }
        }
    }
    out += '"';
}

// --- the order of methods ---

// method_order compares two methods by (exported first, name, package path), byte order.
int method_order(const char* name1, const char* pkg1, const char* name2,
                 const char* pkg2) noexcept {
    const bool unexported1 = pkg1 != nullptr;
    const bool unexported2 = pkg2 != nullptr;
    if (unexported1 != unexported2) {
        return unexported1 ? 1 : -1;
    }
    if (int c = std::strcmp(name1, name2); c != 0) {
        return c < 0 ? -1 : 1;
    }
    if (!unexported1) {
        return 0;
    }
    int c = std::strcmp(pkg1, pkg2);
    return c < 0 ? -1 : (c > 0 ? 1 : 0);
}

// --- the names of composite types ---

// signature_text is the text of an interface method's type for the type's name: "func(int)
// string" is shown as "(int) string".
std::string_view signature_text(const char* sig) noexcept {
    std::string_view s = sig;
    if (s.starts_with("func")) {
        s.remove_prefix(4);
    }
    return s;
}

std::string pkg_name_of(const imethod& m) {
    if (m.gp_pkgname != nullptr) {
        return m.gp_pkgname;
    }
    std::string_view p = m.gp_pkg;
    if (auto i = p.rfind('/'); i != std::string_view::npos) {
        p.remove_prefix(i + 1);
    }
    return std::string(p);
}

// compose is the name that Go's reflect gives to the composite type d, built from the names of
// its parts.
std::string compose(const type_desc& d) {
    std::string s;
    switch (d.gp_kind) {
    case kind::Array:
        s = "[" + std::to_string(d.gp_len) + "]" + d.gp_elem->gp_name;
        break;
    case kind::Chan:
        switch (d.gp_dir) {
        case chan_dir::recv:
            s = std::string("<-chan ") + d.gp_elem->gp_name;
            break;
        case chan_dir::send:
            s = std::string("chan<- ") + d.gp_elem->gp_name;
            break;
        default:
            // "<-" binds to the leftmost chan it can: chan (<-chan int) needs the parentheses.
            if (d.gp_elem->gp_name[0] == '<') {
                s = std::string("chan (") + d.gp_elem->gp_name + ")";
            } else {
                s = std::string("chan ") + d.gp_elem->gp_name;
            }
        }
        break;
    case kind::Func:
        s = "func(";
        for (std::size_t i = 0; i < d.gp_nin; i++) {
            if (i > 0) {
                s += ", ";
            }
            const type_desc* p = d.gp_in[i];
            if (d.gp_variadic && i + 1 == d.gp_nin && p->gp_kind == kind::Slice) {
                s += "...";
                s += p->gp_elem->gp_name;
            } else {
                s += p->gp_name;
            }
        }
        s += ")";
        if (d.gp_nout == 1) {
            s += " ";
            s += d.gp_out[0]->gp_name;
        } else if (d.gp_nout > 1) {
            s += " (";
            for (std::size_t i = 0; i < d.gp_nout; i++) {
                if (i > 0) {
                    s += ", ";
                }
                s += d.gp_out[i]->gp_name;
            }
            s += ")";
        }
        break;
    case kind::Interface:
        if (d.gp_nimethods == 0) {
            s = "interface {}";
            break;
        }
        s = "interface { ";
        for (std::size_t i = 0; i < d.gp_nimethods; i++) {
            const imethod& m = d.gp_imethods[i];
            if (i > 0) {
                s += "; ";
            }
            if (m.gp_pkg != nullptr) {
                s += pkg_name_of(m);
                s += '.';
            }
            s += m.gp_name;
            s += signature_text(m.gp_sig);
        }
        s += " }";
        break;
    case kind::Map:
        s = std::string("map[") + d.gp_key->gp_name + "]" + d.gp_elem->gp_name;
        break;
    case kind::Pointer:
        s = std::string("*") + d.gp_elem->gp_name;
        break;
    case kind::Slice:
        s = std::string("[]") + d.gp_elem->gp_name;
        break;
    case kind::Struct:
        if (d.gp_nfields == 0) {
            s = "struct {}";
            break;
        }
        s = "struct { ";
        for (std::size_t i = 0; i < d.gp_nfields; i++) {
            const field_desc& f = d.gp_fields[i];
            if (i > 0) {
                s += "; ";
            }
            if (!f.gp_embedded) {
                s += f.gp_name;
                s += ' ';
            }
            s += f.gp_type->gp_name;
            if (f.gp_tag != nullptr && f.gp_tag[0] != '\0') {
                s += ' ';
                quote(s, f.gp_tag);
            }
        }
        s += " }";
        break;
    default:
        s = d.gp_name;
    }
    return s;
}

// --- interning ---

bool same_shape(const type_desc& a, const type_desc& b) noexcept {
    if (a.gp_kind != b.gp_kind) {
        return false;
    }
    switch (a.gp_kind) {
    case kind::Array:
        return a.gp_elem == b.gp_elem && a.gp_len == b.gp_len;
    case kind::Chan:
        return a.gp_elem == b.gp_elem && a.gp_dir == b.gp_dir;
    case kind::Func:
        if (a.gp_variadic != b.gp_variadic || a.gp_nin != b.gp_nin || a.gp_nout != b.gp_nout) {
            return false;
        }
        for (std::size_t i = 0; i < a.gp_nin; i++) {
            if (a.gp_in[i] != b.gp_in[i]) {
                return false;
            }
        }
        for (std::size_t i = 0; i < a.gp_nout; i++) {
            if (a.gp_out[i] != b.gp_out[i]) {
                return false;
            }
        }
        return true;
    case kind::Interface:
        if (a.gp_nimethods != b.gp_nimethods) {
            return false;
        }
        for (std::size_t i = 0; i < a.gp_nimethods; i++) {
            const imethod& x = a.gp_imethods[i];
            const imethod& y = b.gp_imethods[i];
            if (!same_str(x.gp_name, y.gp_name) || !same_str(x.gp_pkg, y.gp_pkg) ||
                !same_str(x.gp_sig, y.gp_sig)) {
                return false;
            }
        }
        return true;
    case kind::Map:
        return a.gp_key == b.gp_key && a.gp_elem == b.gp_elem;
    case kind::Pointer:
    case kind::Slice:
        return a.gp_elem == b.gp_elem;
    case kind::Struct:
        if (a.gp_nfields != b.gp_nfields) {
            return false;
        }
        for (std::size_t i = 0; i < a.gp_nfields; i++) {
            const field_desc& x = a.gp_fields[i];
            const field_desc& y = b.gp_fields[i];
            if (!same_str(x.gp_name, y.gp_name) || !same_str(x.gp_pkg, y.gp_pkg) ||
                x.gp_type != y.gp_type || !same_str(x.gp_tag, y.gp_tag) ||
                x.gp_embedded != y.gp_embedded) {
                return false;
            }
        }
        return true;
    case kind::Opaque:
        return same_str(a.gp_name, b.gp_name);
    default:
        return false;
    }
}

std::size_t ptr_hash(const void* p) noexcept {
    return hash_int(static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(p)));
}

std::size_t shape_hash(const type_desc& d) noexcept {
    std::size_t h = hash_int(static_cast<uint64_t>(d.gp_kind));
    auto add = [&h](std::size_t x) { h = hash_combine(h, x); };
    switch (d.gp_kind) {
    case kind::Array:
        add(ptr_hash(d.gp_elem));
        add(hash_int(static_cast<uint64_t>(d.gp_len)));
        break;
    case kind::Chan:
        add(ptr_hash(d.gp_elem));
        add(hash_int(static_cast<uint64_t>(d.gp_dir)));
        break;
    case kind::Func:
        add(hash_int(d.gp_variadic ? 1 : 0));
        add(hash_int(d.gp_nin));
        for (std::size_t i = 0; i < d.gp_nin; i++) {
            add(ptr_hash(d.gp_in[i]));
        }
        add(hash_int(d.gp_nout));
        for (std::size_t i = 0; i < d.gp_nout; i++) {
            add(ptr_hash(d.gp_out[i]));
        }
        break;
    case kind::Interface:
        for (std::size_t i = 0; i < d.gp_nimethods; i++) {
            add(hash_str(d.gp_imethods[i].gp_name));
            add(hash_str(d.gp_imethods[i].gp_pkg));
            add(hash_str(d.gp_imethods[i].gp_sig));
        }
        break;
    case kind::Map:
        add(ptr_hash(d.gp_key));
        add(ptr_hash(d.gp_elem));
        break;
    case kind::Pointer:
    case kind::Slice:
        add(ptr_hash(d.gp_elem));
        break;
    case kind::Struct:
        for (std::size_t i = 0; i < d.gp_nfields; i++) {
            const field_desc& f = d.gp_fields[i];
            add(hash_str(f.gp_name));
            add(hash_str(f.gp_pkg));
            add(ptr_hash(f.gp_type));
            add(hash_str(f.gp_tag));
            add(hash_int(f.gp_embedded ? 1 : 0));
        }
        break;
    case kind::Opaque:
        add(hash_str(d.gp_name));
        break;
    default:
        break;
    }
    return h;
}

template <typename T>
T* copy_array(const T* src, std::size_t n) {
    if (n == 0) {
        return nullptr;
    }
    T* dst = static_cast<T*>(::operator new(n * sizeof(T)));
    for (std::size_t i = 0; i < n; i++) {
        new (dst + i) T(src[i]);
    }
    return dst;
}

// make_desc makes the descriptor for proto: a copy that owns its arrays and its name.
const type_desc* make_desc(const type_desc& proto) {
    auto* d = new type_desc(proto);
    d->gp_in = copy_array(proto.gp_in, proto.gp_nin);
    d->gp_out = copy_array(proto.gp_out, proto.gp_nout);
    d->gp_fields = copy_array(proto.gp_fields, proto.gp_nfields);
    d->gp_imethods = copy_array(proto.gp_imethods, proto.gp_nimethods);
    if (proto.gp_kind == kind::Opaque) {
        d->gp_name = copy_str(proto.gp_name);
    } else if (proto.gp_kind == kind::Interface && proto.gp_name != nullptr &&
               proto.gp_name[0] != '\0') {
        d->gp_name = copy_str(proto.gp_name); // the emitter's text of an unnamed interface type
    } else {
        d->gp_name = copy_str(compose(*d));
    }
    return d;
}

} // namespace

const type_desc* weak_intern(const type_desc& proto, const type_desc* elem) {
    static std::mutex mu;
    static std::vector<std::pair<const type_desc*, const type_desc*>> table;
    std::lock_guard<std::mutex> lock(mu);
    for (const auto& e : table) {
        if (e.first == elem) {
            return e.second;
        }
    }
    auto* d = new type_desc(proto);
    d->gp_name = copy_str(std::string("weak.Pointer[") + elem->gp_name + "]");
    d->gp_pkg = "weak";
    d->gp_named = true;
    d->gp_elem = elem; // (the T of weak.Pointer[T]; fmt's %#v names it)
    table.emplace_back(elem, d);
    return d;
}

const type_desc* intern(const type_desc& proto) {
    switch (proto.gp_kind) {
    case kind::Array:
    case kind::Chan:
    case kind::Func:
    case kind::Interface:
    case kind::Map:
    case kind::Pointer:
    case kind::Slice:
    case kind::Struct:
    case kind::Opaque:
        break;
    default:
        fatal("gocpp: internal error: a type descriptor of this kind is not interned");
    }
    state& s = st();
    const std::size_t h = shape_hash(proto);
    auto eq = [&proto](const void* e) {
        return same_shape(*static_cast<const type_desc*>(e), proto);
    };
    if (const void* e = s.descs.find(h, eq)) {
        return static_cast<const type_desc*>(e);
    }
    std::lock_guard<std::mutex> lock(s.mu);
    if (const void* e = s.descs.find(h, eq)) {
        return static_cast<const type_desc*>(e);
    }
    const type_desc* d = make_desc(proto);
    s.descs.insert(h, d);
    return d;
}

std::size_t interned_count() noexcept {
    return st().descs.size();
}

std::string cxx_name(const std::type_info& t) {
    // g++ starts the name of a type that is local to a file with '*'.
    std::string name = t.name();
    if (!name.empty() && name[0] == '*') {
        name.erase(0, 1);
    }
    std::string d = os::demangle(name);
    return d.empty() ? name : d;
}

} // namespace detail

bool methods_sorted(const method* methods, std::size_t n) noexcept {
    for (std::size_t i = 1; i < n; i++) {
        if (detail::method_order(methods[i - 1].gp_name, methods[i - 1].gp_pkg, methods[i].gp_name,
                                 methods[i].gp_pkg) >= 0) {
            return false;
        }
    }
    return true;
}

bool imethods_sorted(const imethod* methods, std::size_t n) noexcept {
    for (std::size_t i = 1; i < n; i++) {
        if (detail::method_order(methods[i - 1].gp_name, methods[i - 1].gp_pkg, methods[i].gp_name,
                                 methods[i].gp_pkg) >= 0) {
            return false;
        }
    }
    return true;
}

const type_desc* td_interface(std::initializer_list<imethod> methods, const char* name) {
    if (methods.size() == 0) {
        return &td_any;
    }
    if (!imethods_sorted(methods.begin(), methods.size())) {
        detail::fatal("gocpp: internal error: the methods of an interface type are not sorted "
                      "(exported names first, then by name and package: gocpp/type.hpp)");
    }
    type_desc d = shape_interface(methods.begin(), methods.size());
    d.gp_size = detail::iface_size;
    d.gp_align = detail::iface_align;
    d.gp_name = name;
    const type_desc* t = detail::intern(d);
    if (name != nullptr && std::strcmp(t->gp_name, name) != 0) {
        detail::fatal(std::string("gocpp: internal error: the interface type ") + t->gp_name +
                      " is also given the name " + name + " (every site must pass the same name)");
    }
    return t;
}

const type_desc* td_opaque(const char* spelling) {
    type_desc d = shape_basic(kind::Opaque);
    d.gp_name = spelling;
    return detail::intern(d);
}

// --- itabs ---

namespace detail {
namespace {

std::size_t itab_hash(const type_desc* inter, const type_desc* type) noexcept {
    return hash_combine(ptr_hash(inter),
                        static_cast<std::size_t>(reinterpret_cast<std::uintptr_t>(type)));
}

// build makes the itab of (inter, type), a negative one when type lacks a method of inter.
// The two method tables are sorted, so one walk over both matches them: for each method of
// the interface, in order, the walk moves over the type's methods that sort before it; the
// next one must be the same method (name, package and signature text) or the type lacks it.
const itab* build(const type_desc* inter, const type_desc* type) {
    const bool inter_sorted = imethods_sorted(inter->gp_imethods, inter->gp_nimethods);
    if (!inter_sorted || !methods_sorted(type->gp_methods, type->gp_nmethods)) {
        std::string msg = "gocpp: internal error: the methods of ";
        msg += inter_sorted ? type->gp_name : inter->gp_name;
        msg += " are not sorted (exported names first, then by name and package: gocpp/type.hpp)";
        fatal(msg);
    }
    const std::size_t n = inter->gp_nimethods;
    auto* t =
        static_cast<itab*>(::operator new(sizeof(itab) + n * sizeof(fn_ptr) + n * sizeof(bool)));
    auto* fun = reinterpret_cast<fn_ptr*>(reinterpret_cast<char*>(t) + sizeof(itab));
    auto* aware = reinterpret_cast<bool*>(fun + n);
    const imethod* missing = nullptr;
    std::size_t j = 0;
    for (std::size_t i = 0; i < n && missing == nullptr; i++) {
        const imethod& im = inter->gp_imethods[i];
        bool found = false;
        for (; j < type->gp_nmethods; j++) {
            const method& m = type->gp_methods[j];
            const int c = method_order(m.gp_name, m.gp_pkg, im.gp_name, im.gp_pkg);
            if (c < 0) {
                continue;
            }
            if (c == 0 && same_str(m.gp_sig, im.gp_sig) && m.gp_fn != nullptr) {
                fun[i] = m.gp_fn;
                aware[i] = m.gp_aware;
                found = true;
                j++;
            }
            break;
        }
        if (!found) {
            missing = &im;
        }
    }
    new (t) itab{inter, type, missing, missing == nullptr ? fun : nullptr, aware};
    return t;
}

// lookup finds or builds the itab of (inter, type), positive or negative.
const itab* lookup(const type_desc* inter, const type_desc* type) noexcept {
    if (inter == nullptr || type == nullptr || inter->gp_kind != kind::Interface ||
        type->gp_kind == kind::Interface) [[unlikely]] {
        fatal("gocpp: internal error: getitab needs an interface type and a concrete type");
    }
    state& s = st();
    const std::size_t h = itab_hash(inter, type);
    auto eq = [inter, type](const void* e) {
        const itab* t = static_cast<const itab*>(e);
        return t->gp_inter == inter && t->gp_type == type;
    };
    if (const void* e = s.itabs.find(h, eq)) {
        return static_cast<const itab*>(e);
    }
    std::lock_guard<std::mutex> lock(s.mu);
    if (const void* e = s.itabs.find(h, eq)) {
        return static_cast<const itab*>(e);
    }
    const itab* t = build(inter, type);
    s.itabs.insert(h, t);
    s.itab_builds.fetch_add(1, std::memory_order_relaxed);
    return t;
}

} // namespace

const itab* itab_entry(const type_desc* inter, const type_desc* type) noexcept {
    return lookup(inter, type);
}

std::size_t itab_builds() noexcept {
    return st().itab_builds.load(std::memory_order_relaxed);
}

std::size_t itab_cached() noexcept {
    return st().itabs.size();
}

void bad_conversion(const type_desc* inter, const type_desc* type) noexcept {
    std::string msg = "gocpp: internal error: ";
    msg += type != nullptr ? type->gp_name : "<nil>";
    msg += " does not implement ";
    msg += inter->gp_name;
    if (const imethod* m = type != nullptr ? missing_method(inter, type) : nullptr) {
        msg += ": missing method ";
        msg += m->gp_name;
    }
    fatal(msg);
}

void bad_unbox() noexcept {
    fatal("gocpp: internal error: unbox of the nil interface");
}

} // namespace detail

const itab* getitab(const type_desc* inter, const type_desc* type) noexcept {
    if (inter != nullptr && inter->gp_nimethods == 0) {
        return nullptr;
    }
    const itab* t = detail::lookup(inter, type);
    return t->gp_missing == nullptr ? t : nullptr;
}

const imethod* missing_method(const type_desc* inter, const type_desc* type) noexcept {
    if (inter != nullptr && inter->gp_nimethods == 0) {
        return nullptr;
    }
    return detail::lookup(inter, type)->gp_missing;
}

bool implements(const type_desc* inter, const type_desc* type) noexcept {
    return missing_method(inter, type) == nullptr;
}

} // namespace gp
