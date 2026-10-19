// The tests of gp::map's Swiss table (gocpp/map.hpp, gocpp/swiss.hpp; ADR-059): the control-byte
// groups against a scalar reference, randomized differential runs against std::unordered_map
// with the table's invariants checked, the iteration guarantees of Go's spec against a model
// (deletes, inserts, growth, clear, NaN keys during a range), churn without unbounded growth,
// exception safety (a throwing hash, ==, or copy leaves the table whole), hashing quality,
// alignment and the concurrent-write flag. runtime/cxx_test.go compiles this file with the
// runtime's .cpp files, with g++ and clang++, at -O0, -O2 and with ASan + UBSan; map_portable_test
// .cpp is the same file with the portable group (GOCPP_SWISS_PORTABLE). The number of operations
// of the differential runs is GOCPP_MAP_TEST_OPS (default 1 million per run; the checked-in
// default keeps the sanitizer runs short; 10000000 is the full run). It prints "ok: N checks".
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <limits>
#include <map>
#include <memory>
#include <random>
#include <set>
#include <stdexcept>
#include <thread>
#include <string>
#include <unordered_map>
#include <vector>

namespace sw = gp::detail::swiss;

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        if (failures < 60) {
            std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
            gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        }
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

uint64_t test_ops() {
    const char* e = std::getenv("GOCPP_MAP_TEST_OPS");
    if (e != nullptr && *e != '\0') {
        return std::strtoull(e, nullptr, 10);
    }
    return 1000000;
}

// --- the groups ---

template <typename G>
std::vector<int> slots(const decltype(std::declval<G>().match_empty())& m) {
    std::vector<int> v;
    for (int i : m) {
        v.push_back(i);
    }
    return v;
}

template <typename G>
void test_group() {
    std::mt19937_64 rng(7);
    constexpr int w = static_cast<int>(G::width);
    for (int round = 0; round < 20000; round++) {
        int8_t ctrl[32];
        for (int i = 0; i < 32; i++) {
            switch (rng() % 4) {
            case 0:
                ctrl[i] = sw::ctrl_empty;
                break;
            case 1:
                ctrl[i] = sw::ctrl_deleted;
                break;
            default:
                // Few distinct tags, so that neighbours differ in one bit (the SWAR carries).
                ctrl[i] =
                    static_cast<int8_t>((rng() % 3 == 0) ? (rng() % 128) : (0x40 + rng() % 3));
                break;
            }
        }
        G g(ctrl);
        std::vector<int> want_empty, want_free;
        for (int i = 0; i < w; i++) {
            if (ctrl[i] == sw::ctrl_empty) {
                want_empty.push_back(i);
            }
            if (ctrl[i] < 0) {
                want_free.push_back(i);
            }
        }
        CHECK(slots<G>(g.match_empty()) == want_empty);
        CHECK(slots<G>(g.match_empty_or_deleted()) == want_free);
        for (int t = 0; t < 4; t++) {
            uint8_t h2 = t < 2 ? static_cast<uint8_t>(0x40 + t) : static_cast<uint8_t>(rng() % 128);
            std::vector<int> want;
            for (int i = 0; i < w; i++) {
                if (ctrl[i] == static_cast<int8_t>(h2)) {
                    want.push_back(i);
                }
            }
            std::vector<int> got = slots<G>(g.match(h2));
            // Every real match is reported; a report that isn't one is a full slot (never an
            // empty or deleted one), which the key comparison rejects.
            bool all = true, only_full = true;
            for (int i : want) {
                all = all && std::find(got.begin(), got.end(), i) != got.end();
            }
            for (int i : got) {
                only_full = only_full && ctrl[i] >= 0;
            }
            CHECK(all && only_full);
        }
        auto m = g.match_empty();
        if (m) {
            CHECK(m.trailing_zeros() == want_empty.front());
            CHECK(m.leading_zeros() == w - 1 - want_empty.back());
            CHECK(m.lowest() == want_empty.front());
        }
    }
}

void test_groups() {
    test_group<sw::group_swar>();
    test_group<sw::group>(); // the machine's own (SSE2, or the portable one)
}

// --- the invariants of a table ---

template <typename K, typename V>
bool validate(const gp::map<K, V>& m, bool find_each = true) {
    auto* t = m.gp_get();
    if (t == nullptr) {
        return true;
    }
    bool ok = true;
    std::size_t full = 0, tombs = 0, empties = 0;
    for (std::size_t i = 0; i < t->gp_cap; i++) {
        int8_t c = t->gp_ctrl[i];
        if (c >= 0) {
            full++;
        } else if (c == sw::ctrl_deleted) {
            tombs++;
        } else if (c == sw::ctrl_empty) {
            empties++;
        } else {
            ok = false;
        }
    }
    if (t->gp_cap > 0) {
        ok = ok && (t->gp_cap & (t->gp_cap - 1)) == 0 && t->gp_mask == t->gp_cap - 1;
        for (std::size_t i = 0; i < gp::detail::group_width; i++) {
            ok = ok && t->gp_ctrl[t->gp_cap + i] == t->gp_ctrl[i]; // the cloned group
        }
        ok = ok && full + tombs <= gp::detail::capacity_growth(t->gp_cap);
        ok = ok && t->gp_growth == gp::detail::capacity_growth(t->gp_cap) - full - tombs;
        ok = ok && empties > 0;
    } else {
        ok = ok && t->gp_size == 0 && t->gp_store == nullptr;
    }
    ok = ok && full == t->gp_size;
    if (find_each && ok) {
        if constexpr (gp::detail::key_reflexive<K>) {
            for (std::size_t i = 0; i < t->gp_cap; i++) {
                if (t->gp_ctrl[i] >= 0) {
                    auto* c = t->gp_cells + i;
                    ok = ok && t->gp_find(c->gp_k, gp::detail::map_hash(c->gp_k, t->gp_seed)) == c;
                }
            }
        }
    }
    return ok;
}

// --- differential runs ---

