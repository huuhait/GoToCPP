// The stack trace of an unrecovered panic (gocpp/trace.hpp): Go names from mangled C++ names,
// the DWARF line table, and the frames in Go's format. Standard C++ only: the addresses, the
// program's file and demangling come from the OS layer (gocpp/os.hpp).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <cstdint>
#include <cstring>
#include <memory>
#include <new>
#include <string>
#include <string_view>
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
    gp_npcs = os::stack(gp_pcs, gp_max_trace);
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
        bool instance = false;
        if (part.starts_with("gp_")) {
            if (part.starts_with("gp_inst_") && part.size() > 17 && part[16] == '_' &&
                std::all_of(part.begin() + 8, part.begin() + 16, [](char d) {
                    return (d >= '0' && d <= '9') || (d >= 'a' && d <= 'f');
                })) {
                // An instance of a generic function (internal/lower/generics.go), gp_inst_<hash>_F:
                // Go's main.F[...].
                part = part.substr(17);
                instance = true;
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
        if (c.targs || instance) {
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

// One unit of the line table: its header and its line program.
// qs are the queries, sorted by address.
void line_unit(reader& r, std::string_view line_str, std::string_view str,
               const std::vector<line_query*>& qs) {
    bool dw64 = false;
    uint64_t length = r.fixed(4);
    if (length == 0xffffffff) {
        dw64 = true;
        length = r.fixed(8);
    } else if (length >= 0xfffffff0) {
        r.bad = true;
        return;
    }
    if (r.bad || length > r.left()) {
        r.bad = true;
        return;
    }
    reader u{r.p, r.p + length};
    r.p += length; // the next unit
    uint64_t version = u.fixed(2);
    if (version < 2 || version > 5) {
        return;
    }
    if (version >= 5) {
        u.fixed(1); // address_size
        u.fixed(1); // segment_selector_size
    }
    uint64_t header_length = u.fixed(dw64 ? 8 : 4);
    if (u.bad || header_length > u.left()) {
        return;
    }
    const unsigned char* program = u.p + header_length;
    uint64_t min_inst = u.fixed(1);
    if (version >= 4) {
        u.fixed(1); // maximum_operations_per_instruction: 1 except on VLIW
    }
    u.fixed(1); // default_is_stmt
    auto line_base = static_cast<int8_t>(u.fixed(1));
    uint64_t line_range = u.fixed(1);
    uint64_t opcode_base = u.fixed(1);
    if (u.bad || line_range == 0 || opcode_base == 0) {
        return;
    }
    std::vector<uint64_t> std_lengths(static_cast<std::size_t>(opcode_base), 0);
    for (uint64_t k = 1; k < opcode_base; k++) {
        std_lengths[static_cast<std::size_t>(k)] = u.fixed(1);
    }
    std::vector<entry> dirs, files;
    uint64_t file_base = 1; // DWARF 2-4 count files from 1, DWARF 5 from 0
    if (version >= 5) {
        file_base = 0;
        if (!read_entries(u, dw64, line_str, str, dirs) ||
            !read_entries(u, dw64, line_str, str, files)) {
            return;
        }
    } else {
        dirs.push_back(entry{}); // the compilation directory, which only .debug_info has
        for (;;) {
            std::string_view d = u.cstr();
            if (u.bad || d.empty()) {
                break;
            }
            dirs.push_back(entry{d, 0});
        }
        for (;;) {
            std::string_view f = u.cstr();
            if (u.bad || f.empty()) {
                break;
            }
            entry e{f, u.uleb()};
            u.uleb(); // mtime
            u.uleb(); // length
            files.push_back(e);
        }
    }
    if (u.bad || program > u.end) {
        return;
    }
    u.p = program;

    auto path_of = [&](uint64_t file) {
        std::string p;
        if (file < file_base || file - file_base >= files.size()) {
            return p;
        }
        const entry& f = files[static_cast<std::size_t>(file - file_base)];
        if (absolute(f.path) || f.dir >= dirs.size()) {
            return std::string(f.path);
        }
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
        return p;
    };

    // The state machine. A row covers [its address, the next row's address) in its sequence;
    // prev is the last row. A sequence whose address is set to 0 (or to a tombstone, ~0 or
    // ~0 - 1) is the code of a discarded duplicate (an inline function that another unit
    // kept): it is skipped.
    uint64_t addr = 0, file = 1;
    int64_t line = 1;
    bool have_prev = false, skip_seq = false;
    uint64_t prev_addr = 0, prev_file = 0;
    int64_t prev_line = 0;
    auto row = [&](bool end_seq) {
        if (!skip_seq && have_prev && prev_addr < addr) {
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
        line_unit(r, line_str, str, sorted);
    }
}

// --- the trace ---

namespace {

// A frame of the trace.
struct frame {
    go_func fn;      // with raw_pc: none
    uint64_t call;   // the address of the call (the return address - 1), in the file
    uint64_t offset; // of the return address in the function
    bool raw_pc;     // no symbols: "?()" and the address
    bool gopanic;    // the panic frame between a deferred call and the panic it ran in
};

// inner and outer are how many frames Go 1.25 prints at the top and at the bottom of a stack
// with more than inner + outer frames ("...N frames elided..." in between).
constexpr std::size_t inner_frames = 50;
constexpr std::size_t outer_frames = 50;

} // namespace

void append_trace(std::string& out, const uintptr_t* pcs, std::size_t n, bool complete) {
    exe_image img;
    bool have = os::read_exe(img) && !img.funcs.empty();
    auto in_code = [&](uintptr_t pc) {
        return std::any_of(img.code.begin(), img.code.end(),
                           [pc](const auto& r) { return pc >= r.first && pc < r.second; });
    };
    std::vector<frame> frames;
    if (have) {
        // The names of a deep recursion are the same few functions: each is parsed once.
        std::vector<std::pair<const exe_symbol*, go_func>> names;
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
            auto it = std::upper_bound(
                img.funcs.begin(), img.funcs.end(), call,
                [](uint64_t a, const exe_symbol& s) { return a < static_cast<uint64_t>(s.addr); });
            if (it == img.funcs.begin()) {
                continue;
            }
            const exe_symbol& sym = *--it;
            if (sym.size != 0 && call >= sym.addr + sym.size) {
                continue; // between functions
            }
            if (sym.name == "main") {
                break; // C++'s main: the program's start, below it libc
            }
            auto named = std::find_if(names.begin(), names.end(),
                                      [&](const auto& e) { return e.first == &sym; });
            if (named == names.end()) {
                names.emplace_back(&sym, func_name(sym.name));
                named = names.end() - 1;
            }
            if (named->second.shown) {
                frames.push_back(frame{named->second, call, addr - sym.addr, false, false});
            }
        }
    }
    if (std::all_of(frames.begin(), frames.end(), [](const frame& f) { return f.gopanic; })) {
        // No symbols (a stripped program), or none of a function that the trace shows: the
        // addresses in the program's code, for addr2line.
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
    for (std::size_t k = 0; k < shown.size(); k++) {
        lines[k].addr = frames[shown[k]].call;
    }
    if (have) {
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
            out += f.fn.params ? "(...)" : "()";
        }
        out += "\n\t";
        if (lines[k].line > 0) {
            out += lines[k].file;
            out += ':';
            print_int(out, lines[k].line);
        } else {
            out += "?:0";
        }
        out += " +";
        print_hex(out, f.offset);
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
    return {{e.gp_pcs, static_cast<std::size_t>(e.gp_npcs)}, e.gp_npcs < panic_error::gp_max_trace};
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
        std::vector<uintptr_t> pcs;
        bool complete = chain_pcs(e, pcs);
        append_trace(out, pcs.data(), pcs.size(), complete);
    } catch (...) {
        // No memory for the trace (or for the header): keep what is there.
        if (header != 0) {
            out.resize(header);
        }
    }
}

void fatal_trace(std::string& out) noexcept {
    std::size_t start = out.size();
    try {
        panic_error here{gp::string()}; // records the stack, like a panic
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
            return;
        }
        // Like a panic's chain (chain_pcs): the deferred call's frames (gp_run's own frame is
        // left out by append_trace), "panic", then the running panic's stack and chain.
        std::vector<uintptr_t> pcs(st.first, st.first + caller);
        pcs.push_back(0);
        bool complete = chain_pcs(*panicking->gp_panic, pcs);
        append_trace(out, pcs.data(), pcs.size(), complete);
    } catch (...) {
        out.resize(start);
    }
}

void panic_chain(panic_error& newer, const panic_error& running) noexcept {
    panic_error* oldest = &newer;
    while (oldest->gp_prev != nullptr) {
        oldest = oldest->gp_prev.get();
    }
    if (oldest == &running) {
        return;
    }
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
