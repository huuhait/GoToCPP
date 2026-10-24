// The stack trace of an unrecovered panic (gocpp/trace.hpp): Go names from mangled C++ names,
// the DWARF line table, and the frames in Go's format. Standard C++ only: the addresses, the
// program's file and demangling come from the OS layer (gocpp/os.hpp).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <atomic>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <new>
#include <string>
#include <string_view>
#include <unordered_map>
#include <utility>
#include <vector>

namespace gp {

namespace {

// walk_all records the whole stack of the calling thread into all (innermost first, up to
// panic_error::gp_max_walk frames) and reports whether it got to its end. The buffer grows 8
// times each try, so the walks cost O(depth) in all. It throws std::bad_alloc without memory.
bool walk_all(std::vector<uintptr_t>& all) {
    for (std::size_t size = 1024; size <= panic_error::gp_max_walk; size *= 8) {
        all.resize(size);
        int n = os::stack(all.data(), static_cast<int>(size));
        if (n >= 0 && static_cast<std::size_t>(n) < size) {
            all.resize(static_cast<std::size_t>(n));
            return true;
        }
    }
    return false;
}

} // namespace

panic_error::panic_error(gp::string text, panic_identity id) noexcept
    : panic_error(text.gp_view().empty() ? gp::any() : detail::runtime_error_value(text), text,
                  false, id) {}

panic_error::panic_error(gp::any value, gp::string text, bool lazy, panic_identity id) noexcept
    : gp_text(std::move(text)), gp_value(std::move(value)), gp_lazy(lazy), gp_id(id) {
    gp_record();
}

panic_error::panic_error(detail::iface_t, const gp::iface& v, panic_identity id,
                         detail::stack_t how) noexcept
    : gp_value(v), gp_lazy(true), gp_id(id) {
    if (how == detail::stack_t::record) {
        gp_record();
    }
}

panic_error::panic_error(detail::string_t, const gp::string& v, panic_identity id,
                         detail::stack_t how) noexcept
    : gp_text(v), gp_value(gp::to_iface<gp::any>(v, &td_string)), gp_id(id) {
    if (how == detail::stack_t::record) {
        gp_record();
    }
}

panic_error::panic_error(detail::runtime_t, detail::rt_error kind, const char* lit, std::size_t n,
                         panic_identity id, detail::stack_t how) noexcept
    : gp_text(gp::static_bytes, lit, static_cast<int64_t>(n)),
      gp_value(detail::runtime_error_value(kind, gp_text)), gp_id(id) {
    if (how == detail::stack_t::record) {
        gp_record();
    }
}

namespace {

// walk_above is walk_all for the frames outside the frame that holds the address bound.
bool walk_above(std::vector<uintptr_t>& all, const void* bound) {
    for (std::size_t size = 1024; size <= panic_error::gp_max_walk; size *= 8) {
        all.resize(size);
        int n = os::stack_above(all.data(), static_cast<int>(size), bound);
        if (n >= 0 && static_cast<std::size_t>(n) < size) {
            all.resize(static_cast<std::size_t>(n));
            return true;
        }
    }
    return false;
}

// innermost_defer returns an address in the frame of the innermost function of this thread that
// has a defer_frame and is still running, or null.
__attribute__((noinline)) const void* innermost_defer() noexcept {
    const detail::defer_link* d = detail::top_defer;
    // A listed frame below this function's frame is dead (a longjmp over it, or a stack that
    // was not unwound): it must not end a walk.
    auto here = reinterpret_cast<uintptr_t>(__builtin_frame_address(0));
    while (d != nullptr && reinterpret_cast<uintptr_t>(d) <= here) {
        d = d->gp_next();
    }
    return d;
}

} // namespace

void panic_error::gp_record() noexcept {
    const void* bound = detail::defer_links_listed ? innermost_defer() : nullptr;
    if (bound != nullptr) {
        bool found = false;
        gp_npcs = os::stack_until(gp_pcs, gp_max_trace, bound, &found);
        if (found) {
            gp_bound = reinterpret_cast<uintptr_t>(bound);
            return;
        }
    } else {
        gp_npcs = os::stack(gp_pcs, gp_max_trace);
    }
    gp_walk_deep();
}

void panic_error::gp_walk_deep() noexcept {
    if (gp_npcs < gp_max_trace) {
        return;
    }
    // A deep stack: walk it again to its end, for Go's outer frames and the count of the frames
    // left out. Only a deep stack pays for it.
    try {
        auto all = std::make_shared<std::vector<uintptr_t>>();
        if (walk_all(*all)) {
            all->shrink_to_fit();
            gp_all = std::move(all);
        }
    } catch (...) {
        // No memory: the trace shows the inner frames only.
    }
}

void panic_error::gp_record_all() noexcept {
    gp_bound = 0;
    gp_all.reset();
    gp_npcs = os::stack(gp_pcs, gp_max_trace);
    gp_walk_deep();
}

void panic_error::gp_complete() noexcept {
    if (gp_bound == 0) {
        return;
    }
    int room = gp_max_trace - gp_npcs;
    uintptr_t rest[gp_max_trace];
    int n = os::stack_above(rest, room, reinterpret_cast<const void*>(gp_bound));
    if (n < 0) {
        return; // the frame that ends the inner part is not on this stack: the panic stays as it is
    }
    if (n < room) {
        // The stack ends within gp_pcs.
        for (int i = 0; i < n; i++) {
            gp_pcs[gp_npcs + i] = rest[i];
        }
        gp_npcs += n;
        gp_bound = 0;
        return;
    }
    // A deep stack: all of it, as gp_walk_deep records it.
    try {
        auto all = std::make_shared<std::vector<uintptr_t>>(gp_pcs, gp_pcs + gp_npcs);
        std::vector<uintptr_t> outer;
        if (walk_above(outer, reinterpret_cast<const void*>(gp_bound))) {
            all->insert(all->end(), outer.begin(), outer.end());
            all->shrink_to_fit();
            for (int i = gp_npcs; i < gp_max_trace; i++) {
                gp_pcs[i] = (*all)[static_cast<std::size_t>(i)];
            }
            gp_npcs = gp_max_trace;
            gp_all = std::move(all);
            gp_bound = 0;
        }
    } catch (...) {
        // No memory: the trace shows the inner frames only.
    }
}

namespace detail {

namespace {

// --- Go names from mangled C++ names ---

// A comp is one part of a C++ qualified name.
struct comp {
    enum kind_t { source, lambda, call_op, other } kind = other;
    std::string text;   // a source name
    bool targs = false; // template arguments follow it
    int lambda_n = 0;   // a lambda's number, 1 for the first; 0 when unknown
    int targ_n = 0;     // the template argument when it is one int, <N> (from 1); else 0
};

struct encoding;

// A name: the parts of a qualified name, and for a local name (a lambda in a function) the
// function it is in.
struct name_t {
    std::unique_ptr<encoding> outer;
    std::vector<comp> comps;
    bool konst = false; // a nested name with the const qualifier: a const member function
};

// An encoding: a function's name, and whether it has parameters.
struct encoding {
    name_t name;
    bool params = false;
};

// A small parser for the Itanium C++ ABI's mangled names (itanium-cxx-abi.github.io). It knows
// enough of the grammar to find the parts of the names of gocpp's functions and to skip the
// types of their parameters (also of templates, as gocpp's runtime types are); anything else
// sets bad, and the caller shows the demangled C++ name instead.
class demangler {
public:
    explicit demangler(std::string_view s) : s_(s) {}

    bool bad = false;

    // top parses the whole symbol: _Z <encoding> [.clone suffix].
    bool top(encoding& e) {
        if (!eat("_Z")) {
            return false;
        }
        parse_encoding(e, false);
        return !bad && (at_end() || peek() == '.');
    }

private:
    std::string_view s_;
    std::size_t i_ = 0;
    int depth_ = 0;

    bool at_end() const { return i_ >= s_.size(); }
    char peek(std::size_t k = 0) const { return i_ + k < s_.size() ? s_[i_ + k] : '\0'; }
    bool eat(std::string_view p) {
        if (s_.substr(i_, p.size()) == p) {
            i_ += p.size();
            return true;
        }
        return false;
    }
    void fail() {
        bad = true;
        i_ = s_.size();
    }
    static bool digit(char c) { return c >= '0' && c <= '9'; }
    static bool lower(char c) { return c >= 'a' && c <= 'z'; }

    // number: a decimal number (n for a negative one, in literals).
    uint64_t number() {
        if (!digit(peek())) {
            fail();
            return 0;
        }
        uint64_t n = 0;
        while (digit(peek())) {
            n = n * 10 + static_cast<uint64_t>(peek() - '0');
            if (n > (uint64_t(1) << 32)) {
                fail();
                return 0;
            }
            i_++;
        }
        return n;
    }

    // seq_id: the [0-9A-Z]* before the _ of a substitution or template parameter.
    void seq_id() {
        while ((digit(peek()) || (peek() >= 'A' && peek() <= 'Z'))) {
            i_++;
        }
        if (!eat("_")) {
            fail();
        }
    }

    std::string source_name() {
        uint64_t n = number();
        if (bad || n > s_.size() - i_) {
            fail();
            return "";
        }
        std::string r(s_.substr(i_, static_cast<std::size_t>(n)));
        i_ += static_cast<std::size_t>(n);
        return r;
    }

    // The encoding of a function: its name, then for a template its return type, then its
    // parameter types. inner: inside a local name, where the parameters end at an E.
    void parse_encoding(encoding& e, bool inner) {
        if (++depth_ > 32) {
            fail();
            return;
        }
        parse_name(e.name);
        if (bad) {
            return;
        }
        auto end = [&] { return at_end() || peek() == '.' || (inner && peek() == 'E'); };
        if (end()) {
            depth_--;
            return; // not a function (a variable)
        }
        if (!e.name.comps.empty() && e.name.comps.back().targs) {
            type(); // a function template's return type
        }
        if (peek() == 'v' &&
            (i_ + 1 == s_.size() || s_[i_ + 1] == '.' || (inner && s_[i_ + 1] == 'E'))) {
            i_++; // no parameters
        } else {
            while (!bad && !end()) {
                type();
                e.params = true;
            }
        }
        depth_--;
    }

    void parse_name(name_t& n) {
        if (peek() == 'N') {
            for (std::size_t j = i_ + 1; j < s_.size() && std::strchr("rVK", s_[j]) != nullptr;
                 j++) {
                n.konst = n.konst || s_[j] == 'K';
            }
            nested(n.comps);
        } else if (peek() == 'Z') {
            local(n);
        } else if (eat("St")) {
            n.comps.push_back(comp{comp::source, "std"});
            unqualified(n.comps);
            if (peek() == 'I') {
                template_args();
                n.comps.back().targs = true;
            }
        } else if (peek() == 'S') {
            fail(); // a substitution: not in gocpp's names
        } else {
            eat("L"); // internal linkage
            unqualified(n.comps);
            if (peek() == 'I') {
                template_args();
                n.comps.back().targs = true;
            }
        }
    }

    // Z <function encoding> E <entity name> [<discriminator>]
    void local(name_t& n) {
        i_++; // Z
        n.outer = std::make_unique<encoding>();
        parse_encoding(*n.outer, true);
        if (bad || !eat("E")) {
            fail();
            return;
        }
        if (peek() == 's' || peek() == 'd') {
            fail(); // a string literal, a default argument
            return;
        }
        if (peek() == 'Z') {
            fail(); // not a valid entity
            return;
        }
        parse_name(n);
        // The discriminator of an entity that has the same name as another one there.
        if (eat("__")) {
            number();
            eat("_");
        } else if (peek() == '_' && digit(peek(1))) {
            i_++;
            i_++;
        }
    }