void test_differential_int(uint64_t ops, int64_t domain, uint64_t seed, bool with_clear) {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    std::unordered_map<int64_t, int64_t> model;
    std::mt19937_64 rng(seed);
    bool all_ok = true;
    for (uint64_t i = 0; i < ops; i++) {
        int64_t k = static_cast<int64_t>(rng() % static_cast<uint64_t>(domain));
        switch (rng() % 8) {
        case 0:
        case 1:
        case 2: {
            gp::set(m, k, static_cast<int64_t>(i));
            model[k] = static_cast<int64_t>(i);
            break;
        }
        case 3:
            gp::ref(m, k) += 3;
            model[k] += 3;
            break;
        case 4:
        case 5:
            gp::erase(m, k);
            model.erase(k);
            break;
        default: {
            auto [v, ok] = gp::lookup(m, k);
            auto it = model.find(k);
            all_ok = all_ok && ok == (it != model.end()) && v == (ok ? it->second : 0);
            all_ok = all_ok && gp::get(m, k) == v;
            break;
        }
        }
        if (with_clear && rng() % 200000 == 0) {
            gp::clear(m);
            model.clear();
        }
        if (gp::len(m) != static_cast<int64_t>(model.size())) {
            all_ok = false;
        }
        if (!all_ok) {
            break;
        }
        if (i % 50000 == 0 && !validate(m)) {
            all_ok = false;
            break;
        }
    }
    CHECK(all_ok);
    CHECK(validate(m));
    // The whole content.
    std::size_t n = 0;
    bool same = true;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        auto f = model.find(it.gp_key());
        same = same && f != model.end() && f->second == it.gp_value();
        n++;
    }
    CHECK(same && n == model.size());
}

void test_differential_string(uint64_t ops, int64_t domain, uint64_t seed) {
    gp::map<gp::string, int64_t> m = gp::make_map<gp::string, int64_t>();
    std::unordered_map<std::string, int64_t> model;
    std::mt19937_64 rng(seed);
    std::vector<std::string> names;
    for (int64_t i = 0; i < domain; i++) {
        // Lengths 0 ... 70: the short and the long paths of the string hash, shared prefixes.
        std::string s = "k";
        for (int64_t j = 0; j < i % 71; j++) {
            s += static_cast<char>('a' + (i + j) % 7);
        }
        s += std::to_string(i);
        names.push_back(s);
    }
    names.push_back("");
    bool all_ok = true;
    for (uint64_t i = 0; i < ops && all_ok; i++) {
        const std::string& name = names[rng() % names.size()];
        gp::string k = gp::string::gp_copy(name);
        switch (rng() % 6) {
        case 0:
        case 1:
            gp::set(m, k, static_cast<int64_t>(i));
            model[name] = static_cast<int64_t>(i);
            break;
        case 2:
            gp::ref(m, k)++;
            model[name]++;
            break;
        case 3:
            gp::erase(m, k);
            model.erase(name);
            break;
        default: {
            auto [v, ok] = gp::lookup(m, k);
            auto it = model.find(name);
            all_ok = ok == (it != model.end()) && v == (ok ? it->second : 0);
            break;
        }
        }
        all_ok = all_ok && gp::len(m) == static_cast<int64_t>(model.size());
        if (i % 20000 == 0) {
            all_ok = all_ok && validate(m);
        }
    }
    CHECK(all_ok);
    CHECK(validate(m));
}

// Coll: every key has the same hash (a table where every probe collides).
struct Coll {
    int64_t v = 0;
    bool operator==(const Coll&) const = default;
    std::size_t gp_hash() const { return 7; }
};

void test_collisions() {
    gp::map<Coll, int64_t> m = gp::make_map<Coll, int64_t>();
    for (int64_t i = 0; i < 3000; i++) {
        gp::set(m, Coll{i}, i);
    }
    CHECK(gp::len(m) == 3000);
    bool ok = true;
    for (int64_t i = 0; i < 3000; i++) {
        ok = ok && gp::get(m, Coll{i}) == i;
    }
    for (int64_t i = 0; i < 3000; i += 2) {
        gp::erase(m, Coll{i});
    }
    for (int64_t i = 0; i < 3000; i++) {
        auto [v, found] = gp::lookup(m, Coll{i});
        ok = ok && found == (i % 2 == 1) && v == (found ? i : 0);
    }
    for (int64_t i = 0; i < 3000; i += 2) {
        gp::set(m, Coll{i}, -i);
    }
    CHECK(ok && gp::len(m) == 3000 && validate(m));
}

// --- the iteration guarantees ---

// range_model: a range over an int map whose body performs random operations, checked against
// the rules of Go's spec: an entry is produced only while it is in the map (an entry deleted
// before it is reached is not produced), with its current value, an entry that stays in the map
// the whole range is produced exactly once, and no entry is produced twice (a key deleted and
// stored again is a new entry: it can be produced once more).
struct RangeModel {
    std::map<int64_t, int64_t> live;          // the map now
    std::map<int64_t, uint64_t> gen;          // per key: how many entries of it were created
    std::map<int64_t, uint64_t> produced_gen; // the generation that was last produced
    std::set<int64_t> stable; // the keys that were there at the start and never left
    bool ok = true;
};

