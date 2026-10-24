// The running goroutine's own stack, for runtime.Caller, Callers, FuncForPC and Stack
// (gocpp/trace.hpp, step 8.10): the program's file is read once and kept, and what is asked about
// an address is kept too. Standard C++ only: the addresses and the file come from the OS layer
// (gocpp/os.hpp).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <cstdint>
#include <mutex>
#include <string>
#include <string_view>
#include <unordered_map>
#include <vector>

namespace gp::detail {

namespace {

// The program's file, read the first time it is asked for. Never freed: a leak check finds it
// through this pointer.
struct image_state {
    exe_image img;
    bool have = false;
};

image_state& state() {
    static image_state* s = [] {
        auto* st = new image_state;
        st->have = os::read_exe(st->img) && !st->img.funcs.empty();
        return st;
    }();
    return *s;
}

// mu guards the caches below (and the use of the image's line table).
// (No constinit: with the posix thread model of MinGW-w64 (WinLibs, MSYS2) std::mutex has no
// constexpr constructor; with libstdc++ on Linux and the mcf model it is constant-initialized
// anyway.)
std::mutex mu;

// The Go name of each function that was asked about, by its symbol.
std::unordered_map<const exe_symbol*, go_func>& names() {
    static auto* m = new std::unordered_map<const exe_symbol*, go_func>;
    return *m;
}

// What was asked of an address: the key is the address, shifted, with a bit for "call".
std::unordered_map<uint64_t, frame_info>& answers() {
    static auto* m = new std::unordered_map<uint64_t, frame_info>;
    return *m;
}

constexpr std::size_t max_answers = std::size_t(1) << 16;

const go_func& name_of(const exe_symbol& sym) {
    auto& m = names();
    auto it = m.find(&sym);
    if (it == m.end()) {
        it = m.emplace(&sym, func_name(sym.name)).first;
    }
    return it->second;
}

bool in_code(const exe_image& img, uintptr_t pc) {
    return std::any_of(img.code.begin(), img.code.end(),
                       [pc](const auto& r) { return pc >= r.first && pc < r.second; });
}

// The natives of the package runtime that are frames of the trace as Go names them: Callers
// is the first frame that Callers(0, ...) reports.
struct named_native {
    std::string_view symbol; // the mangled name's start
    const char* name;
};
constexpr named_native natives[] = {
    {"_ZN2gp7runtime7callersE", "runtime.Callers"},
    {"_ZN2gp7runtime6callerE", "runtime.Caller"},
    {"_ZN2gp7runtime5stackE", "runtime.Stack"},
};

// The frames of the calls that the compiler inlined (-release -trace, ADR-136): runtime.Callers
// gives one address for each frame that Go shows, also for an inlined call, as Go's does, and
// CallersFrames, Caller and FuncForPC name it. An inlined call has no address of its own in the
// program, so it gets a made-up one: inlined_tag, the return address of the physical frame and
// the depth of the inlined call (1: inlined into the function). No address of a program's code has
// the tag's bit (user space ends far below it), and the address - 1 that a caller of Callers may
// ask about decodes the same.
constexpr uintptr_t inlined_tag = uintptr_t(1) << 62;
constexpr int inlined_shift = 8;
constexpr std::size_t max_inlined_depth = (std::size_t(1) << (inlined_shift - 1)) - 1; // 127

uintptr_t inlined_pc(uintptr_t pc, std::size_t depth) {
    return inlined_tag | (pc << inlined_shift) | (static_cast<uintptr_t>(depth) << 1) | 1;
}

bool decode_inlined(uintptr_t x, uintptr_t& pc, std::size_t& depth) {
    if ((x & inlined_tag) == 0) {
        return false;
    }
    pc = (x & ~inlined_tag) >> inlined_shift;
    depth = static_cast<std::size_t>((x & ((uintptr_t(1) << inlined_shift) - 1)) >> 1);
    return depth > 0;
}

// The calls inlined at the return address pc of a frame in sym (inline_frames), outermost first.
void inlines_at(const exe_image& img, const exe_symbol& sym, uintptr_t pc,
                std::vector<inline_level>& out) {
    uint64_t call = pc - img.bias - 1;
    inline_frames(img, call, sym.addr, sym.size != 0 ? sym.addr + sym.size : ~uint64_t(0), out);
}

// A level that the trace shows as a frame: a Go function (not the runtime's, not C++).
bool shown_level(const inline_level& l) {
    return l.fn.shown && !l.fn.raw;
}

} // namespace

const exe_image* program_image() {
    image_state& st = state();
    return st.have ? &st.img : nullptr;
}

std::vector<uintptr_t> shown_frames(std::size_t want) {
    std::vector<uintptr_t> out;
    image_state& st = state();
    if (!st.have || want == 0) {
        return out;
    }
    // The runtime's own frames and the helpers come between the Go frames, a few for each: ask
    // for more than want, and again, bigger, when the walk had to stop before it found want.
    std::size_t cap = 64 + want * 4;
    for (;;) {
        std::vector<uintptr_t> all(cap);
        int n = os::stack(all.data(), static_cast<int>(cap));
        out.clear();
        bool done = false;
        {
            std::lock_guard<std::mutex> lk(mu);
            std::vector<inline_level> levels;
            for (int k = 0; k < n && !done; k++) {
                uintptr_t pc = all[static_cast<std::size_t>(k)];
                if (!in_code(st.img, pc)) {
                    continue; // libc, libstdc++
                }
                const exe_symbol* sym = find_symbol(st.img, pc - st.img.bias - 1);
                if (sym == nullptr) {
                    continue;
                }
                // The Go functions inlined at pc first, innermost first (also into C++'s main).
                inlines_at(st.img, *sym, pc, levels);
                for (std::size_t j = levels.size(); j-- > 0 && !done;) {
                    // The depth has 7 bits in a made-up address (inlined_pc): a call inlined more
                    // than 127 levels deep (no compiler does that) has no frame of its own.
                    if (shown_level(levels[j]) && j + 1 <= max_inlined_depth) {
                        out.push_back(inlined_pc(pc, j + 1));
                        done = out.size() == want;
                    }
                }
                if (done) {
                    break;
                }
                if (sym->name == "main") {
                    done = true; // C++'s main: the program's start
                } else if (name_of(*sym).shown) {
                    out.push_back(pc);
                    done = out.size() == want;
                }
            }
        }
        if (done || static_cast<std::size_t>(n) < cap || cap >= (std::size_t(1) << 22)) {
            return out;
        }
        cap *= 8; // the stack goes on
    }
}

void resolve_pcs(const uintptr_t* pcs, std::size_t n, bool call, frame_info* out) {
    for (std::size_t i = 0; i < n; i++) {
        out[i] = frame_info{};
    }
    image_state& st = state();
    if (!st.have) {
        return;
    }
    std::lock_guard<std::mutex> lk(mu);
    auto& cache = answers();
    if (cache.size() > max_answers) {
        cache.clear();
    }
    std::vector<std::size_t> todo;
    std::vector<line_query> queries;
    std::vector<inline_level> levels;
    for (std::size_t i = 0; i < n; i++) {
        uint64_t key = (static_cast<uint64_t>(pcs[i]) << 1) | (call ? 1 : 0);
        if (auto it = cache.find(key); it != cache.end()) {
            out[i] = it->second;
            continue;
        }
        uintptr_t pc = pcs[i];
        std::size_t depth = 0;
        bool inlined = decode_inlined(pc, pc, depth);
        uint64_t near_lo = 0, near_hi = 0;
        if (!in_code(st.img, pc)) {
            continue;
        }
        uint64_t addr = pc - st.img.bias;
        uint64_t look = (call || inlined) && addr > 0 ? addr - 1 : addr;
        const exe_symbol* sym = find_symbol(st.img, look);
        if (sym == nullptr) {
            continue;
        }
        frame_info f;
        f.ok = true;
        f.fn = name_of(*sym);
        f.entry = static_cast<uintptr_t>(sym->addr + st.img.bias);
        // The calls inlined there: an inlined call's frame is its function at the line of the call
        // inlined into it (or of its code, the innermost one), and the function's own frame is at
        // the line of the outermost inlined call, as in the trace (append_trace).
        inline_frames(st.img, look, sym->addr,
                      sym->size != 0 ? sym->addr + sym->size : ~uint64_t(0), levels);
        if (inlined) {
            if (depth > levels.size() || !shown_level(levels[depth - 1])) {
                continue; // not an address that shown_frames made for this program
            }
            f.fn = levels[depth - 1].fn;
            if (depth < levels.size()) {
                f.file = levels[depth].file;
                f.line = levels[depth].line;
            } else {
                near_lo = levels[depth - 1].lo; // the innermost call: the line of its code
                near_hi = levels[depth - 1].hi;
            }
        } else if (!levels.empty()) {
            f.file = levels[0].file;
            f.line = levels[0].line;
        }
        bool native = false;
        for (const named_native& nn : natives) {
            if (std::string_view(sym->name).starts_with(nn.symbol)) {
                native = true;
                f.fn.name = nn.name;
                f.fn.shown = true;
                f.fn.raw = false;
                f.fn.params = true;
                f.file = "runtime/extern.go"; // no line: gocpp's runtime is C++
                f.line = 0;
            }
        }
        out[i] = f;
        if (!native && (f.file.empty() || f.line <= 0)) {
            todo.push_back(i);
            line_query q;
            q.addr = look;
            q.near_lo = near_lo;
            q.near_hi = near_hi;
            queries.push_back(q);
        } else {
            cache.emplace(key, f);
        }
    }
    if (queries.empty()) {
        return;
    }
    indexed_lines_of(st.img.debug_line, st.img.debug_line_str, st.img.debug_str, queries.data(),
                     queries.size());
    for (std::size_t j = 0; j < todo.size(); j++) {
        std::size_t i = todo[j];
        out[i].file = std::move(queries[j].file);
        out[i].line = queries[j].line;
        uint64_t key = (static_cast<uint64_t>(pcs[i]) << 1) | (call ? 1 : 0);
        cache.emplace(key, out[i]);
    }
}

} // namespace gp::detail