    // N [CV-qualifiers] [ref-qualifier] <prefix>... E
    void nested(std::vector<comp>& comps) {
        i_++; // N
        while (peek() == 'r' || peek() == 'V' || peek() == 'K') {
            i_++;
        }
        if (peek() == 'R' || peek() == 'O') {
            i_++;
        }
        while (!bad && !eat("E")) {
            if (at_end()) {
                fail();
                return;
            }
            if (eat("St")) {
                comps.push_back(comp{comp::source, "std"});
                continue;
            }
            if (peek() == 'S') {
                i_++;
                seq_id(); // a substitution
                comps.push_back(comp{comp::other, "?"});
                continue;
            }
            if (peek() == 'I') {
                if (comps.empty()) {
                    fail();
                    return;
                }
                comps.back().targ_n = template_args();
                comps.back().targs = true;
                continue;
            }
            if (peek() == 'M') {
                i_++; // the closure of a variable's initializer: the variable is the prefix
                continue;
            }
            unqualified(comps);
        }
    }

    void unqualified(std::vector<comp>& comps) {
        eat("L");
        char c = peek();
        if (digit(c)) {
            comp n{comp::source, source_name()};
            if (n.text.starts_with("$_")) {
                // clang++'s name for a lambda in a function that isn't inline: numbered in
                // the whole file, so its number in the function is unknown (lambda_n 0).
                n.kind = comp::lambda;
            }
            comps.push_back(n);
        } else if (eat("Ul")) {
            // A lambda: Ul <template parameters> <parameter types> E [<number>] _; the first one
            // is #1. A template parameter of a lambda template is Ty (a type), Tn <type> (a
            // value: gocpp's <int = N>) or Tp <parameter> (a pack).
            while (!bad && !eat("E")) {
                if (at_end()) {
                    fail();
                    return;
                }
                while (eat("Tp")) {
                }
                if (eat("Ty")) {
                    continue;
                }
                if (eat("Tn")) {
                    type();
                    continue;
                }
                type();
            }
            int k = 1;
            if (digit(peek())) {
                k = static_cast<int>(number()) + 2;
            }
            if (!eat("_")) {
                fail();
                return;
            }
            comp l{comp::lambda, ""};
            l.lambda_n = k;
            comps.push_back(l);
        } else if (eat("Ut")) {
            if (digit(peek())) {
                number();
            }
            if (!eat("_")) {
                fail();
            }
            comps.push_back(comp{comp::other, "?"});
        } else if (c == 'C' && (digit(peek(1)) || peek(1) == 'I')) {
            i_ += 2; // a constructor
            if (s_[i_ - 1] == 'I') {
                i_++;
                type();
            }
            comps.push_back(comp{comp::other, "?"});
        } else if (c == 'D' && digit(peek(1))) {
            i_ += 2; // a destructor
            comps.push_back(comp{comp::other, "?"});
        } else if (lower(c) && lower(peek(1))) {
            // An operator name; cl is operator(), the call operator of a lambda.
            bool cl = c == 'c' && peek(1) == 'l';
            bool cv = c == 'c' && peek(1) == 'v';
            bool li = c == 'l' && peek(1) == 'i';
            i_ += 2;
            if (cv) {
                type();
            } else if (li) {
                source_name();
            }
            comps.push_back(comp{cl ? comp::call_op : comp::other, cl ? "()" : "?"});
        } else {
            fail();
            return;
        }
        while (!bad && eat("B")) {
            source_name(); // an ABI tag
        }
    }

    // template_args skips template arguments, I <arg>... E. It returns N when they are one
    // positive int literal, ILi<N>EE (gocpp's numbers of func literals), else 0.
    int template_args() {
        std::size_t start = i_;
        i_++; // I
        while (!bad && !eat("E")) {
            if (at_end()) {
                fail();
                return 0;
            }
            template_arg();
        }
        std::string_view a = s_.substr(start, i_ - start);
        if (bad || !a.starts_with("ILi") || !a.ends_with("EE") || a.size() < 6 || a[3] == '0') {
            return 0;
        }
        int n = 0;
        for (char c : a.substr(3, a.size() - 5)) {
            if (!digit(c) || n > 100000000) {
                return 0;
            }
            n = n * 10 + (c - '0');
        }
        return n;
    }

    // template_arg counts the depth like type: a pack (J) holds template arguments.
    void template_arg() {
        if (++depth_ > 64) {
            fail();
            return;
        }
        template_arg_body();
        depth_--;
    }

    void template_arg_body() {
        if (eat("L")) {
            // A literal: L <type> <value> E; L_Z <encoding> E is an external name.
            if (peek() == '_') {
                fail();
                return;
            }
            type();
            while (!bad && !at_end() && peek() != 'E') {
                i_++;
            }
            if (!eat("E")) {
                fail();
            }
        } else if (eat("J")) {
            while (!bad && !eat("E")) {
                if (at_end()) {
                    fail();
                    return;
                }
                template_arg();
            }
        } else if (peek() == 'X') {
            fail(); // an expression
        } else {
            type();
        }
    }

    // type skips one type.
    void type() {
        if (++depth_ > 64) {
            fail();
            return;
        }
        char c = peek();
        if (c != '\0' && std::strchr("vwbcahstijlmxynofdegz", c) != nullptr) {
            i_++;
        } else if (c == 'u') {
            i_++;
            source_name();
        } else if (c == 'D') {
            char d = peek(1);
            i_ += 2;
            if (d == 'p') {
                type();
            } else if (d == 'F' || d == 'B' || d == 'U') {
                number();
                eat("x");
                if (!eat("_")) {
                    fail();
                }
            } else if (d == '\0' || std::strchr("defhisuacn", d) == nullptr) {
                fail();
            }
        } else if (c == 'r' || c == 'V' || c == 'K' || c == 'P' || c == 'R' || c == 'O') {
            i_++;
            type();
        } else if (c == 'N') {
            std::vector<comp> ignored;
            nested(ignored);
        } else if (c == 'Z') {
            name_t ignored;
            local(ignored);
        } else if (c == 'S') {
            i_++;
            char d = peek();
            if (d == 't') {
                i_++;
                std::vector<comp> ignored;
                unqualified(ignored);
            } else if (d != '\0' && std::strchr("absiod", d) != nullptr) {
                i_++;
            } else {
                seq_id();
            }
            if (peek() == 'I') {
                template_args();
            }
        } else if (c == 'T') {
            i_++;
            seq_id();
            if (peek() == 'I') {
                template_args();
            }
        } else if (c == 'A') {
            i_++;
            if (digit(peek())) {
                number();
            }
            if (!eat("_")) {
                fail();
                return;
            }
            type();
        } else if (c == 'F') {
            i_++;
            eat("Y");
            while (!bad && !eat("E")) {
                if (at_end()) {
                    fail();
                    return;
                }
                if ((peek() == 'R' || peek() == 'O') && peek(1) == 'E') {
                    i_++;
                    continue;
                }
                type();
            }
        } else if (c == 'M') {
            i_++;
            type();
            type();
        } else if (digit(c)) {
            source_name();
            if (peek() == 'I') {
                template_args();
            }
        } else {
            fail();
        }
        depth_--;
    }
};

// decode_package turns the rest of a gpkg_ namespace back into the import path (ADR-010).
bool decode_package(std::string_view s, std::string& path) {
    path.clear();
    for (std::size_t i = 0; i < s.size(); i++) {
        char c = s[i];
        if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
            path += c;
            continue;
        }
        if (c != '_' || i + 1 >= s.size()) {
            return false;
        }
        char code = s[++i];
        switch (code) {
        case '_':
            path += '/';
            break;
        case 'd':
            path += '.';
            break;
        case 'h':
            path += '-';
            break;
        case 'u':
            path += '_';
            break;
        case 't':
            path += '~';
            break;
        case 'p':
            path += '+';
            break;
        case 'x': {
            if (i + 2 >= s.size()) {
                return false;
            }
            auto hex = [](char h) {
                return h >= '0' && h <= '9' ? h - '0' : h >= 'a' && h <= 'f' ? h - 'a' + 10 : -1;
            };
            int hi = hex(s[i + 1]), lo = hex(s[i + 2]);
            if (hi < 0 || lo < 0) {
                return false;
            }
            path += static_cast<char>(hi * 16 + lo);
            i += 2;
            break;
        }
        default:
            return false;
        }
    }
    return !path.empty();
}

// A gocpp name: the Go name of a function, or why there is none.
struct go_name {
    enum { gocpp, runtime, other } kind = other;
    std::string name;
    bool closure = false; // a func literal: a func literal in it is .N, not .funcN
};

// gocpp_name finds the Go name of the function e.
go_name gocpp_name(const encoding& e) {
    go_name r;
    const std::vector<comp>& cs = e.name.comps;
    if (e.name.outer != nullptr) {
        // A local entity: a lambda's operator() is a func literal of the function it is in.
        go_name outer = gocpp_name(*e.name.outer);
        if (outer.kind != go_name::gocpp) {
            r.kind = outer.kind;
            return r;
        }
        if (cs.size() == 2 && cs[0].kind == comp::lambda && cs[1].kind == comp::call_op) {
            // A func literal is a lambda template, [n]<int = N>(...) (internal/lower's
            // closures.go): N is its number in the function, as Go numbers them, where the
            // compilers' own numbers of lambdas differ. A generic lambda (auto parameters) is
            // gocpp's own helper (std::apply's callable, a deferred builtin), never a Go func
            // literal.
            int n = cs[0].lambda_n;
            if (cs[1].targs) {
                if (cs[1].targ_n == 0) {
                    r.kind = go_name::runtime;
                    return r;
                }
                n = cs[1].targ_n;
            }
            r.kind = go_name::gocpp;
            r.name =
                outer.name + (outer.closure ? "." : ".func") + (n > 0 ? std::to_string(n) : "?");
            r.closure = true;
            return r;
        }
        return r; // something else in a function: the C++ name
    }
    if (cs.empty() || cs[0].kind != comp::source) {
        return r;
    }
    const std::string& first = cs[0].text;
    if (first == "gp" || first == "std" || first.starts_with("__") ||
        (cs.size() == 1 && first.starts_with("gp_"))) {
        r.kind = go_name::runtime; // the runtime, the C++ library, gocpp's gp_init_all
        return r;
    }
    std::string path;
    if (!first.starts_with("gpkg_") || !decode_package(std::string_view(first).substr(5), path)) {
        return r;
    }
    if (cs.size() < 2) {
        return r;
    }
    r.name = path;
    // A method (step 3.10) is a member of its type's struct, gpkg_main::T::M: a value method is a
    // const member function (main.T.M), a pointer method a static one (main.(*T).M), whose
    // mangled name has no qualifier.
    bool ptr_method = cs.size() == 3 && !e.name.konst;
    for (std::size_t k = 1; k < cs.size(); k++) {
        const comp& c = cs[k];
        if (c.kind != comp::source) {
            return go_name{}; // a constructor, an operator, ...: the C++ name
        }
        std::string part = c.text;
        if (part.starts_with("gp_")) {
            // gp_inst_<hash>_F: the hash is 8 hex digits, 10 or 12 when another instance had
            // the same 8.
            std::size_t hex = 0;
            if (part.starts_with("gp_inst_")) {
                while (8 + hex < part.size() && ((part[8 + hex] >= '0' && part[8 + hex] <= '9') ||
                                                 (part[8 + hex] >= 'a' && part[8 + hex] <= 'f'))) {
                    hex++;
                }
            }
            if ((hex == 8 || hex == 10 || hex == 12) && part.size() > 9 + hex &&
                part[8 + hex] == '_') {
                // An instance of a generic function (internal/lower/generics.go), gp_inst_<hash>_F:
                // Go's main.F[...].
                part = part.substr(9 + hex) + "[...]"; // (before the (*...) of a pointer method)
            } else if (part.ends_with("_")) {
                part.pop_back(); // a user name that starts with gp_ (ADR-010)
            } else if (k + 1 == cs.size() && part == "gp_var_init") {
                part = "init"; // Go initializes the package variables in main.init
            } else if (k + 1 == cs.size() && part.starts_with("gp_init_") && part.size() > 8 &&
                       part.size() <= 17 && part[8] != '0' &&
                       std::all_of(part.begin() + 8, part.end(),
                                   [](char d) { return d >= '0' && d <= '9'; })) {
                // gp_init_1 is Go's main.init.0.
                part = "init." + std::to_string(std::stoll(part.substr(8)) - 1);
            } else {
                r.kind = go_name::runtime; // a generated helper (gp_init_funcs)
                return r;
            }
        }
        r.name += '.';
        if (ptr_method && k == 1) {
            part = "(*" + part + ")";
        }
        r.name += part;
        if (c.targs) {
            r.name += "[...]";
        }
    }
    r.kind = go_name::gocpp;
    return r;
}

} // namespace

namespace {

// has_wrap_tag reports whether the mangled name carries the abi tag gp_wrap, B7gp_wrap, which the
// wrapper methods of the interface structs have. The name is walked as the Itanium ABI writes it,
// each <length><name> skipped whole, so a user name that contains the text (a method called
// B7gp_wrap, mangled 9B7gp_wrap) is no tag.
bool has_wrap_tag(std::string_view sym) {
    if (!sym.starts_with("_Z")) {
        return false;
    }
    auto number = [&](std::size_t& i) {
        std::size_t n = 0;
        while (i < sym.size() && sym[i] >= '0' && sym[i] <= '9') {
            n = n * 10 + static_cast<std::size_t>(sym[i] - '0');
            if (n > sym.size()) {
                return sym.size() + 1;
            }
            i++;
        }
        return n;
    };
    for (std::size_t i = 2; i < sym.size();) {
        char c = sym[i];
        if (c >= '0' && c <= '9') {
            std::size_t n = number(i);
            if (i + n > sym.size()) {
                return false;
            }
            i += n; // a source name
        } else if (c == 'B') {
            i++;
            std::size_t n = number(i);
            if (i + n > sym.size()) {
                return false;
            }
            if (sym.substr(i, n) == "gp_wrap") {
                return true;
            }
            i += n;
        } else if ((c == 'S' || c == 'T') && i + 1 < sym.size() &&
                   (sym[i + 1] == '_' || (sym[i + 1] >= '0' && sym[i + 1] <= '9') ||
                    (sym[i + 1] >= 'A' && sym[i + 1] <= 'Z'))) {
            // A substitution or a template parameter, S3_ or T_: its digits are no length (St,
            // Sa, Ss... are letters in lower case and fall through).
            while (i < sym.size() && sym[i] != '_') {
                i++;
            }
            i++;
        } else {
            i++;
        }
    }
    return false;
}

} // namespace

go_func func_name(std::string_view symbol) {
    go_func f;
    std::string_view base = symbol.substr(0, symbol.find('.')); // without .cold, .constprop.0
    if (has_wrap_tag(base)) {
        // An interface's wrapper method, tagged [[gnu::abi_tag("gp_wrap")]] (internal/lower/
        // ifaces.go): Go has no frame for it, as for a thunk.
        return f;
    }
    if (!symbol.starts_with("_Z")) {
        // A C name. Names that start with _ are the implementation's (_start, __libc_start_main,
        // __cxa_throw, _Unwind_RaiseException).
        f.name = std::string(base);
        f.shown = !base.empty() && base[0] != '_';
        f.raw = true;
        f.params = true;
        return f;
    }
    encoding e;
    demangler d(symbol);
    f.params = true; // unless the parser finds there are none
    if (d.top(e)) {
        // A value method's receiver is a parameter in Go, *this in C++.
        f.params = e.params || (e.name.konst && e.name.outer == nullptr);
        go_name g = gocpp_name(e);
        if (g.kind == go_name::runtime) {
            return f; // not shown
        }
        if (g.kind == go_name::gocpp) {
            f.name = std::move(g.name);
            f.shown = true;
            return f;
        }
    } else if (base.starts_with("_ZN2gp") || base.starts_with("_ZNK2gp") ||
               base.starts_with("_ZZN2gp") || base.starts_with("_ZZNK2gp") ||
               base.starts_with("_ZNSt") || base.starts_with("_ZNKSt") ||
               base.starts_with("_ZSt") || base.starts_with("_ZZNSt") ||
               base.starts_with("_ZN9__gnu_cxx") || base.starts_with("_ZNK9__gnu_cxx")) {
        return f; // the runtime's or the C++ library's, in a form the parser doesn't know
    }
    // Another C++ function (the user's own C++, Phase 04): its C++ name.
    f.name = os::demangle(std::string(base));
    if (f.name.empty()) {
        f.name = std::string(base);
    }
    f.shown = true;
    f.raw = true;
    return f;
}

// --- the DWARF line table ---

namespace {

// A reader of little-endian DWARF data that never reads outside its bytes: past the end, it
// sets bad and gives zeros.
struct reader {
    const unsigned char* p;
    const unsigned char* end;
    bool bad = false;