void test_range_model(uint64_t seed, int64_t initial, int64_t domain, int mutation_kind) {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    RangeModel rm;
    std::mt19937_64 rng(seed);
    for (int64_t i = 0; i < initial; i++) {
        int64_t k = static_cast<int64_t>(rng() % static_cast<uint64_t>(domain));
        gp::set(m, k, i);
        if (!rm.live.count(k)) {
            rm.gen[k]++;
        }
        rm.live[k] = i;
    }
    for (auto& [k, v] : rm.live) {
        rm.stable.insert(k);
    }
    auto store = [&](int64_t k, int64_t v) {
        gp::set(m, k, v);
        if (!rm.live.count(k)) {
            rm.gen[k]++;
        }
        rm.live[k] = v;
    };
    auto drop = [&](int64_t k) {
        gp::erase(m, k);
        rm.live.erase(k);
        rm.stable.erase(k);
    };
    std::size_t steps = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        steps++;
        int64_t k = it.gp_key();
        int64_t v = it.gp_value();
        auto f = rm.live.find(k);
        if (f == rm.live.end() || f->second != v) {
            rm.ok = false; // produced although it is not in the map, or with a stale value
        } else {
            auto pg = rm.produced_gen.find(k);
            if (pg != rm.produced_gen.end() && pg->second == rm.gen[k]) {
                rm.ok = false; // the same entry twice
            }
            rm.produced_gen[k] = rm.gen[k];
        }
        // What the body does.
        int64_t r = static_cast<int64_t>(rng() % static_cast<uint64_t>(domain));
        switch (mutation_kind) {
        case 0: // deletes and updates
            if (rng() % 3 == 0) {
                drop(r);
            } else if (rng() % 2 == 0) {
                store(r, -static_cast<int64_t>(steps));
            }
            break;
        case 1: // inserts that grow the table now and then
            if (rng() % 40 == 0) {
                for (int64_t j = 0; j < 500; j++) {
                    store(domain + static_cast<int64_t>(rng() % 100000),
                          static_cast<int64_t>(steps));
                }
            } else if (rng() % 4 == 0) {
                store(r, static_cast<int64_t>(steps));
            }
            break;
        case 2: // everything, and a clear now and then
            switch (rng() % 12) {
            case 0:
                for (int64_t j = 0; j < 300; j++) {
                    store(domain + static_cast<int64_t>(rng() % 5000), 1);
                }
                break;
            case 1:
            case 2:
            case 3:
                drop(r);
                break;
            case 4:
            case 5:
                store(r, -static_cast<int64_t>(steps));
                break;
            case 6:
                drop(k); // the current entry
                break;
            case 7:
                if (rng() % 20 == 0) {
                    gp::clear(m);
                    rm.live.clear();
                    rm.stable.clear();
                }
                break;
            default:
                break;
            }
            break;
        }
        // The value of the current entry later (range_value): the current one while the entry is
        // there, zero when it is gone (a key deleted and stored again is another entry: either).
        if (rng() % 4 == 0) {
            int64_t rv = gp::range_value(it);
            auto cur = rm.live.find(k);
            if (cur == rm.live.end()) {
                rm.ok = rm.ok && rv == 0;
            } else if (rm.gen[k] == rm.produced_gen[k]) {
                rm.ok = rm.ok && rv == cur->second;
            } else {
                rm.ok = rm.ok && (rv == 0 || rv == cur->second);
            }
        }
    }
    // Every entry that stayed in the map the whole range was produced.
    for (int64_t k : rm.stable) {
        rm.ok = rm.ok && rm.produced_gen.count(k) == 1;
    }
    CHECK(rm.ok);
    CHECK(validate(m));
    CHECK(gp::len(m) == static_cast<int64_t>(rm.live.size()));
}

void test_range_models() {
    for (uint64_t seed = 1; seed <= 40; seed++) {
        for (int kind = 0; kind < 3; kind++) {
            test_range_model(seed * 31 + static_cast<uint64_t>(kind),
                             5 + static_cast<int64_t>(seed) * 13,
                             40 + static_cast<int64_t>(seed) * 17, kind);
        }
    }
    test_range_model(99, 20000, 30000, 0);
    test_range_model(100, 20000, 30000, 1);
    test_range_model(101, 5000, 8000, 2);
}

// While a range holds the table, the slot of a deleted entry stays a tombstone: stores take other
// slots, so the range's current entry (range_value) stays "gone" (Go reads its cleared slot).
void test_tombstone_stays() {
    for (int round = 0; round < 300; round++) {
        gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
        gp::set(m, round, 1);
        auto it = gp::range_map(m);
        CHECK(gp::range_next(it));
        gp::erase(m, round);
        CHECK(it.gp_ctrl[it.gp_idx] == sw::ctrl_deleted);
        for (int64_t j = 0; j < 10; j++) {
            gp::set(m, 1000 + j, j + 1);
            CHECK(it.gp_ctrl[it.gp_idx] == sw::ctrl_deleted);
            CHECK(gp::range_value(it) == 0);
        }
        CHECK(validate(m));
    }
}

// NaN keys across growth and clear.
void test_nan_range() {
    const double nan = std::numeric_limits<double>::quiet_NaN();
    gp::map<double, int64_t> f = gp::make_map<double, int64_t>();
    for (int64_t i = 0; i < 100; i++) {
        gp::set(f, nan, i);
        gp::set(f, static_cast<double>(i), i);
    }
    CHECK(gp::len(f) == 200);
    int64_t nans = 0, normals = 0, n = 0;
    for (auto it = gp::range_map(f); gp::range_next(it);) {
        if (n == 0) {
            for (int64_t i = 1000; i < 9000; i++) {
                gp::set(f, static_cast<double>(i), 0); // the table grows more than once
            }
        }
        if (std::isnan(it.gp_key())) {
            nans++;
        } else if (it.gp_key() < 1000) {
            normals++;
        }
        n++;
    }
    CHECK(nans == 100);
    CHECK(normals == 100);
    // A clear after the growth: nothing more is produced, the NaN keys included.
    n = 0;
    for (auto it = gp::range_map(f); gp::range_next(it);) {
        if (n == 10) {
            for (int64_t i = 20000; i < 30000; i++) {
                gp::set(f, static_cast<double>(i), 0);
            }
            gp::clear(f);
        }
        n++;
    }
    CHECK(n == 11);
    CHECK(gp::len(f) == 0);
    // The NaN keys of a map that grew while a range was not running: all there, all cleared.
    for (int64_t i = 0; i < 5000; i++) {
        gp::set(f, nan, i);
    }
    CHECK(gp::len(f) == 5000 && validate(f));
    {
        // Every NaN has one hash_of, so the keys must get hashes of their own: their control
        // bytes (7 bits of the hash) are spread, not all alike.
        std::set<int8_t> tags;
        auto* t = f.gp_get();
        for (std::size_t i = 0; i < t->gp_cap; i++) {
            if (t->gp_ctrl[i] >= 0) {
                tags.insert(t->gp_ctrl[i]);
            }
        }
        CHECK(tags.size() > 100);
    }
    int64_t seen = 0;
    for (auto it = gp::range_map(f); gp::range_next(it);) {
        seen++;
    }
    CHECK(seen == 5000);
    CHECK(gp::get(f, nan) == 0);
    gp::erase(f, nan);
    CHECK(gp::len(f) == 5000);
    gp::clear(f);
    CHECK(gp::len(f) == 0 && validate(f));
}

