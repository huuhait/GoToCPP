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
constinit std::mutex mu;

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
            for (int k = 0; k < n && !done; k++) {
                uintptr_t pc = all[static_cast<std::size_t>(k)];
                if (!in_code(st.img, pc)) {
                    continue; // libc, libstdc++
                }
                const exe_symbol* sym = find_symbol(st.img, pc - st.img.bias - 1);
                if (sym == nullptr) {
                    continue;
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
    for (std::size_t i = 0; i < n; i++) {
        uint64_t key = (static_cast<uint64_t>(pcs[i]) << 1) | (call ? 1 : 0);
        if (auto it = cache.find(key); it != cache.end()) {
            out[i] = it->second;
            continue;
        }
        if (!in_code(st.img, pcs[i])) {
            continue;
        }
        uint64_t addr = pcs[i] - st.img.bias;
        uint64_t look = call && addr > 0 ? addr - 1 : addr;
        const exe_symbol* sym = find_symbol(st.img, look);
        if (sym == nullptr) {
            continue;
        }
        frame_info f;
        f.ok = true;
        f.fn = name_of(*sym);
        f.entry = static_cast<uintptr_t>(sym->addr + st.img.bias);
        for (const named_native& nn : natives) {
            if (std::string_view(sym->name).starts_with(nn.symbol)) {
                f.fn.name = nn.name;
                f.fn.shown = true;
                f.fn.raw = false;
                f.fn.params = true;
                f.file = "runtime/extern.go"; // no line: gocpp's runtime is C++
                f.line = 0;
            }
        }
        out[i] = f;
        if (f.file.empty()) {
            todo.push_back(i);
            line_query q;
            q.addr = look;
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