    std::size_t left() const { return static_cast<std::size_t>(end - p); }

    uint64_t fixed(std::size_t n) {
        if (left() < n) {
            bad = true;
            p = end;
            return 0;
        }
        uint64_t v = 0;
        for (std::size_t k = 0; k < n; k++) {
            v |= static_cast<uint64_t>(p[k]) << (8 * k);
        }
        p += n;
        return v;
    }
    uint64_t uleb() {
        uint64_t v = 0;
        for (int shift = 0;; shift += 7) {
            if (p >= end) {
                bad = true;
                return 0;
            }
            unsigned char b = *p++;
            if (shift < 64) {
                v |= static_cast<uint64_t>(b & 0x7f) << shift;
            }
            if ((b & 0x80) == 0) {
                return v;
            }
        }
    }
    int64_t sleb() {
        uint64_t v = 0;
        int shift = 0;
        unsigned char b = 0;
        do {
            if (p >= end) {
                bad = true;
                return 0;
            }
            b = *p++;
            if (shift < 64) {
                v |= static_cast<uint64_t>(b & 0x7f) << shift;
            }
            shift += 7;
        } while ((b & 0x80) != 0);
        if (shift < 64 && (b & 0x40) != 0) {
            v |= ~uint64_t(0) << shift;
        }
        return static_cast<int64_t>(v);
    }
    std::string_view cstr() {
        const void* nul = std::memchr(p, 0, left());
        if (nul == nullptr) {
            bad = true;
            p = end;
            return {};
        }
        std::string_view s(reinterpret_cast<const char*>(p),
                           static_cast<std::size_t>(static_cast<const unsigned char*>(nul) - p));
        p += s.size() + 1;
        return s;
    }
    void skip(uint64_t n) {
        if (left() < n) {
            bad = true;
            p = end;
            return;
        }
        p += n;
    }
};

// str_at is the NUL-terminated string at off in a string section.
std::string_view str_at(std::string_view sec, uint64_t off) {
    if (off >= sec.size()) {
        return {};
    }
    std::string_view s = sec.substr(static_cast<std::size_t>(off));
    return s.substr(0, s.find('\0'));
}

// table_entry sets at to base + index * width, the offset of an entry of width bytes of a table
// in a section of size bytes, when the whole entry is in the section; false when it is not, also
// when the sum would wrap (the numbers come from the file, which may be damaged).
bool table_entry(uint64_t base, uint64_t index, uint64_t width, std::size_t size, uint64_t& at) {
    if (width == 0 || base > size || index > (size - base) / width ||
        size - base - index * width < width) {
        return false;
    }
    at = base + index * width;
    return true;
}

bool absolute(std::string_view p) {
    return p.starts_with('/') || p.starts_with('\\') ||
           (p.size() > 2 && p[1] == ':' && (p[2] == '/' || p[2] == '\\'));
}

// The DWARF constants that the line table uses.
enum : uint64_t {
    form_block2 = 0x03,
    form_block4 = 0x04,
    form_data2 = 0x05,
    form_data4 = 0x06,
    form_data8 = 0x07,
    form_string = 0x08,
    form_block = 0x09,
    form_block1 = 0x0a,
    form_data1 = 0x0b,
    form_sdata = 0x0d,
    form_strp = 0x0e,
    form_udata = 0x0f,
    form_data16 = 0x1e,
    form_line_strp = 0x1f,
    lnct_path = 1,
    lnct_directory_index = 2,
};

struct entry {
    std::string_view path;
    uint64_t dir = 0;
};

// read_entries reads a DWARF 5 directory or file name table.
bool read_entries(reader& r, bool dw64, std::string_view line_str, std::string_view str,
                  std::vector<entry>& out) {
    std::size_t nformats = static_cast<std::size_t>(r.fixed(1));
    std::vector<std::pair<uint64_t, uint64_t>> formats;
    for (std::size_t k = 0; k < nformats && !r.bad; k++) {
        uint64_t type = r.uleb();
        uint64_t form = r.uleb();
        formats.emplace_back(type, form);
    }
    uint64_t count = r.uleb();
    if (r.bad || count > r.left()) {
        return false;
    }
    for (uint64_t k = 0; k < count && !r.bad; k++) {
        entry e;
        for (auto [type, form] : formats) {
            std::string_view s;
            uint64_t v = 0;
            switch (form) {
            case form_string:
                s = r.cstr();
                break;
            case form_line_strp:
                s = str_at(line_str, r.fixed(dw64 ? 8 : 4));
                break;
            case form_strp:
                s = str_at(str, r.fixed(dw64 ? 8 : 4));
                break;
            case form_udata:
                v = r.uleb();
                break;
            case form_sdata:
                r.sleb();
                break;
            case form_data1:
                v = r.fixed(1);
                break;
            case form_data2:
                v = r.fixed(2);
                break;
            case form_data4:
                v = r.fixed(4);
                break;
            case form_data8:
                v = r.fixed(8);
                break;
            case form_data16:
                r.skip(16);
                break;
            case form_block:
                r.skip(r.uleb());
                break;
            case form_block1:
                r.skip(r.fixed(1));
                break;
            case form_block2:
                r.skip(r.fixed(2));
                break;
            case form_block4:
                r.skip(r.fixed(4));
                break;
            default:
                return false; // a form the line table doesn't use (strx needs .debug_str_offsets)
            }
            if (type == lnct_path) {
                e.path = s;
            } else if (type == lnct_directory_index) {
                e.dir = v;
            }
        }
        out.push_back(e);
    }
    return !r.bad;
}

// A line_collector keeps every row of the line table instead of answering queries: the ranges of
// addresses [lo, hi) of the table, in the order of the units, with the line and the file (an index
// in files). indexed_lines_of builds one once and answers from it.
struct line_span {
    uint64_t lo = 0, hi = 0;
    int64_t line = 0;
    uint32_t file = 0;
};
struct line_collector {
    std::vector<line_span> ranges;
    std::vector<std::string> files;
    std::unordered_map<std::string, uint32_t> ids;
};

// The header of a unit of the line table: what its line program needs, and its file names.
struct line_header {
    uint64_t min_inst = 1, line_range = 0, opcode_base = 0;
    int8_t line_base = 0;
    std::vector<uint64_t> std_lengths;
    std::vector<entry> dirs, files;
    uint64_t file_base = 1; // DWARF 2-4 count files from 1, DWARF 5 from 0
    const unsigned char* program = nullptr;