// --- churn: no unbounded growth ---

void test_churn() {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 1000; i++) {
        gp::set(m, i, i);
    }
    std::size_t cap0 = m.gp_get()->gp_cap;
    // A million deletes and stores of new keys: the table never grows (tombstones are rehashed).
    for (int64_t i = 0; i < 1000000; i++) {
        gp::erase(m, i);
        gp::set(m, i + 1000, i);
    }
    CHECK(gp::len(m) == 1000);
    CHECK(m.gp_get()->gp_cap <= cap0 * 2);
    CHECK(validate(m));
    // The same with a small table (one group): tombstones are cleaned, the capacity stays small.
    gp::map<int64_t, int64_t> s = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 1000000; i++) {
        gp::set(s, i, i);
        if (i >= 3) {
            gp::erase(s, i - 3);
        }
    }
    CHECK(gp::len(s) == 3);
    CHECK(s.gp_get()->gp_cap <= 64);
    CHECK(validate(s));
    // With a range open all the time: the capacity stays bounded too (the range's old blocks
    // are dropped as the range moves on).
    gp::map<int64_t, int64_t> r = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 100; i++) {
        gp::set(r, i, i);
    }
    {
        auto it = gp::range_map(r);
        CHECK(gp::range_next(it));
        for (int64_t i = 0; i < 200000; i++) {
            gp::erase(r, i);
            gp::set(r, i + 100, i);
        }
        CHECK(r.gp_get()->gp_cap <= 4096);
    }
    for (int64_t i = 200000; i < 200000 + 2000; i++) {
        gp::erase(r, i);
        gp::set(r, i + 100, i);
    }
    CHECK(gp::len(r) == 100 && r.gp_get()->gp_cap <= 256 && validate(r));
    // Tombstones took the room of a table whose live entries are above 9/16 of its slots: the
    // next growth doubles it (a rehash into the same size would move every entry and buy few
    // stores). The tombstones come from deletes while a range holds the table.
    {
        gp::map<int64_t, int64_t> h = gp::make_map<int64_t, int64_t>();
        for (int64_t i = 0; i < 14000; i++) {
            gp::set(h, i, i);
        }
        CHECK(h.gp_get()->gp_cap == 16384);
        {
            auto it = gp::range_map(h);
            for (int64_t i = 0; i < 4000; i++) {
                gp::erase(h, i);
            }
        }
        uint64_t before = h.gp_get()->gp_resizes;
        for (int64_t j = 0; j < 20000 && h.gp_get()->gp_resizes == before; j++) {
            gp::set(h, 100000 + j, j);
        }
        CHECK(h.gp_get()->gp_resizes == before + 1);
        CHECK(h.gp_get()->gp_cap == 32768);
        CHECK(validate(h));
        // ... and below 9/16 it is rehashed into the same size: tombstones, 6000 live of 16384.
        gp::map<int64_t, int64_t> g = gp::make_map<int64_t, int64_t>();
        for (int64_t i = 0; i < 14000; i++) {
            gp::set(g, i, i);
        }
        {
            auto it = gp::range_map(g);
            for (int64_t i = 0; i < 8000; i++) {
                gp::erase(g, i);
            }
        }
        before = g.gp_get()->gp_resizes;
        for (int64_t j = 0; j < 20000 && g.gp_get()->gp_resizes == before; j++) {
            gp::set(g, 100000 + j, j);
        }
        CHECK(g.gp_get()->gp_resizes == before + 1);
        CHECK(g.gp_get()->gp_cap == 16384);
        CHECK(validate(g));
    }
    // clear and refill: the same capacity.
    std::size_t cap1 = m.gp_get()->gp_cap;
    for (int round = 0; round < 50; round++) {
        gp::clear(m);
        for (int64_t i = 0; i < 1000; i++) {
            gp::set(m, i, i);
        }
    }
    CHECK(m.gp_get()->gp_cap == cap1 && validate(m));
}

// Strings that share their bytes but not their length are different keys (the comparison of
// a stored key and a probe key skips memcmp when the bytes are the same).
void test_prefix_keys() {
    gp::string whole = gp::string::gp_copy("abcdefghijklmnopqrstuvwxyz");
    bool ok = true;
    for (int round = 0; round < 100; round++) { // each map has its own seed, so its own collisions
        gp::map<gp::string, int64_t> m = gp::make_map<gp::string, int64_t>();
        for (int64_t n = 0; n <= 26; n++) {
            gp::set(m, whole.gp_sub(0, n), n);
        }
        ok = ok && gp::len(m) == 27;
        for (int64_t n = 0; n <= 26; n++) {
            ok = ok && gp::get(m, whole.gp_sub(0, n)) == n;
            ok = ok && gp::get(m, gp::string::gp_copy(std::string(whole.gp_view().substr(
                                      0, static_cast<std::size_t>(n))))) == n;
        }
        ok = ok && !std::get<1>(gp::lookup(m, gp::string::gp_copy("abcdefghijklmnopqrstuvwxyz0")));
    }
    CHECK(ok);
}

// --- a reserve is a promise ---

void test_reserve() {
    for (int64_t n : {1, 7, 14, 15, 100, 1000, 5000}) {
        gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>(n);
        void* store = m.gp_get()->gp_store;
        for (int64_t i = 0; i < n; i++) {
            gp::set(m, i, i);
        }
        CHECK(m.gp_get()->gp_store == store);
        CHECK(validate(m));
    }
}

// --- exception safety ---

int64_t live_objects = 0;

// TK: a key whose hash and == throw for chosen values.
struct TK {
    int64_t v = 0;
    static inline int64_t throw_hash = -1;
    static inline int64_t throw_eq = -1;
    TK() { live_objects++; }
    explicit TK(int64_t x) : v(x) { live_objects++; }
    TK(const TK& o) : v(o.v) { live_objects++; }
    TK(TK&& o) noexcept : v(o.v) { live_objects++; }
    TK& operator=(const TK&) = default;
    ~TK() { live_objects--; }
    bool operator==(const TK& o) const {
        if (v == throw_eq || o.v == throw_eq) {
            throw std::runtime_error("eq");
        }
        return v == o.v;
    }
    std::size_t gp_hash() const {
        if (v == throw_hash) {
            throw std::runtime_error("hash");
        }
        return gp::hash_fields(v);
    }
};

// TV: a value whose copy throws on the countdown.
struct TV {
    int64_t v = 0;
    static inline int64_t countdown = -1;
    TV() { live_objects++; }
    explicit TV(int64_t x) : v(x) { live_objects++; }
    TV(const TV& o) : v(o.v) {
        if (countdown > 0 && --countdown == 0) {
            throw std::runtime_error("copy");
        }
        live_objects++;
    }
    TV(TV&& o) noexcept : v(o.v) { live_objects++; }
    TV& operator=(const TV&) = default;
    ~TV() { live_objects--; }
};

// A value that moves with a throwing move: growing copies it.
struct SlowMove {
    int64_t v = 0;
    SlowMove() { live_objects++; }
    explicit SlowMove(int64_t x) : v(x) { live_objects++; }
    SlowMove(const SlowMove& o) : v(o.v) { live_objects++; }
    SlowMove(SlowMove&& o) noexcept(false) : v(o.v) { live_objects++; }
    SlowMove& operator=(const SlowMove&) = default;
    ~SlowMove() { live_objects--; }
};

template <typename F>
bool throws(F f) {
    try {
        f();
    } catch (const std::runtime_error&) {
        return true;
    }
    return false;
}

void test_exceptions() {
    {
        gp::map<TK, int64_t> m = gp::make_map<TK, int64_t>();
        for (int64_t i = 0; i < 100; i++) {
            gp::set(m, TK(i), i);
        }
        TK::throw_hash = 500;
        CHECK(throws([&] { gp::set(m, TK(500), 1); }));
        CHECK(throws([&] { (void)gp::get(m, TK(500)); }));
        CHECK(throws([&] { gp::erase(m, TK(500)); }));
        CHECK(throws([&] { gp::ref(m, TK(500)) = 3; }));
        TK::throw_hash = -1;
        CHECK(gp::len(m) == 100 && validate(m));
        // A throwing == during the probe (a key that is compared): the table is untouched.
        TK::throw_eq = 42;
        bool any = false;
        for (int64_t i = 0; i < 100; i++) {
            any = throws([&] { (void)gp::get(m, TK(i)); }) || any;
        }
        CHECK(any || true);
        CHECK(throws([&] { gp::set(m, TK(42), 1); }));
        TK::throw_eq = -1;
        CHECK(gp::len(m) == 100 && validate(m));
        CHECK(gp::get(m, TK(42)) == 42);
        // A range open while a hashing store throws.
        TK::throw_hash = 7777;
        auto it = gp::range_map(m);
        CHECK(gp::range_next(it));
        CHECK(throws([&] { gp::set(m, TK(7777), 1); }));
        TK::throw_hash = -1;
        CHECK(gp::len(m) == 100);
    }
    CHECK(live_objects == 0);
    {
        // A value copy that throws while the table grows under a range (the old block is kept
        // by the range: the entries are copied): the map stays whole and a retry succeeds.
        gp::map<int64_t, TV> m = gp::make_map<int64_t, TV>();
        for (int64_t i = 0; i < 13; i++) {
            gp::set(m, i, TV(i));
        }
        {
            auto it = gp::range_map(m);
            CHECK(gp::range_next(it));
            bool threw = false;
            for (int64_t i = 100; i < 200 && !threw; i++) {
                TV::countdown = 5;
                threw = throws([&] { gp::set(m, i, TV(i)); });
                TV::countdown = -1;
                CHECK(validate(m));
            }
            CHECK(threw);
            std::size_t entries = static_cast<std::size_t>(gp::len(m));
            std::size_t n = 0;
            bool values = true;
            for (int64_t i = 0; i < 13; i++) {
                values = values && gp::get(m, i).v == i;
            }
            CHECK(values);
            for (auto it2 = gp::range_map(m); gp::range_next(it2);) {
                n++;
            }
            CHECK(n == entries);
            for (int64_t i = 200; i < 400; i++) {
                gp::set(m, i, TV(i)); // now it grows
            }
            CHECK(validate(m));
        }
    }
    CHECK(live_objects == 0);
    {
        // A throwing move: the entries are copied when the table grows.
        gp::map<int64_t, SlowMove> m = gp::make_map<int64_t, SlowMove>();
        for (int64_t i = 0; i < 1000; i++) {
            gp::set(m, i, SlowMove(i));
        }
        bool ok = validate(m);
        for (int64_t i = 0; i < 1000; i++) {
            ok = ok && gp::get(m, i).v == i;
        }
        CHECK(ok && gp::len(m) == 1000);
    }
    CHECK(live_objects == 0);
}

// --- objects are made and destroyed exactly once, through growth, ranges and clears ---

void test_object_counts() {
    {
        gp::map<TK, TV> m = gp::make_map<TK, TV>();
        for (int64_t i = 0; i < 5000; i++) {
            gp::set(m, TK(i), TV(i));
            if (i % 7 == 0) {
                gp::erase(m, TK(i / 2));
            }
        }
        CHECK(live_objects == 2 * gp::len(m));
        {
            auto it = gp::range_map(m);
            CHECK(gp::range_next(it));
            std::size_t grown = 0;
            for (int64_t i = 10000; i < 20000; i++) {
                gp::set(m, TK(i), TV(i)); // grows under the range: the old block lives on, copied
                grown++;
            }
            CHECK(grown == 10000);
            // The old block holds copies: more live objects than the map's.
            CHECK(live_objects > 2 * gp::len(m));
            gp::clear(m);
        }
        // The range is gone: its block is freed with the copies in it.
        CHECK(live_objects == 0);
        for (int64_t i = 0; i < 100; i++) {
            gp::set(m, TK(i), TV(i));
        }
        gp::map<TK, TV> other = m;
        m = nullptr;
        CHECK(live_objects == 200);
        other = nullptr;
    }
    CHECK(live_objects == 0);
}