    // The path of the file numbered file, as the trace prints it ("" for a bad number).
    std::string path(uint64_t file) const {
        std::string p;
        if (file < file_base || file - file_base >= files.size()) {
            return p;
        }
        const entry& f = files[static_cast<std::size_t>(file - file_base)];
        if (absolute(f.path) || f.dir >= dirs.size()) {
            p = std::string(f.path);
        } else {
            std::string_view dir = dirs[static_cast<std::size_t>(f.dir)].path;
            if (!absolute(dir) && f.dir != 0 && !dirs.empty() && !dirs[0].path.empty()) {
                p += dirs[0].path;
                p += '/';
            }
            if (!dir.empty()) {
                p += dir;
                p += '/';
            }
            p += f.path;
        }
#ifdef _WIN32
        // Go prints file names of Windows programs with slashes (C:/work/main.go); MinGW's DWARF
        // has the backslashes of the native paths.
        std::replace(p.begin(), p.end(), '\\', '/');
#endif
        // A file of the standard library is $GOCPP/stdlib/... in the compile's work folder, which
        // the object cache maps to /gocpp-work (objcache.go). g++'s link-time compile (-flto)
        // writes it relative to the link's folder instead, with the random work folder of the
        // compile in it: the trace shows it as the other builds do.
        if (std::size_t at = p.find("/$GOCPP/");
            at != std::string::npos && !p.starts_with("/gocpp-work/$GOCPP/")) {
            p = "/gocpp-work" + p.substr(at);
        }
        return p;
    }
};

// read_line_header reads the header of a unit of the line table from u, which holds the unit
// after its length; u is then at the line program. False for a version or a header it can't read.
bool read_line_header(reader& u, bool dw64, std::string_view line_str, std::string_view str,
                      line_header& h) {
    uint64_t version = u.fixed(2);
    if (version < 2 || version > 5) {
        return false;
    }
    if (version >= 5) {
        u.fixed(1); // address_size
        u.fixed(1); // segment_selector_size
    }
    uint64_t header_length = u.fixed(dw64 ? 8 : 4);
    if (u.bad || header_length > u.left()) {
        return false;
    }
    h.program = u.p + header_length;
    h.min_inst = u.fixed(1);
    if (version >= 4) {
        u.fixed(1); // maximum_operations_per_instruction: 1 except on VLIW
    }
    u.fixed(1); // default_is_stmt
    h.line_base = static_cast<int8_t>(u.fixed(1));
    h.line_range = u.fixed(1);
    h.opcode_base = u.fixed(1);
    if (u.bad || h.line_range == 0 || h.opcode_base == 0) {
        return false;
    }
    h.std_lengths.assign(static_cast<std::size_t>(h.opcode_base), 0);
    for (uint64_t k = 1; k < h.opcode_base; k++) {
        h.std_lengths[static_cast<std::size_t>(k)] = u.fixed(1);
    }
    if (version >= 5) {
        h.file_base = 0;
        if (!read_entries(u, dw64, line_str, str, h.dirs) ||
            !read_entries(u, dw64, line_str, str, h.files)) {
            return false;
        }
    } else {
        h.dirs.push_back(entry{}); // the compilation directory, which only .debug_info has
        for (;;) {
            std::string_view d = u.cstr();
            if (u.bad || d.empty()) {
                break;
            }
            h.dirs.push_back(entry{d, 0});
        }
        for (;;) {
            std::string_view f = u.cstr();
            if (u.bad || f.empty()) {
                break;
            }
            entry e{f, u.uleb()};
            u.uleb(); // mtime
            u.uleb(); // length
            h.files.push_back(e);
        }
    }
    if (u.bad || h.program > u.end) {
        return false;
    }
    u.p = h.program;
    return true;
}

// read_unit_length reads the length of a unit of a DWARF section (and whether it is the 64-bit
// form) and returns the reader of the unit's body, moving r past the unit. False when it is bad.
bool read_unit_length(reader& r, reader& unit, bool& dw64) {
    dw64 = false;
    uint64_t length = r.fixed(4);
    if (length == 0xffffffff) {
        dw64 = true;
        length = r.fixed(8);
    } else if (length >= 0xfffffff0) {
        r.bad = true;
        return false;
    }
    if (r.bad || length > r.left()) {
        r.bad = true;
        return false;
    }
    unit = reader{r.p, r.p + length};
    r.p += length; // the next unit
    return true;
}

// One unit of the line table: its header and its line program.
// qs are the queries, sorted by address; with col, nothing is asked and every row is kept in col.
void line_unit(reader& r, std::string_view line_str, std::string_view str,
               const std::vector<line_query*>& qs, line_collector* col) {
    bool dw64 = false;
    reader u{nullptr, nullptr};
    if (!read_unit_length(r, u, dw64)) {
        return;
    }
    line_header h;
    if (!read_line_header(u, dw64, line_str, str, h)) {
        return;
    }
    const uint64_t min_inst = h.min_inst, line_range = h.line_range, opcode_base = h.opcode_base;
    const int8_t line_base = h.line_base;
    const std::vector<uint64_t>& std_lengths = h.std_lengths;
    auto path_of = [&](uint64_t file) { return h.path(file); };

    // The state machine. A row covers [its address, the next row's address) in its sequence;
    // prev is the last row. A sequence whose address is set to 0 (or to a tombstone, ~0 or
    // ~0 - 1) is the code of a discarded duplicate (an inline function that another unit
    // kept): it is skipped.
    uint64_t addr = 0, file = 1;
    int64_t line = 1;
    bool have_prev = false, skip_seq = false;
    uint64_t prev_addr = 0, prev_file = 0;
    int64_t prev_line = 0;
    std::unordered_map<uint64_t, uint32_t> file_ids; // the files of this unit, by number, for col
    auto row = [&](bool end_seq) {
        if (!skip_seq && have_prev && prev_addr < addr && col != nullptr) {
            auto it = file_ids.find(prev_file);
            if (it == file_ids.end()) {
                auto [id, fresh] = col->ids.try_emplace(path_of(prev_file), 0);
                if (fresh) {
                    id->second = static_cast<uint32_t>(col->files.size());
                    col->files.push_back(id->first);
                }
                it = file_ids.emplace(prev_file, id->second).first;
            }
            col->ranges.push_back(line_span{prev_addr, addr, prev_line, it->second});
        } else if (!skip_seq && have_prev && prev_addr < addr) {
            // The queries in [prev_addr, addr): a binary search, so a row costs log(queries).
            auto k = std::lower_bound(qs.begin(), qs.end(), prev_addr,
                                      [](const line_query* q, uint64_t a) { return q->addr < a; });
            for (; k != qs.end() && (*k)->addr < addr; ++k) {
                if ((*k)->line == 0) {
                    (*k)->file = path_of(prev_file);
                    (*k)->line = prev_line;
                }
            }
        }
        have_prev = !end_seq;
        prev_addr = addr;
        prev_file = file;
        prev_line = line;
        if (end_seq) {
            addr = 0;
            file = 1;
            line = 1;
            skip_seq = false;
        }
    };
    while (!u.bad && u.p < u.end) {
        uint64_t op = u.fixed(1);
        if (op >= opcode_base) {
            uint64_t adj = op - opcode_base;
            addr += (adj / line_range) * min_inst;
            line += line_base + static_cast<int64_t>(adj % line_range);
            row(false);
            continue;
        }
        switch (op) {
        case 0: { // an extended opcode
            uint64_t len = u.uleb();
            if (u.bad || len == 0 || len > u.left()) {
                return;
            }
            const unsigned char* next = u.p + len;
            uint64_t sub = u.fixed(1);
            if (sub == 1) { // DW_LNE_end_sequence
                row(true);
            } else if (sub == 2) { // DW_LNE_set_address
                addr = u.fixed(static_cast<std::size_t>(std::min<uint64_t>(len - 1, 8)));
                skip_seq = addr == 0 || addr >= ~uint64_t(0) - 1;
            }
            u.p = next; // DW_LNE_define_file, DW_LNE_set_discriminator, ...
            break;
        }
        case 1: // DW_LNS_copy
            row(false);
            break;
        case 2: // DW_LNS_advance_pc
            addr += u.uleb() * min_inst;
            break;
        case 3: // DW_LNS_advance_line
            line += u.sleb();
            break;
        case 4: // DW_LNS_set_file
            file = u.uleb();
            break;
        case 8: // DW_LNS_const_add_pc
            addr += ((255 - opcode_base) / line_range) * min_inst;
            break;
        case 9: // DW_LNS_fixed_advance_pc
            addr += u.fixed(2);
            break;
        default: // the others take their operands as ULEB128 numbers
            for (uint64_t k = 0; k < std_lengths[static_cast<std::size_t>(op)]; k++) {
                u.uleb();
            }
            break;
        }
    }
}

} // namespace

void lines_of(std::string_view debug_line, std::string_view line_str, std::string_view str,
              line_query* queries, std::size_t n) {
    if (n == 0) {
        return;
    }
    std::vector<line_query*> sorted(n);
    for (std::size_t k = 0; k < n; k++) {
        sorted[k] = &queries[k];
    }
    std::sort(sorted.begin(), sorted.end(),
              [](const line_query* a, const line_query* b) { return a->addr < b->addr; });
    const auto* p = reinterpret_cast<const unsigned char*>(debug_line.data());
    reader r{p, p + debug_line.size()};
    while (!r.bad && r.left() > 0) {
        line_unit(r, line_str, str, sorted, nullptr);
    }
}

namespace {

// The line table of the program, read once by indexed_lines_of: the ranges sorted by address (the
// first unit wins where two overlap, as in lines_of).
struct line_index {
    const char* data = nullptr; // the section it was read from
    std::size_t size = 0;
    line_collector col;
    bool built = false;
};

} // namespace

void indexed_lines_of(std::string_view debug_line, std::string_view line_str, std::string_view str,
                      line_query* queries, std::size_t n) {
    static std::mutex mu;
    // Never freed: a leak check finds it through here. Made under the lock, not by a function-local
    // static's initializer: with the guard of that initializer ThreadSanitizer sees no edge from
    // the constructor in one goroutine to the use in another that took the lock first (a false race
    // at the first traces of two goroutines at once, in a -race program: seen in 5 of 60 runs of
    // one program, and not reproduced by a deterministic test).
    static line_index* idx = nullptr;
    std::lock_guard<std::mutex> lk(mu);
    if (idx == nullptr) {
        idx = new line_index;
    }
    if (!idx->built || idx->data != debug_line.data() || idx->size != debug_line.size()) {
        *idx = line_index{};
        idx->data = debug_line.data();
        idx->size = debug_line.size();
        const auto* p = reinterpret_cast<const unsigned char*>(debug_line.data());
        reader r{p, p + debug_line.size()};
        const std::vector<line_query*> none;
        while (!r.bad && r.left() > 0) {
            line_unit(r, line_str, str, none, &idx->col);
        }
        std::stable_sort(idx->col.ranges.begin(), idx->col.ranges.end(),
                         [](const line_span& a, const line_span& b) { return a.lo < b.lo; });
        idx->built = true;
    }
    const std::vector<line_span>& rs = idx->col.ranges;
    for (std::size_t k = 0; k < n; k++) {
        line_query& q = queries[k];
        if (q.line != 0) {
            continue;
        }
        // The last range that starts at or before the address; the ones before it that overlap it
        // (rare: the same code in two units) are looked at too.
        auto it = std::upper_bound(rs.begin(), rs.end(), q.addr,
                                   [](uint64_t a, const line_span& r) { return a < r.lo; });
        for (std::size_t back = 0; back < 4 && it != rs.begin(); back++) {
            --it;
            if (q.addr < it->hi) {
                q.file = idx->col.files[it->file];
                q.line = it->line;
                break;
            }
        }
        if (q.line > 0 || q.near_hi <= q.near_lo) {
            continue;
        }
        // No line at addr: the nearest row of the inlined call's range that has one, the last one
        // before addr, else the first one after it; a row of a .gp (or .go) file first, as the
        // range also has the code of the runtime's headers that are inlined into the call.
        for (int pass = 0; pass < 2 && q.line <= 0; pass++) {
            const line_span* before = nullptr;
            const line_span* after = nullptr;
            for (auto r = std::lower_bound(rs.begin(), rs.end(), q.near_lo,
                                           [](const line_span& s, uint64_t a) { return s.lo < a; });
                 r != rs.end() && r->lo < q.near_hi; ++r) {
                const std::string& file = idx->col.files[r->file];
                if (r->line <= 0 ||
                    (pass == 0 && !file.ends_with(".gp") && !file.ends_with(".go"))) {
                    continue;
                }
                if (r->lo <= q.addr) {
                    before = &*r;
                } else if (after == nullptr) {
                    after = &*r;
                }
            }
            if (const line_span* pick = before != nullptr ? before : after; pick != nullptr) {
                q.file = idx->col.files[pick->file];
                q.line = pick->line;
            }
        }
    }
}

// --- the inlined calls (DWARF .debug_info) ---

std::string* exe_image::dwarf_section(std::string_view name) noexcept {
    if (name == ".debug_line") {
        return &debug_line;
    }
    if (name == ".debug_line_str") {
        return &debug_line_str;
    }
    if (name == ".debug_str") {
        return &debug_str;
    }
    if constexpr (read_inlines) {
        if (name == ".debug_info") {
            return &debug_info;
        }
        if (name == ".debug_abbrev") {
            return &debug_abbrev;
        }
        if (name == ".debug_addr") {
            return &debug_addr;
        }
        if (name == ".debug_str_offsets") {
            return &debug_str_offsets;
        }
        if (name == ".debug_rnglists") {
            return &debug_rnglists;
        }
        if (name == ".debug_ranges") {
            return &debug_ranges;
        }
    }
    return nullptr;
}

namespace {

// The DWARF constants of .debug_info that the reader of the inlined calls uses.
enum : uint64_t {
    tag_inlined_subroutine = 0x1d,
    tag_subprogram = 0x2e,
    at_stmt_list = 0x10,
    at_low_pc = 0x11,
    at_high_pc = 0x12,
    at_abstract_origin = 0x31,
    at_specification = 0x47,
    at_ranges = 0x55,
    at_call_file = 0x58,
    at_call_line = 0x59,
    at_linkage_name = 0x6e,
    at_str_offsets_base = 0x72,
    at_addr_base = 0x73,
    at_rnglists_base = 0x74,
    at_mips_linkage_name = 0x2007,
    form_addr = 0x01,
    form_flag = 0x0c,
    form_ref_addr = 0x10,
    form_ref1 = 0x11,
    form_ref2 = 0x12,
    form_ref4 = 0x13,
    form_ref8 = 0x14,
    form_ref_udata = 0x15,
    form_indirect = 0x16,
    form_sec_offset = 0x17,
    form_exprloc = 0x18,
    form_flag_present = 0x19,
    form_strx = 0x1a,
    form_addrx = 0x1b,
    form_ref_sup4 = 0x1c,
    form_strp_sup = 0x1d,
    form_ref_sig8 = 0x20,
    form_implicit_const = 0x21,
    form_loclistx = 0x22,
    form_rnglistx = 0x23,
    form_ref_sup8 = 0x24,
    form_strx1 = 0x25,
    form_strx4 = 0x28,
    form_addrx1 = 0x29,
    form_addrx4 = 0x2c,
    form_gnu_addr_index = 0x1f01,
    form_gnu_str_index = 0x1f02,
    form_gnu_ref_alt = 0x1f20,
    form_gnu_strp_alt = 0x1f21,
};

// An abbreviation: a DIE's tag, whether it has children, and its attributes with their forms.
struct abbrev_attr {
    uint64_t at, form;
    int64_t implicit; // the value of DW_FORM_implicit_const
};
struct abbrev {
    uint64_t tag = 0;
    bool children = false;
    std::vector<abbrev_attr> attrs;
};
// The abbreviation table at an offset of .debug_abbrev, by code (codes are small and dense).
using abbrev_table = std::vector<abbrev>;

bool read_abbrevs(std::string_view sec, uint64_t off, abbrev_table& out) {
    if (off >= sec.size()) {
        return false;
    }
    const auto* p = reinterpret_cast<const unsigned char*>(sec.data());
    reader r{p + off, p + sec.size()};
    for (;;) {
        uint64_t code = r.uleb();
        if (r.bad) {
            return false;
        }
        if (code == 0) {
            return true;
        }
        if (code > (uint64_t(1) << 20)) {
            return false; // not the dense numbering of a compiler
        }
        abbrev a;
        a.tag = r.uleb();
        a.children = r.fixed(1) != 0;
        for (;;) {
            abbrev_attr at{r.uleb(), r.uleb(), 0};
            if (r.bad) {
                return false;
            }
            if (at.at == 0 && at.form == 0) {
                break;
            }
            if (at.form == form_implicit_const) {
                at.implicit = r.sleb();
            }
            a.attrs.push_back(at);
        }
        if (out.size() <= code) {
            out.resize(static_cast<std::size_t>(code) + 1);
        }
        out[static_cast<std::size_t>(code)] = std::move(a);
    }
}

// A unit of .debug_info: where its DIEs are, how to read them, and the bases of its DWARF 5
// indexed forms (from its unit DIE).
struct info_unit {
    uint64_t off = 0;  // of the unit's header in .debug_info
    uint64_t dies = 0; // of its first DIE
    uint64_t end = 0;  // of the next unit
    uint64_t version = 0;
    uint64_t addr_size = 8;
    bool dw64 = false;
    uint64_t abbrev_off = 0;
    uint64_t str_offsets_base = 8; // DWARF 5's default: after the header of the first table
    uint64_t addr_base = 8;
    uint64_t rnglists_base = 0;
    bool have_rnglists_base = false;
    uint64_t low_pc = 0;
    uint64_t stmt_list = ~uint64_t(0);
    uint32_t table = 0; // its abbreviation table, in inline_index::tables
};

// The value of an attribute: a number (an address, a constant, an offset, an index, a reference
// made absolute), or a string.
struct attr_value {
    uint64_t form = 0;
    uint64_t num = 0;
    std::string_view str;
    bool is_str = false;
};

// The sections that the reader of the inlined calls looks at.
struct info_sections {
    std::string_view info, abbrev, str, line_str, str_offsets, addr, rnglists, ranges, line;
};

// read_form reads the value of an attribute of the form form at r, for the unit u.
bool read_form(reader& r, uint64_t form, int64_t implicit, const info_unit& u,
               const info_sections& s, attr_value& v) {
    v = attr_value{};
    v.form = form;
    const std::size_t off_size = u.dw64 ? 8 : 4;
    for (int indirect = 0; form == form_indirect && indirect < 4; indirect++) {
        form = r.uleb();
        v.form = form;
    }
    switch (form) {
    case form_addr:
        v.num = r.fixed(static_cast<std::size_t>(u.addr_size));
        break;
    case form_data1:
    case form_ref1:
    case form_flag:
    case form_strx1:
    case form_addrx1:
        v.num = r.fixed(1);
        break;
    case form_data2:
    case form_ref2:
    case form_strx1 + 1:  // strx2
    case form_addrx1 + 1: // addrx2
        v.num = r.fixed(2);
        break;
    case form_strx1 + 2:  // strx3
    case form_addrx1 + 2: // addrx3
        v.num = r.fixed(3);
        break;
    case form_data4:
    case form_ref4:
    case form_ref_sup4:
    case form_strx4:
    case form_addrx4:
        v.num = r.fixed(4);
        break;
    case form_data8:
    case form_ref8:
    case form_ref_sig8:
    case form_ref_sup8:
        v.num = r.fixed(8);
        break;
    case form_data16:
        r.skip(16);
        break;
    case form_sdata:
        v.num = static_cast<uint64_t>(r.sleb());
        break;
    case form_udata:
    case form_ref_udata:
    case form_strx:
    case form_addrx:
    case form_loclistx:
    case form_rnglistx:
    case form_gnu_addr_index:
    case form_gnu_str_index:
        v.num = r.uleb();
        break;
    case form_string:
        v.str = r.cstr();
        v.is_str = true;
        break;
    case form_strp:
        v.str = str_at(s.str, r.fixed(off_size));
        v.is_str = true;
        break;
    case form_line_strp:
        v.str = str_at(s.line_str, r.fixed(off_size));
        v.is_str = true;
        break;
    case form_ref_addr:
        v.num = r.fixed(u.version <= 2 ? static_cast<std::size_t>(u.addr_size) : off_size);
        break;
    case form_sec_offset:
    case form_strp_sup:
    case form_gnu_ref_alt:
    case form_gnu_strp_alt:
        v.num = r.fixed(off_size);
        break;
    case form_block1:
        r.skip(r.fixed(1));
        break;
    case form_block2:
        r.skip(r.fixed(2));
        break;
    case form_block4:
        r.skip(r.fixed(4));
        break;
    case form_block:
    case form_exprloc:
        r.skip(r.uleb());
        break;
    case form_flag_present:
        v.num = 1;
        break;
    case form_implicit_const:
        v.num = static_cast<uint64_t>(implicit);
        break;
    default:
        return false; // a form this reader does not know: the unit is left
    }
    return !r.bad;
}

// The string of an attribute value, also of the indexed forms (strx).
std::string_view string_of(const attr_value& v, const info_unit& u, const info_sections& s) {
    if (v.is_str) {
        return v.str;
    }
    switch (v.form) {
    case form_strx:
    case form_strx1:
    case form_strx1 + 1:
    case form_strx1 + 2:
    case form_strx4:
    case form_gnu_str_index: {
        std::size_t w = u.dw64 ? 8 : 4;
        uint64_t at = 0;
        if (!table_entry(u.str_offsets_base, v.num, w, s.str_offsets.size(), at)) {
            return {};
        }
        const auto* p = reinterpret_cast<const unsigned char*>(s.str_offsets.data());
        reader r{p + at, p + at + w};
        return str_at(s.str, r.fixed(w));
    }
    }
    return {};
}

// The address of an attribute value, also of the indexed forms (addrx).
bool address_of(const attr_value& v, const info_unit& u, const info_sections& s, uint64_t& out) {
    switch (v.form) {
    case form_addr:
        out = v.num;
        return true;
    case form_addrx:
    case form_addrx1:
    case form_addrx1 + 1:
    case form_addrx1 + 2:
    case form_addrx4:
    case form_gnu_addr_index: {
        uint64_t at = 0;
        if (!table_entry(u.addr_base, v.num, u.addr_size, s.addr.size(), at)) {
            return false;
        }
        const auto* p = reinterpret_cast<const unsigned char*>(s.addr.data());
        reader r{p + at, p + at + u.addr_size};
        out = r.fixed(static_cast<std::size_t>(u.addr_size));
        return !r.bad;
    }
    }
    return false;
}

// The absolute .debug_info offset of a reference.
bool reference_of(const attr_value& v, const info_unit& u, uint64_t& out) {
    switch (v.form) {
    case form_ref1:
    case form_ref2:
    case form_ref4:
    case form_ref8:
    case form_ref_udata:
        out = u.off + v.num;
        return true;
    case form_ref_addr:
        out = v.num;
        return true;
    }
    return false; // a reference into another file (ref_sup, GNU_ref_alt) or a type signature
}

// ranges_of appends the address ranges [lo, hi) of a DW_AT_ranges value to out.
void ranges_of(const attr_value& v, const info_unit& u, const info_sections& s,
               std::vector<std::pair<uint64_t, uint64_t>>& out) {
    if (u.version >= 5) {
        uint64_t off = v.num;
        if (v.form == form_rnglistx) {
            if (!u.have_rnglists_base) {
                return;
            }
            std::size_t w = u.dw64 ? 8 : 4;
            uint64_t at = 0;
            if (!table_entry(u.rnglists_base, v.num, w, s.rnglists.size(), at)) {
                return;
            }
            const auto* p = reinterpret_cast<const unsigned char*>(s.rnglists.data());
            reader r{p + at, p + at + w};
            uint64_t rel = r.fixed(w); // from the base, which is in the section (table_entry)
            if (rel >= s.rnglists.size() - u.rnglists_base) {
                return;
            }
            off = u.rnglists_base + rel;
        }
        if (off >= s.rnglists.size()) {
            return;
        }
        const auto* p = reinterpret_cast<const unsigned char*>(s.rnglists.data());
        reader r{p + off, p + s.rnglists.size()};
        uint64_t base = u.low_pc;
        auto indexed = [&](uint64_t idx, uint64_t& a) {
            attr_value x;
            x.form = form_addrx;
            x.num = idx;
            return address_of(x, u, s, a);
        };
        const auto as = static_cast<std::size_t>(u.addr_size);
        for (int guard = 0; guard < 1 << 20 && !r.bad; guard++) {
            uint64_t kind = r.fixed(1);
            uint64_t a = 0, b = 0;
            switch (kind) {
            case 0: // DW_RLE_end_of_list
                return;
            case 1: // DW_RLE_base_addressx
                if (!indexed(r.uleb(), base)) {
                    return;
                }
                break;
            case 2: // DW_RLE_startx_endx
                if (!indexed(r.uleb(), a) || !indexed(r.uleb(), b)) {
                    return;
                }
                out.emplace_back(a, b);
                break;
            case 3: { // DW_RLE_startx_length
                if (!indexed(r.uleb(), a)) {
                    return;
                }
                uint64_t n = r.uleb();
                out.emplace_back(a, a + n);
                break;
            }
            case 4: // DW_RLE_offset_pair
                a = r.uleb();
                b = r.uleb();
                out.emplace_back(base + a, base + b);
                break;
            case 5: // DW_RLE_base_address
                base = r.fixed(as);
                break;
            case 6: // DW_RLE_start_end
                a = r.fixed(as);
                b = r.fixed(as);
                out.emplace_back(a, b);
                break;
            case 7: // DW_RLE_start_length
                a = r.fixed(as);
                b = r.uleb();
                out.emplace_back(a, a + b);
                break;
            default:
                return;
            }
        }
        return;
    }
    // DWARF 2-4: .debug_ranges, pairs of addresses from the unit's base, (0, 0) at the end and
    // (~0, base) setting the base.
    if (v.num >= s.ranges.size()) {
        return;
    }
    const auto* p = reinterpret_cast<const unsigned char*>(s.ranges.data());
    reader r{p + v.num, p + s.ranges.size()};
    const auto as = static_cast<std::size_t>(u.addr_size);
    const uint64_t all = as == 8 ? ~uint64_t(0) : (uint64_t(1) << (8 * as)) - 1;
    uint64_t base = u.low_pc;
    for (int guard = 0; guard < 1 << 20 && !r.bad; guard++) {
        uint64_t a = r.fixed(as), b = r.fixed(as);
        if (r.bad || (a == 0 && b == 0)) {
            return;
        }
        if (a == all) {
            base = b;
            continue;
        }
        out.emplace_back(base + a, base + b);
    }
}

// An inlined call: one range of the code of a function that the compiler inlined into another,
// the call's place, and the function inlined (the DIE of its abstract origin).
struct inline_site {
    uint64_t lo = 0, hi = 0; // the range, addresses in the file
    uint64_t origin = 0;     // .debug_info offset of the inlined function's DIE
    int64_t line = 0;        // of the call
    uint32_t file = 0;       // of the call: in inline_index::files
    uint32_t depth = 0;      // 1: inlined into the function itself; 2: into that call; ...
    uint32_t unit = 0;       // in inline_index::units
};

// What the reader keeps of the program's .debug_info (for the life of the program, as the line
// index): the inlined calls, sorted by address, and what it needs to name their functions.
struct inline_index {
    const char* data = nullptr; // the .debug_info that it was read from
    std::size_t size = 0;
    bool built = false;
    std::vector<inline_site> sites;
    std::vector<std::string> files;
    std::vector<info_unit> units; // by offset
    std::vector<abbrev_table> tables;
    std::unordered_map<uint64_t, go_func> names; // by the origin's offset
};

info_sections sections_of(const exe_image& img) {
    return info_sections{img.debug_info,     img.debug_abbrev,      img.debug_str,
                         img.debug_line_str, img.debug_str_offsets, img.debug_addr,
                         img.debug_rnglists, img.debug_ranges,      img.debug_line};
}

// read_die reads the attributes of the DIE at r (after its code) with the abbreviation a; fn is
// called with each attribute and its value. False when a value can't be read.
template <class Fn>
bool read_die(reader& r, const abbrev& a, const info_unit& u, const info_sections& s, Fn&& fn) {
    attr_value v;
    for (const abbrev_attr& at : a.attrs) {
        if (!read_form(r, at.form, at.implicit, u, s, v)) {
            return false;
        }
        fn(at.at, v);
    }
    return true;
}

// build_inlines reads every unit of .debug_info into idx.
void build_inlines(const info_sections& s, inline_index& idx) {
    const auto* base = reinterpret_cast<const unsigned char*>(s.info.data());
    reader r{base, base + s.info.size()};
    std::unordered_map<uint64_t, uint32_t> table_of; // abbreviation tables by offset
    std::unordered_map<std::string, uint32_t> file_ids;
    std::vector<std::pair<uint64_t, uint64_t>> rs;
    while (!r.bad && r.left() > 0) {
        info_unit u;
        u.off = static_cast<uint64_t>(r.p - base);
        reader body{nullptr, nullptr};
        if (!read_unit_length(r, body, u.dw64)) {
            break;
        }
        u.end = static_cast<uint64_t>(r.p - base);
        u.version = body.fixed(2);
        if (u.version < 2 || u.version > 5) {
            continue;
        }
        const std::size_t off_size = u.dw64 ? 8 : 4;
        if (u.version >= 5) {
            uint64_t type = body.fixed(1);
            u.addr_size = body.fixed(1);
            u.abbrev_off = body.fixed(off_size);
            if (type != 1 && type != 3) { // DW_UT_compile, DW_UT_partial: the others have no code
                continue;
            }
        } else {
            u.abbrev_off = body.fixed(off_size);
            u.addr_size = body.fixed(1);
        }
        if (body.bad || (u.addr_size != 4 && u.addr_size != 8)) {
            continue;
        }
        u.dies = static_cast<uint64_t>(body.p - base);
        auto t = table_of.find(u.abbrev_off);
        if (t == table_of.end()) {
            abbrev_table table;
            if (!read_abbrevs(s.abbrev, u.abbrev_off, table)) {
                continue;
            }
            t = table_of.emplace(u.abbrev_off, static_cast<uint32_t>(idx.tables.size())).first;
            idx.tables.push_back(std::move(table));
        }
        u.table = t->second;
        const abbrev_table& table = idx.tables[u.table];
        const auto unit_no = static_cast<uint32_t>(idx.units.size());

        // The unit DIE first: the bases of the indexed forms, the base address and the line table.
        uint64_t code = body.uleb();
        if (body.bad || code == 0 || code >= table.size()) {
            continue;
        }
        attr_value low{};
        bool have_low = false;
        if (!read_die(body, table[static_cast<std::size_t>(code)], u, s,
                      [&](uint64_t at, const attr_value& v) {
                          switch (at) {
                          case at_low_pc:
                              low = v;
                              have_low = true;
                              break;
                          case at_stmt_list:
                              u.stmt_list = v.num;
                              break;
                          case at_str_offsets_base:
                              u.str_offsets_base = v.num;
                              break;
                          case at_addr_base:
                              u.addr_base = v.num;
                              break;
                          case at_rnglists_base:
                              u.rnglists_base = v.num;
                              u.have_rnglists_base = true;
                              break;
                          }
                      })) {
            continue;
        }
        if (have_low) {
            address_of(low, u, s, u.low_pc);
        }
        idx.units.push_back(u);
        if (!table[static_cast<std::size_t>(code)].children) {
            continue;
        }

        // The file names of the unit's line table, for DW_AT_call_file.
        line_header lh;
        bool have_lines = false;
        if (u.stmt_list < s.line.size()) {
            const auto* lp = reinterpret_cast<const unsigned char*>(s.line.data());
            reader lr{lp + u.stmt_list, lp + s.line.size()};
            reader lu{nullptr, nullptr};
            bool ldw64 = false;
            have_lines = read_unit_length(lr, lu, ldw64) &&
                         read_line_header(lu, ldw64, s.line_str, s.str, lh);
        }
        std::unordered_map<uint64_t, uint32_t> unit_files;
        auto file_id = [&](uint64_t f) -> uint32_t {
            if (auto it = unit_files.find(f); it != unit_files.end()) {
                return it->second;
            }
            std::string path = have_lines ? lh.path(f) : std::string();
            auto [it, fresh] = file_ids.try_emplace(path, 0);
            if (fresh) {
                it->second = static_cast<uint32_t>(idx.files.size());
                idx.files.push_back(path);
            }
            unit_files.emplace(f, it->second);
            return it->second;
        };

        // The DIEs below it: the depth of inlined calls of each open DIE (0 in a function).
        std::vector<uint32_t> open{0};
        while (!body.bad && body.p < body.end && !open.empty()) {
            uint64_t c = body.uleb();
            if (c == 0) {
                open.pop_back();
                continue;
            }
            if (body.bad || c >= table.size()) {
                break;
            }
            const abbrev& a = table[static_cast<std::size_t>(c)];
            if (a.tag != tag_inlined_subroutine) {
                if (!read_die(body, a, u, s, [](uint64_t, const attr_value&) {})) {
                    break;
                }
                if (a.children) {
                    open.push_back(a.tag == tag_subprogram ? 0 : open.back());
                }
                continue;
            }
            attr_value lo{}, hi{}, ranges{}, origin{};
            bool have_lo = false, have_hi = false, have_ranges = false, have_origin = false;
            uint64_t call_file = 0;
            int64_t call_line = 0;
            if (!read_die(body, a, u, s, [&](uint64_t at, const attr_value& v) {
                    switch (at) {
                    case at_low_pc:
                        lo = v;
                        have_lo = true;
                        break;
                    case at_high_pc:
                        hi = v;
                        have_hi = true;
                        break;
                    case at_ranges:
                        ranges = v;
                        have_ranges = true;
                        break;
                    case at_abstract_origin:
                        origin = v;
                        have_origin = true;
                        break;
                    case at_call_file:
                        call_file = v.num;
                        break;
                    case at_call_line:
                        call_line = static_cast<int64_t>(v.num);
                        break;
                    }
                })) {
                break;
            }
            uint32_t depth = open.back() + 1;
            if (a.children) {
                open.push_back(depth);
            }
            rs.clear();
            uint64_t l = 0, h = 0;
            if (have_ranges) {
                ranges_of(ranges, u, s, rs);
            } else if (have_lo && address_of(lo, u, s, l) && have_hi) {
                if (address_of(hi, u, s, h)) {
                    rs.emplace_back(l, h);
                } else if (!hi.is_str) {
                    rs.emplace_back(l, l + hi.num); // DWARF 4: the size of the range
                }
            }
            // A call without its function (MinGW's g++ writes no DW_AT_abstract_origin under -flto,
            // which has no early debug information for PE objects) is kept with origin 0: it has no
            // frame, but the function around it gets the line of the call.
            uint64_t org = 0;
            if (rs.empty()) {
                continue;
            }
            if (!have_origin || !reference_of(origin, u, org)) {
                org = 0;
            }
            uint32_t fid = file_id(call_file);
            for (const auto& [x, y] : rs) {
                if (x < y && x != 0) {
                    idx.sites.push_back(inline_site{x, y, org, call_line, fid, depth, unit_no});
                }
            }
        }
    }
    std::sort(idx.sites.begin(), idx.sites.end(),
              [](const inline_site& a, const inline_site& b) { return a.lo < b.lo; });
}

// origin_name returns how the trace shows the inlined function whose DIE is at off: the Go name
// of its linkage name (DW_AT_linkage_name, on the DIE itself or on the DIE that its abstract
// origin or its specification points to). A function without one is not shown.
go_func origin_name(inline_index& idx, const info_sections& s, uint64_t off) {
    if (auto it = idx.names.find(off); it != idx.names.end()) {
        return it->second;
    }
    go_func f;
    uint64_t at_off = off;
    for (int hop = 0; hop < 4 && off != 0; hop++) {
        auto u = std::upper_bound(idx.units.begin(), idx.units.end(), at_off,
                                  [](uint64_t o, const info_unit& x) { return o < x.off; });
        if (u == idx.units.begin()) {
            break;
        }
        --u;
        if (at_off < u->dies || at_off >= u->end) {
            break;
        }
        const auto* base = reinterpret_cast<const unsigned char*>(s.info.data());
        reader r{base + at_off, base + u->end};
        const abbrev_table& table = idx.tables[u->table];
        uint64_t code = r.uleb();
        if (r.bad || code == 0 || code >= table.size()) {
            break;
        }
        std::string_view linkage;
        uint64_t next = 0;
        bool have_next = false;
        if (!read_die(r, table[static_cast<std::size_t>(code)], *u, s,
                      [&](uint64_t at, const attr_value& v) {
                          if (at == at_linkage_name || at == at_mips_linkage_name) {
                              linkage = string_of(v, *u, s);
                          } else if ((at == at_abstract_origin || at == at_specification) &&
                                     !have_next) {
                              have_next = reference_of(v, *u, next);
                          }
                      })) {
            break;
        }
        if (!linkage.empty()) {
            f = func_name(linkage);
            break;
        }
        if (!have_next) {
            break;
        }
        at_off = next;
    }
    idx.names.emplace(off, f);
    return f;
}

} // namespace

void inline_frames(const exe_image& img, uint64_t call, uint64_t func_lo, uint64_t func_hi,
                   std::vector<inline_level>& out) {
    out.clear();
    if constexpr (!read_inlines) {
        return; // not a -release -trace build (read_inlines)
    }
    read_inline_frames(img, call, func_lo, func_hi, out);
}

void read_inline_frames(const exe_image& img, uint64_t call, uint64_t func_lo, uint64_t func_hi,
                        std::vector<inline_level>& out) {
    out.clear();
    if (img.debug_info.empty() || img.debug_abbrev.empty()) {
        return;
    }
    // One index for the program, as indexed_lines_of has one: a trace of a crash waits here for a
    // runtime.Stack of another thread that is building it (the first call reads all of
    // .debug_info), and the threads of a report of several goroutines take it one after the other.
    // (Made under the lock, never freed: see indexed_lines_of.)
    static std::mutex mu;
    static inline_index* idx = nullptr;
    std::lock_guard<std::mutex> lk(mu);
    if (idx == nullptr) {
        idx = new inline_index;
    }
    info_sections s = sections_of(img);
    if (!idx->built || idx->data != img.debug_info.data() || idx->size != img.debug_info.size()) {
        *idx = inline_index{};
        idx->data = img.debug_info.data();
        idx->size = img.debug_info.size();
        try {
            build_inlines(s, *idx);
        } catch (...) {
            *idx = inline_index{}; // no memory: no inlined frames
            return;
        }
        idx->built = true;
    }
    // The calls inlined into the function [func_lo, func_hi) start in it: look at those whose
    // range has call.
    auto first = std::lower_bound(idx->sites.begin(), idx->sites.end(), func_lo,
                                  [](const inline_site& a, uint64_t x) { return a.lo < x; });
    std::vector<const inline_site*> chain;
    for (auto it = first; it != idx->sites.end() && it->lo <= call && it->lo < func_hi; ++it) {
        if (call < it->hi) {
            chain.push_back(&*it);
        }
    }
    std::sort(chain.begin(), chain.end(),
              [](const inline_site* a, const inline_site* b) { return a->depth < b->depth; });
    for (const inline_site* c : chain) {
        out.push_back(inline_level{origin_name(*idx, s, c->origin), idx->files[c->file], c->line,
                                   c->lo, c->hi});
    }
}

// --- the trace ---

namespace {

// A frame of the trace.
struct frame {
    go_func fn;           // with raw_pc: none
    uint64_t call;        // the address of the call (the return address - 1), in the file
    uint64_t offset;      // of the return address in the function
    bool raw_pc;          // no symbols: "?()" and the address
    bool gopanic;         // the panic frame between a deferred call and the panic it ran in
    bool inlined = false; // a function that the compiler inlined: "(...)" and no offset, as Go
    bool known = false;   // file and line are the place of a call inlined into it (inline_frames)
    std::string file{};
    int64_t line = 0;
    uint64_t near_lo = 0, near_hi = 0; // the innermost inlined call's range (line_query)
};

// inner and outer are how many frames Go 1.25 prints at the top and at the bottom of a stack
// with more than inner + outer frames ("...N frames elided..." in between).
constexpr std::size_t inner_frames = 50;
constexpr std::size_t outer_frames = 50;

} // namespace

const exe_symbol* find_symbol(const exe_image& img, uint64_t addr) noexcept {
    auto it = std::upper_bound(
        img.funcs.begin(), img.funcs.end(), addr,
        [](uint64_t a, const exe_symbol& s) { return a < static_cast<uint64_t>(s.addr); });
    if (it == img.funcs.begin()) {
        return nullptr;
    }
    const exe_symbol& sym = *--it;
    if (sym.size != 0 && addr >= sym.addr + sym.size) {
        return nullptr; // between functions
    }
    return &sym;
}

namespace {

// shared_func_name is func_name of a symbol of the kept image (program_image), remembered for the
// life of the process: a report of many goroutines (the deadlock detector's) shows the same few
// functions over and over, and demangling them is most of the cost of a trace.
go_func shared_func_name(const exe_symbol& sym) {
    static std::mutex mu;
    // (Made under the lock, never freed: see indexed_lines_of.)
    static std::unordered_map<const exe_symbol*, go_func>* memo = nullptr;
    {
        std::lock_guard<std::mutex> lk(mu);
        if (memo == nullptr) {
            memo = new std::unordered_map<const exe_symbol*, go_func>;
        }
        if (auto it = memo->find(&sym); it != memo->end()) {
            return it->second;
        }
    }
    go_func f = func_name(sym.name);
    std::lock_guard<std::mutex> lk(mu);
    memo->emplace(&sym, f);
    return f;
}

} // namespace

void append_trace(std::string& out, const uintptr_t* pcs, std::size_t n, bool complete,
                  const exe_image* cached) {
    exe_image own;
    bool have =
        cached != nullptr ? !cached->funcs.empty() : os::read_exe(own) && !own.funcs.empty();
    const exe_image& img = cached != nullptr ? *cached : own;
    auto in_code = [&](uintptr_t pc) {
        return std::any_of(img.code.begin(), img.code.end(),
                           [pc](const auto& r) { return pc >= r.first && pc < r.second; });
    };
    std::vector<frame> frames;
    if (have) {
        // The names of a deep recursion are the same few functions: each is parsed once.
        std::vector<std::pair<const exe_symbol*, go_func>> names;
        std::vector<inline_level> levels;
        for (std::size_t k = 0; k < n; k++) {
            if (pcs[k] == 0) {
                frames.push_back(frame{go_func{}, 0, 0, false, true});
                continue;
            }
            if (!in_code(pcs[k])) {
                continue; // a shared library: libc, libstdc++, libgcc
            }
            // pc - 1 is in the call instruction, so in the calling function and on its line
            // (the return address can be the start of the next line or function).
            uint64_t addr = pcs[k] - img.bias;
            uint64_t call = addr - 1;
            const exe_symbol* found = find_symbol(img, call);
            if (found == nullptr) {
                continue; // before the first function, or between functions
            }
            const exe_symbol& sym = *found;
            // C++'s main is the program's start, below it libc: the trace ends there, after the Go
            // functions that the compiler inlined into it (g++'s -flto can inline a small
            // main.main into run_main into main).
            const bool cxx_main = sym.name == "main";
            auto named = std::find_if(names.begin(), names.end(),
                                      [&](const auto& e) { return e.first == &sym; });
            if (named == names.end()) {
                names.emplace_back(&sym, cxx_main            ? go_func{}
                                         : cached != nullptr ? shared_func_name(sym)
                                                             : func_name(sym.name));
                named = names.end() - 1;
            }
            // The calls that the compiler inlined at call, innermost first, as Go shows them: each
            // inlined function at the line of the call inlined into it (the innermost at the line
            // of its code), then the function at the line of the outermost inlined call. A
            // function that the trace doesn't show (the runtime's helpers), or that isn't Go (C++
            // that the program calls: its frames are shown only when they are on the stack, as in
            // a debug build), only lends its call's line to the function around it.
            inline_frames(img, call, sym.addr, sym.size != 0 ? sym.addr + sym.size : ~uint64_t(0),
                          levels);
            for (std::size_t j = levels.size(); j-- > 0;) {
                if (!levels[j].fn.shown || levels[j].fn.raw) {
                    continue;
                }
                frame f{levels[j].fn, call, 0, false, false};
                // main.main inlined into C++'s main (g++'s -flto) is the program's own frame, as Go
                // shows it: main.main() with an offset (into C++'s main here).
                f.inlined = !(cxx_main && levels[j].fn.name == "main.main");
                if (!f.inlined) {
                    f.offset = addr - sym.addr;
                }
                if (j + 1 < levels.size()) {
                    f.known = true;
                    f.file = levels[j + 1].file;
                    f.line = levels[j + 1].line;
                } else {
                    f.near_lo = levels[j].lo; // the line of its code, from the line table
                    f.near_hi = levels[j].hi;
                }
                frames.push_back(std::move(f));
            }
            if (cxx_main) {
                break;
            }
            if (named->second.shown) {
                frame f{named->second, call, addr - sym.addr, false, false};
                if (!levels.empty()) {
                    f.known = true;
                    f.file = levels[0].file;
                    f.line = levels[0].line;
                }
                frames.push_back(std::move(f));
            }
        }
    }
    if (std::all_of(frames.begin(), frames.end(), [](const frame& f) { return f.gopanic; }) &&
        (!have || img.debug_line.empty())) {
        // No symbols (a stripped program), or none of a function that the trace shows in a program
        // without line tables: the addresses in the program's code, for addr2line. With symbols and
        // lines, a stack of only the runtime's frames shows none (Go hides its runtime's frames
        // too).
        frames.clear();
        for (std::size_t k = 0; k < n; k++) {
            if (pcs[k] == 0) {
                frames.push_back(frame{go_func{}, 0, 0, false, true});
            } else if (in_code(pcs[k])) {
                frames.push_back(frame{go_func{}, pcs[k] - img.bias, 0, true, false});
            }
        }
    }

    // Like Go 1.25: a stack of more than 100 frames shows the inner 50, how many are left out,
    // and the outer 50. A stack that couldn't be recorded to its end (complete is false) shows
    // the inner 50 and Go's older line.
    std::vector<std::size_t> shown;
    std::size_t elided = 0;
    if (!complete && frames.size() > inner_frames) {
        frames.resize(inner_frames);
    }
    for (std::size_t k = 0; k < frames.size(); k++) {
        if (complete && frames.size() > inner_frames + outer_frames && k == inner_frames) {
            elided = frames.size() - inner_frames - outer_frames;
            k += elided - 1;
            continue;
        }
        shown.push_back(k);
    }

    std::vector<line_query> lines(shown.size());
    bool near = false; // a query with an inlined call's range: only indexed_lines_of answers it
    for (std::size_t k = 0; k < shown.size(); k++) {
        const frame& f = frames[shown[k]];
        lines[k].addr = f.call;
        lines[k].near_lo = f.near_lo;
        lines[k].near_hi = f.near_hi;
        near = near || f.near_hi > f.near_lo;
        if (f.known && f.line > 0) {
            lines[k].file = f.file; // a line that is set is not asked of the line table
            lines[k].line = f.line;
        }
    }
    if (have && (cached != nullptr || near)) {
        // A caller that keeps the image asks again and again (runtime.Stack, the race reporter).
        indexed_lines_of(img.debug_line, img.debug_line_str, img.debug_str, lines.data(),
                         lines.size());
    } else if (have) {
        lines_of(img.debug_line, img.debug_line_str, img.debug_str, lines.data(), lines.size());
    }
    for (std::size_t k = 0; k < shown.size(); k++) {
        if (elided != 0 && k == inner_frames) {
            out += "...";
            print_uint(out, elided);
            out += " frames elided...\n";
        }
        const frame& f = frames[shown[k]];
        if (f.gopanic) {
            // Go shows runtime.gopanic, with its argument words and its line in the runtime's
            // panic.go; gocpp's panic has no such frame of its own.
            out += "panic(...)\n\t?:0\n";
            continue;
        }
        if (f.raw_pc) {
            out += "?()\n\t?:0 pc=";
            print_hex(out, f.call + 1);
            out += '\n';
            continue;
        }
        out += f.fn.name;
        if (!f.fn.raw || f.fn.name.find('(') == std::string::npos) {
            out += f.fn.params || f.inlined ? "(...)" : "()";
        }
        out += "\n\t";
        if (lines[k].line > 0) {
            out += lines[k].file;
            out += ':';
            print_int(out, lines[k].line);
        } else {
            out += "?:0";
        }
        if (!f.inlined) {
            out += " +";
            print_hex(out, f.offset);
        }
        out += '\n';
    }
    if (!complete) {
        out += "...additional frames elided...\n";
    }
}

namespace {

// recorded returns the stack that the panic e recorded, and whether it is the whole stack.
std::pair<std::pair<const uintptr_t*, std::size_t>, bool> recorded(const panic_error& e) {
    if (e.gp_all != nullptr) {
        return {{e.gp_all->data(), e.gp_all->size()}, true};
    }
    // A full gp_pcs without gp_all: the second walk failed (no memory, or a stack of more than
    // panic_error::gp_max_walk frames).
    // (A panic that was not completed, gp_bound, has only the inner part of its stack.)
    return {{e.gp_pcs, static_cast<std::size_t>(e.gp_npcs)},
            e.gp_bound == 0 && e.gp_npcs < panic_error::gp_max_trace};
}

} // namespace

namespace {

// chain_pcs appends to pcs the trace's addresses of the panic e and its chain, and returns
// whether they are the whole stack: like Go, the frames of the last panic's deferred call, a
// "panic" frame (the 0 address, append_trace), the frames of the panic before it, and so on
// down to the stack of the first panic.
bool chain_pcs(const panic_error& e, std::vector<uintptr_t>& pcs) {
    for (const panic_error* p = &e;;) {
        auto [st, whole] = recorded(*p);
        if (p->gp_prev != nullptr && p->gp_inner >= 0 &&
            static_cast<std::size_t>(p->gp_inner) <= st.second) {
            pcs.insert(pcs.end(), st.first, st.first + p->gp_inner);
            pcs.push_back(0);
            p = p->gp_prev.get();
            continue;
        }
        pcs.insert(pcs.end(), st.first, st.first + st.second);
        return whole;
    }
}

} // namespace

void append_created_by(std::string& out, const goroutine* g) {
    if (g != nullptr) {
        append_created_by(out, g->create_pc, g->parent_id);
    }
}

void append_created_by(std::string& out, const void* create_pc, uint64_t parent_id) {
    if (create_pc == nullptr) {
        return;
    }
    if (create_pc == &creator_gofunc) {
        // Go's own goroutine of an AfterFunc: the go statement of time.goFunc, in a system
        // goroutine (no "in goroutine N"), at the line of Go 1.25's time/sleep.go.
        out += "created by time.goFunc\n\ttime/sleep.go:215 +0x2d\n";
        return;
    }
    if (create_pc == &creator_pull) {
        // The coroutine of iter.Pull (Go 1.25's iter/iter.go:269, in the goroutine that called
        // Pull).
        out += "created by iter.Pull[...] in goroutine ";
        print_uint(out, parent_id);
        out += "\n\titer/iter.go:269 +0x90\n";
        return;
    }
    auto pc = reinterpret_cast<uintptr_t>(create_pc);
    frame_info fi;
    resolve_pcs(&pc, 1, true, &fi);
    if (!fi.ok || !fi.fn.shown) {
        return;
    }
    if (const exe_image* img = read_inlines ? program_image() : nullptr; img != nullptr) {
        // A go statement in code that the compiler inlined: the line of the innermost Go function
        // there (the runtime's helpers that start the goroutine are inlined around the call too).
        uint64_t call = pc - img->bias - 1;
        if (const exe_symbol* sym = find_symbol(*img, call); sym != nullptr) {
            std::vector<inline_level> levels;
            inline_frames(*img, call, sym->addr,
                          sym->size != 0 ? sym->addr + sym->size : ~uint64_t(0), levels);
            std::size_t j = levels.size();
            while (j > 0 && (!levels[j - 1].fn.shown || levels[j - 1].fn.raw)) {
                j--;
            }
            // levels[j - 1] is the innermost Go function, which has the line table's line when it
            // is the innermost call; else the line is that of the call inlined into it (j < size).
            if (j < levels.size() && levels[j].line > 0) {
                fi.file = levels[j].file;
                fi.line = levels[j].line;
            }
        }
    }
    out += "created by ";
    out += fi.fn.name;
    out += " in goroutine ";
    print_uint(out, parent_id);
    out += "\n\t";
    out += fi.file.empty() ? std::string("?") : fi.file;
    out += ':';
    print_int(out, fi.line);
    out += " +";
    print_hex(out, pc - fi.entry);
    out += '\n';
}

namespace {

// The traceback setting, as Go keeps it (runtime.setTraceback): the level in the bits above 2
// (0 none, 1 the goroutine's stack, 2 and more also the runtime's frames, which gocpp does not
// have), bit 0 "all goroutines" and bit 1 "crash" (Go aborts: gocpp ends with code 2 as for a
// panic). debug.SetTraceback replaces the setting, with the bits of the GOTRACEBACK variable
// added to it: the variable is the floor.
constexpr uint32_t tb_all = 1;
constexpr uint32_t tb_crash = 2;
constexpr int tb_shift = 2;
constexpr uint32_t tb_unset = ~uint32_t(0);

constinit std::atomic<uint32_t> tb_cache{tb_unset};

// tb_parse is Go's setTraceback for a level text: none, single (or empty), all, system, crash,
// wer (Windows only, like crash; elsewhere an invalid word), or a number n (all goroutines, level
// n; 0 prints no trace). Anything else is "all" with level 0: no trace.
uint32_t tb_parse(std::string_view level) noexcept {
    if (level == "none") {
        return 0;
    }
    if (level == "single" || level.empty()) {
        return 1u << tb_shift;
    }
    if (level == "all") {
        return (1u << tb_shift) | tb_all;
    }
    if (level == "system") {
        return (2u << tb_shift) | tb_all;
    }
    if (level == "crash") {
        return (2u << tb_shift) | tb_all | tb_crash;
    }
#ifdef _WIN32
    if (level == "wer") {
        return (2u << tb_shift) | tb_all | tb_crash;
    }
#endif
    uint32_t t = tb_all;
    uint64_t n = 0;
    bool number = true;
    for (char c : level) {
        if (c < '0' || c > '9' || n > (uint64_t(1) << 28)) {
            number = false;
            break;
        }
        n = n * 10 + static_cast<uint64_t>(c - '0');
    }
    if (number && n <= (uint64_t(1) << 28)) {
        t |= static_cast<uint32_t>(n) << tb_shift;
    }
    return t;
}

// The setting of the GOTRACEBACK variable, read once.
uint32_t tb_env() noexcept {
    static const uint32_t env = [] {
        const char* v = std::getenv("GOTRACEBACK");
        return tb_parse(v != nullptr ? std::string_view(v) : std::string_view());
    }();
    return env;
}

uint32_t tb_get() noexcept {
#ifdef GOCPP_NO_TRACE
    // A -release build without -trace (the driver defines GOCPP_NO_TRACE: ADR-136, the amendment
    // of 2026-10-23): a crash prints only its message, as Go's GOTRACEBACK=none, whatever the
    // variable or debug.SetTraceback says. runtime.Stack and runtime.Callers do not ask.
    return 0;
#else
    uint32_t t = tb_cache.load(std::memory_order_acquire);
    return t == tb_unset ? tb_env() : t;
#endif
}

} // namespace

bool traceback_all() noexcept {
    return (tb_get() & tb_all) != 0;
}

int traceback_level() noexcept {
    return static_cast<int>(tb_get() >> tb_shift);
}

void set_traceback_level(std::string_view level) noexcept {
    tb_cache.store(tb_parse(level) | tb_env(), std::memory_order_release);
}

void panic_report(std::string& out, const panic_error& e) noexcept {
    std::size_t header = 0;
    try {
        // The chain, the first panic first.
        std::vector<const panic_error*> chain;
        for (const panic_error* p = &e; p != nullptr; p = p->gp_prev.get()) {
            chain.push_back(p);
        }
        std::reverse(chain.begin(), chain.end());
        panic_report(out, chain.data(), chain.size());
        header = out.size();
        if (traceback_level() == 0) {
            return; // GOTRACEBACK=none: the panic's text only
        }
        std::vector<uintptr_t> pcs;
        bool complete = chain_pcs(e, pcs);
        append_trace(out, pcs.data(), pcs.size(), complete);
        append_created_by(out, static_cast<const goroutine*>(os::tls_get()));
    } catch (...) {
        // No memory for the trace (or for the header): keep what is there.
        if (header != 0) {
            out.resize(header);
        }
    }
    if (traceback_all()) {
        append_other_goroutines(out);
    }
}

void fatal_trace(std::string& out) noexcept {
    std::size_t start = out.size();
    try {
        panic_error here{gp::string()};
        here.gp_record_all(); // the whole stack, not only up to the next function with defers
        auto [st, whole] = recorded(here);
        // The innermost gp_run that runs a deferred call for a panic. Its caller's frame is the
        // first one with its return address after those of the gp_runs inside it with the same
        // one (a recursion that runs deferred calls, normally, at the same place).
        const run_mark* panicking = run_marks();
        std::size_t same = 0;
        while (panicking != nullptr && panicking->gp_panic == nullptr) {
            panicking = panicking->gp_next;
        }
        for (const run_mark* m = run_marks(); panicking != nullptr && m != panicking;
             m = m->gp_next) {
            same += m->gp_ret == panicking->gp_ret ? 1 : 0;
        }
        std::size_t caller = st.second;
        for (std::size_t k = 0; panicking != nullptr && k < st.second; k++) {
            if (st.first[k] == panicking->gp_ret && same-- == 0) {
                caller = k;
                break;
            }
        }
        if (caller == st.second) {
            append_trace(out, st.first, st.second, whole);
            append_created_by(out, static_cast<const goroutine*>(os::tls_get()));
            return;
        }
        // Like a panic's chain (chain_pcs): the deferred call's frames (gp_run's own frame is
        // left out by append_trace), "panic", then the running panic's stack and chain.
        std::vector<uintptr_t> pcs(st.first, st.first + caller);
        pcs.push_back(0);
        // (The frame of the function that handles the running panic is on this stack.)
        panicking->gp_panic->gp_complete();
        bool complete = chain_pcs(*panicking->gp_panic, pcs);
        append_trace(out, pcs.data(), pcs.size(), complete);
        append_created_by(out, static_cast<const goroutine*>(os::tls_get()));
    } catch (...) {
        out.resize(start);
    }
}

void panic_chain(panic_error& newer, panic_error& running) noexcept {
    panic_error* oldest = &newer;
    while (oldest->gp_prev != nullptr) {
        oldest = oldest->gp_prev.get();
    }
    if (oldest == &running) {
        return;
    }
    // The caller is the function whose gp_run handles both panics: a panic that still has only
    // its inner stack ends in that function's frame, which is on the stack now (the stacks are
    // compared below, and a copy of running goes into the chain).
    running.gp_complete();
    oldest->gp_complete();
    try {
        oldest->gp_prev = std::make_shared<panic_error>(running);
        oldest->gp_inner = -1;
        // The frames of oldest's stack that are the deferred call's: those above the frames it
        // has in common with this stack, which are gp_run's caller and the callers below it
        // (gp_run's own frame differs: here it is in its handler, there in the call).
        auto [st, whole] = recorded(*oldest);
        std::vector<uintptr_t> here;
        if (!whole || !walk_all(here)) {
            return;
        }
        std::size_t common = 0;
        while (common < st.second && common < here.size() &&
               st.first[st.second - 1 - common] == here[here.size() - 1 - common]) {
            common++;
        }
        if (common > 0 && common < st.second) {
            oldest->gp_inner = static_cast<int64_t>(st.second - common);
        }
    } catch (...) {
        // No memory: the chain (or the trace's frames of the older panics) stays incomplete.
    }
}

} // namespace detail

} // namespace gp