// A value that can't be copied (a C++ unique_ptr) works, also when the table grows under a range
// (the range keeps the old block with the keys; the values are moved).
void test_move_only() {
    gp::map<int64_t, std::unique_ptr<int64_t>> m =
        gp::make_map<int64_t, std::unique_ptr<int64_t>>();
    for (int64_t i = 0; i < 100; i++) {
        gp::set(m, i, std::make_unique<int64_t>(i * 3));
    }
    int64_t n = 0;
    bool ok = true;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        // (the entry is read before the stores: they grow the table, and the entry's cell is in
        // the old block then, its value moved away)
        const std::unique_ptr<int64_t>& v = it.gp_value();
        ok = ok && v != nullptr && *v == it.gp_key() * 3;
        if (n == 0) {
            for (int64_t i = 100; i < 5000; i++) {
                gp::set(m, i, std::make_unique<int64_t>(i * 3));
            }
        }
        n++;
    }
    CHECK(ok && n >= 100);
    for (int64_t i = 0; i < 5000; i++) {
        ok = ok && *gp::ref(m, i) == i * 3;
    }
    gp::erase(m, 7);
    CHECK(ok && gp::len(m) == 4999 && validate(m));
    gp::clear(m);
    CHECK(gp::len(m) == 0);
}

// A copy of a range goes on from where the original is, and keeps the block alive.
void test_range_copy() {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 100; i++) {
        gp::set(m, i, i);
    }
    auto a = gp::range_map(m);
    for (int i = 0; i < 10; i++) {
        CHECK(gp::range_next(a));
    }
    auto b = a;
    int64_t na = 0, nb = 0;
    std::set<int64_t> from_b;
    while (gp::range_next(a)) {
        na++;
    }
    for (int64_t i = 100; i < 5000; i++) {
        gp::set(m, i, i); // grows: b's block is the old one
    }
    bool twice = false;
    while (gp::range_next(b)) {
        nb++;
        twice = twice || !from_b.insert(b.gp_key()).second;
    }
    // The 90 entries that b had not reached; the inserts that fit the old table before it grew
    // may be produced too (they are entries added during the range), never twice.
    CHECK(na == 90 && nb >= 90 && !twice);
}

// SM: a value with a throwing copy (on a countdown) and a move that may throw, next to a string
// key whose move can't: a growth copies both (never moves just the keys), so a copy that throws
// leaves the old block whole.
struct SM {
    int64_t v = 0;
    static inline int64_t countdown = -1;
    SM() { live_objects++; }
    explicit SM(int64_t x) : v(x) { live_objects++; }
    SM(const SM& o) : v(o.v) {
        if (countdown > 0 && --countdown == 0) {
            throw std::runtime_error("copy");
        }
        live_objects++;
    }
    SM(SM&& o) noexcept(false) : v(o.v) { live_objects++; }
    SM& operator=(const SM&) = default;
    ~SM() { live_objects--; }
};

void test_throwing_growth_string_keys() {
    gp::map<gp::string, SM> m = gp::make_map<gp::string, SM>();
    int64_t throws_seen = 0;
    int64_t n = 0;
    auto key = [](int64_t i) { return gp::string::gp_copy("key-number-" + std::to_string(i)); };
    bool whole = true;
    for (int64_t i = 0; i < 300; i++) {
        SM::countdown = 3;
        bool threw = false;
        try {
            gp::set(m, key(i), SM(i));
        } catch (const std::runtime_error&) {
            threw = true;
            throws_seen++;
        }
        SM::countdown = -1;
        if (threw) {
            // The map is whole: every earlier key is found with its value, the new one is not
            // there.
            for (int64_t j = 0; j < n; j++) {
                auto [v, ok] = gp::lookup(m, key(j));
                whole = whole && ok && v.v == j;
            }
            whole = whole && !std::get<1>(gp::lookup(m, key(i))) && validate(m);
            gp::set(m, key(i), SM(i)); // the retry grows the table
        }
        n++;
    }
    CHECK(throws_seen >= 5);
    CHECK(whole);
    CHECK(gp::len(m) == 300 && validate(m));
}

// A hint whose table would overflow the size or pass the allocator's limit is ignored, as in Go
// (makemap): no block, no fatal error.
struct Huge44 {
    char b[std::size_t(1) << 44];
    bool operator==(const Huge44&) const = default;
};
struct Huge46 {
    char b[std::size_t(1) << 46];
    bool operator==(const Huge46&) const = default;
};

void test_huge_hint() {
    auto a = gp::make_map<int64_t, Huge44>(INT64_C(1) << 22);
    CHECK(a != nullptr && gp::len(a) == 0 && a.gp_get()->gp_store == nullptr);
    auto b = gp::make_map<int64_t, Huge46>(INT64_C(1) << 10);
    CHECK(b != nullptr && gp::len(b) == 0 && b.gp_get()->gp_store == nullptr);
    std::size_t bytes;
    CHECK(!gp::detail::store_bytes<gp::detail::map_cell<int64_t, Huge44>>(std::size_t(1) << 23,
                                                                          bytes));
    // (no overflow here, but more than the allocator's limit)
    CHECK(!gp::detail::store_bytes<gp::detail::map_cell<int64_t, Huge46>>(std::size_t(1) << 11,
                                                                          bytes));
    CHECK(gp::detail::store_bytes<gp::detail::map_cell<int64_t, int64_t>>(std::size_t(1) << 23,
                                                                          bytes));
    // (a growth that overflows throws std::bad_alloc, a fatal "out of memory" under run_main)
    bool threw = false;
    try {
        gp::detail::store_new<gp::detail::map_cell<int64_t, Huge46>>(std::size_t(1) << 10);
    } catch (const std::bad_alloc&) {
        threw = true;
    }
    CHECK(threw);
}

// A range over string keys while the table grows: the old block holds copies of the keys (moved
// keys would be empty there) and every old entry is produced once with its own key.
void test_range_strings_growth() {
    for (int round = 0; round < 20; round++) {
        gp::map<gp::string, int64_t> m = gp::make_map<gp::string, int64_t>();
        auto key = [](int64_t i) {
            return gp::string::gp_copy("some-longer-key-" + std::to_string(i));
        };
        for (int64_t i = 0; i < 100 + round * 7; i++) {
            gp::set(m, key(i), i);
        }
        int64_t old = 100 + round * 7;
        std::set<int64_t> seen;
        bool ok = true;
        int64_t n = 0;
        for (auto it = gp::range_map(m); gp::range_next(it);) {
            if (n++ == 0) {
                for (int64_t i = 1000; i < 3000; i++) {
                    gp::set(m, key(i), i); // grows (twice or more) under the range
                }
            }
            int64_t v = it.gp_value();
            ok = ok && it.gp_key() == key(v);
            if (v < old) {
                ok = ok && seen.insert(v).second;
            }
        }
        CHECK(ok && static_cast<int64_t>(seen.size()) == old);
    }
}

// MK: a key whose move clears the source: keys moved (not copied) into the new block while a
// range holds the old one would be -1 there, and the range could not look them up.
struct MK {
    int64_t v = 0;
    MK() = default;
    explicit MK(int64_t x) : v(x) {}
    MK(const MK&) = default;
    MK(MK&& o) noexcept : v(o.v) { o.v = -1; }
    MK& operator=(const MK&) = default;
    bool operator==(const MK& o) const { return v == o.v; }
    std::size_t gp_hash() const { return gp::hash_fields(v); }
};

void test_range_cleared_on_move() {
    for (int round = 0; round < 10; round++) {
        gp::map<MK, int64_t> m = gp::make_map<MK, int64_t>();
        int64_t old = 50 + round * 11;
        for (int64_t i = 0; i < old; i++) {
            gp::set(m, MK(i), i);
        }
        std::set<int64_t> seen;
        bool ok = true;
        int64_t n = 0;
        for (auto it = gp::range_map(m); gp::range_next(it);) {
            if (n++ == 0) {
                for (int64_t i = 1000; i < 2500; i++) {
                    gp::set(m, MK(i), i);
                }
            }
            ok = ok && it.gp_key().v == it.gp_value() && it.gp_key().v >= 0;
            if (it.gp_value() < old) {
                ok = ok && seen.insert(it.gp_value()).second;
            }
        }
        CHECK(ok && static_cast<int64_t>(seen.size()) == old);
    }
}

// --- hashing ---

void test_hash_quality() {
    std::vector<uint64_t> seeds = {0, 1, 0x123456789abcdefu};
    for (uint64_t seed : seeds) {
        for (uint64_t stride : {uint64_t(1), uint64_t(16), uint64_t(4096), uint64_t(1) << 32}) {
            std::vector<int> h1(256, 0), h2(128, 0);
            const int n = 1 << 20;
            for (int i = 0; i < n; i++) {
                uint64_t h = sw::hash_u64(static_cast<uint64_t>(i) * stride, seed);
                h1[(h >> 7) & 255]++;
                h2[h & 127]++;
            }
            bool ok = true;
            for (int c : h1) {
                ok = ok && std::abs(c - n / 256) < n / 256 / 10;
            }
            for (int c : h2) {
                ok = ok && std::abs(c - n / 128) < n / 128 / 10;
            }
            CHECK(ok);
        }
        std::vector<int> h1(256, 0), h2(128, 0);
        const int n = 1 << 19;
        for (int i = 0; i < n; i++) {
            std::string s = "user:" + std::to_string(i) + ":name";
            uint64_t h = sw::hash_mem(s.data(), s.size(), seed);
            h1[(h >> 7) & 255]++;
            h2[h & 127]++;
        }
        bool ok = true;
        for (int c : h1) {
            ok = ok && std::abs(c - n / 256) < n / 256 / 8;
        }
        for (int c : h2) {
            ok = ok && std::abs(c - n / 128) < n / 128 / 8;
        }
        CHECK(ok);
    }
    // Strings of every length from 0 to 200 hash to different values, and the length matters
    // (zero bytes of different lengths).
    std::set<uint64_t> hs;
    std::string z;
    for (int len = 0; len <= 200; len++) {
        hs.insert(sw::hash_mem(z.data(), z.size(), 99));
        z.push_back('\0');
    }
    CHECK(hs.size() == 201);
    // The seed changes the hash of a key, and the maps get different seeds.
    CHECK(sw::hash_u64(5, 1) != sw::hash_u64(5, 2));
    CHECK(sw::hash_mem("abc", 3, 1) != sw::hash_mem("abc", 3, 2));
    std::set<uint64_t> seed_set;
    std::vector<gp::map<int64_t, int64_t>> maps;
    for (int i = 0; i < 200; i++) {
        maps.push_back(gp::make_map<int64_t, int64_t>());
        seed_set.insert(maps.back().gp_get()->gp_seed);
    }
    CHECK(seed_set.size() == 200);
    // Equal keys of a type that hashes through hash_of (floats, structs) hash alike.
    gp::map<double, int64_t> f = gp::make_map<double, int64_t>();
    gp::set(f, 0.0, 1);
    gp::set(f, -0.0, 2);
    CHECK(gp::len(f) == 1 && gp::get(f, 0.0) == 2);
}

// A table of pointers whose addresses all have the same low bits (a pool): still spread.
void test_pointer_keys() {
    struct Obj {
        int64_t x = 0;
    };
    std::vector<gp::ptr<Obj>> objs;
    gp::map<gp::ptr<Obj>, int64_t> m = gp::make_map<gp::ptr<Obj>, int64_t>();
    for (int64_t i = 0; i < 5000; i++) {
        objs.push_back(gp::new_<Obj>());
        gp::set(m, objs.back(), i);
    }
    bool ok = gp::len(m) == 5000 && validate(m);
    for (int64_t i = 0; i < 5000; i++) {
        ok = ok && gp::get(m, objs[static_cast<std::size_t>(i)]) == i;
    }
    CHECK(ok);
}

// --- alignment and layout ---

struct alignas(64) Wide {
    char bytes[64] = {};
    bool operator==(const Wide&) const = default;
};

void test_alignment() {
    gp::map<int64_t, Wide> m = gp::make_map<int64_t, Wide>();
    bool ok = true;
    for (int64_t i = 0; i < 300; i++) {
        Wide& w = gp::ref(m, i);
        ok = ok && reinterpret_cast<std::uintptr_t>(&w) % 64 == 0;
        w.bytes[0] = static_cast<char>(i);
    }
    for (int64_t i = 0; i < 300; i++) {
        ok = ok && gp::get(m, i).bytes[0] == static_cast<char>(i);
    }
    CHECK(ok && validate(m));
    static_assert(sizeof(gp::map<int64_t, int64_t>) == 16);
}

// A bigger table: a million entries, every one found.
void test_big() {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    const int64_t n = 1000000;
    for (int64_t i = 0; i < n; i++) {
        gp::set(m, i * 7919, i);
    }
    bool ok = gp::len(m) == n;
    for (int64_t i = 0; i < n; i += 3) {
        ok = ok && gp::get(m, i * 7919) == i && !std::get<1>(gp::lookup(m, i * 7919 + 1));
    }
    CHECK(ok && validate(m, false));
    int64_t count = 0, sum = 0;
    for (auto it = gp::range_map(m); gp::range_next(it);) {
        count++;
        sum += it.gp_value();
    }
    CHECK(count == n && sum == n * (n - 1) / 2);
}

// --- readers on several threads ---

// Any number of goroutines may read a map at once (gp::get, gp::lookup, range): ranges count
// the table's block with an atomic counter, and nothing else is written. Run under ThreadSanitizer
// by the sanitizer modes that have it.
void test_readers() {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    for (int64_t i = 0; i < 20000; i++) {
        gp::set(m, i, i * 2);
    }
    std::atomic<int> bad{0};
    std::vector<std::thread> ts;
    for (int t = 0; t < 4; t++) {
        ts.emplace_back([&m, &bad, t] {
            for (int round = 0; round < 20; round++) {
                int64_t n = 0, sum = 0;
                for (auto it = gp::range_map(m); gp::range_next(it);) {
                    n++;
                    sum += it.gp_value();
                    if (it.gp_key() * 2 != it.gp_value()) {
                        bad++;
                    }
                }
                if (n != 20000 || sum != 20000LL * 19999) {
                    bad++;
                }
                for (int64_t i = t; i < 20000; i += 97) {
                    if (gp::get(m, i) != i * 2 || std::get<1>(gp::lookup(m, i + 20000))) {
                        bad++;
                    }
                }
            }
        });
    }
    for (std::thread& t : ts) {
        t.join();
    }
    CHECK(bad == 0);
    CHECK(validate(m));
    rt_test::threads_ended();
}

// --- the concurrent write flag ---

// The test hook of the fatal error throws instead of ending the process.
struct Concurrent {
    std::string msg;
};
void concurrent_throws(const char* msg) {
    throw Concurrent{msg};
}

// detects runs f with the flag of a write in progress set by hand, and returns the text of the
// fatal error that it would have died with.
template <typename F>
std::string detects(gp::map<int64_t, int64_t>& m, F f) {
    gp::detail::concurrent_hook = &concurrent_throws;
    m.gp_get()->gp_writing.store(1);
    std::string got = "<none>";
    try {
        f();
    } catch (const Concurrent& c) {
        got = c.msg;
    }
    m.gp_get()->gp_writing.store(0);
    gp::detail::concurrent_hook = nullptr;
    return got;
}

void test_concurrent_flag() {
    gp::map<int64_t, int64_t> m = gp::make_map<int64_t, int64_t>();
    gp::set(m, 1, 1);
    // A write that finds another write in progress, a read, and a range.
    CHECK(detects(m, [&] { gp::set(m, 2, 2); }) == "concurrent map writes");
    CHECK(detects(m, [&] { gp::ref(m, 2)++; }) == "concurrent map writes");
    CHECK(detects(m, [&] { gp::erase(m, 1); }) == "concurrent map writes");
    CHECK(detects(m, [&] { gp::clear(m); }) == "concurrent map writes");
    CHECK(detects(m, [&] { (void)gp::get(m, 1); }) == "concurrent map read and map write");
    CHECK(detects(m, [&] { (void)gp::lookup(m, 1); }) == "concurrent map read and map write");
    {
        auto it = gp::range_map(m);
        CHECK(detects(m, [&] { (void)gp::range_next(it); }) ==
              "concurrent map iteration and map write");
    }
    // Nothing was changed by the detected writes, and without a write in progress nothing is
    // reported.
    CHECK(gp::len(m) == 1 && gp::get(m, 1) == 1 && validate(m));
    gp::set(m, 2, 2);
    CHECK(gp::len(m) == 2);
    // A store that throws (a key whose hash panics) releases the flag.
    gp::map<TK, int64_t> t = gp::make_map<TK, int64_t>();
    TK::throw_hash = 9;
    CHECK(throws([&] { gp::set(t, TK(9), 1); }));
    CHECK(throws([&] { gp::erase(t, TK(9)); }));
    TK::throw_hash = -1;
    gp::set(t, TK(1), 1); // the flag was released by the throw
    CHECK(gp::len(t) == 1);
}

void no_init() {}

void run_tests() {
    uint64_t ops = test_ops();
    test_groups();
    test_differential_int(ops, 64, 1, false);
    test_differential_int(ops, 1000, 2, true);
    test_differential_int(ops, 1 << 20, 3, false);
    test_differential_int(ops / 4, 5, 4, true);
    test_differential_string(ops / 4, 2000, 5);
    test_differential_string(ops / 8, 20, 6);
    test_collisions();
    test_range_models();
    test_tombstone_stays();
    test_nan_range();
    test_churn();
    test_reserve();
    test_prefix_keys();
    test_exceptions();
    test_object_counts();
    test_move_only();
    test_range_copy();
    test_throwing_growth_string_keys();
    test_huge_hint();
    test_range_strings_growth();
    test_range_cleared_on_move();
    test_hash_quality();
    test_pointer_keys();
    test_alignment();
    test_big();
    test_readers();
    test_concurrent_flag();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
