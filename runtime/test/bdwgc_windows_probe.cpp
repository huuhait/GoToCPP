// The Windows probes of the vendored bdwgc 8.2.12 (phase 16 README §2.5, §3 step 16.1, ADR-104):
// the cost of a thread that registers with the collector and exits, and the scan of the static
// data that Windows repeats in every collection.
//
// It is a Windows program (it includes <windows.h>), built by runtime/bdwgcwin_test.go for
// windows/amd64 with internal/build's Bdwgc, cross-compiled on Linux or built natively on the
// Windows PC, and run there on the real PC (never under Wine: Wine's thread creation and address
// space would say nothing about the machine the plan measures). The modes:
//
//   threads <N> <reps>   create N threads one at a time and join each one. "plain" is
//                        std::thread alone; "registered" is the same thread with
//                        GC_get_stack_base, GC_register_my_thread, one GC_malloc and
//                        GC_unregister_my_thread, which is the shape of §2.5. Each rep prints
//                        its total and per-thread cost; the summary line has the median.
//
//   static <mode> <reps> run GC_gcollect reps times with a small live heap (10000 objects of
//                        32 bytes, kept reachable from a stack array, so that the heap scan does
//                        not depend on the static data). mode is one of:
//                          default  GC_set_no_dls(0): every collection re-registers the dynamic
//                                   libraries (VirtualQuery over the address space and the
//                                   writable MEM_IMAGE pages of the executable and every DLL);
//                          nodls    GC_set_no_dls(1): no static roots at all (the plan's
//                                   mitigation without the executable's own sections);
//                          own      GC_set_no_dls(1) plus GC_add_roots of the executable's own
//                                   writable MEM_IMAGE pages: the plan's mitigation.
//                        Each mode also checks whether an object reachable only from a global
//                        survives a collection (a disappearing link to it, with the pointer in
//                        the global disguised so that the global is not itself a root): the
//                        link must stay in default and own, and is expected to clear in nodls.
//
//   info                 the Windows version, the processors, bdwgc's version, and the writable
//                        MEM_IMAGE bytes and regions of the process (what the default scan
//                        walks and pushes).
//
// The numbers are a spike: they say what the PC does today, not a budget. Nothing here is part
// of a product build.
#include <gc.h>

#include <windows.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <thread>
#include <vector>

namespace {

using Clock = std::chrono::steady_clock;

std::atomic<unsigned long long> sink{0};

double millis(Clock::duration d) {
    return std::chrono::duration<double, std::milli>(d).count();
}

// median returns the median of a sorted copy of v.
double median(std::vector<double> v) {
    std::sort(v.begin(), v.end());
    size_t n = v.size();
    if (n == 0) {
        return 0;
    }
    return n % 2 ? v[n / 2] : (v[n / 2 - 1] + v[n / 2]) / 2;
}

// registered_thread is the shape that §2.5 gives every thread that touches GC memory: register,
// allocate, unregister, exit.
void registered_thread() {
    GC_stack_base sb;
    if (GC_get_stack_base(&sb) != GC_SUCCESS || GC_register_my_thread(&sb) != GC_SUCCESS) {
        std::fprintf(stderr, "FAIL: a thread could not register\n");
        std::exit(1);
    }
    void* p = GC_malloc(32);
    sink.fetch_add(static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p)),
                   std::memory_order_relaxed);
    if (GC_unregister_my_thread() != GC_SUCCESS) {
        std::fprintf(stderr, "FAIL: a thread could not unregister\n");
        std::exit(1);
    }
}

void plain_thread() {
    sink.fetch_add(1, std::memory_order_relaxed);
}

// run_threads creates n threads one at a time and joins each one.
void run_threads(int n, bool registered) {
    for (int i = 0; i < n; i++) {
        std::thread t(registered ? registered_thread : plain_thread);
        t.join();
    }
}

int probe_threads(int n, int reps) {
    std::vector<double> plain, reg;
    for (int r = 0; r < reps; r++) {
        auto t0 = Clock::now();
        run_threads(n, false);
        auto t1 = Clock::now();
        run_threads(n, true);
        auto t2 = Clock::now();
        double plainMs = millis(t1 - t0), regMs = millis(t2 - t1);
        plain.push_back(plainMs);
        reg.push_back(regMs);
        std::printf("threads n=%d rep=%d plain_ms=%.3f plain_us_per_thread=%.3f "
                    "registered_ms=%.3f registered_us_per_thread=%.3f\n",
                    n, r + 1, plainMs, plainMs * 1000.0 / n, regMs, regMs * 1000.0 / n);
        std::fflush(stdout);
    }
    std::printf("summary threads n=%d reps=%d plain_median_ms=%.3f plain_median_us_per_thread=%.3f "
                "registered_median_ms=%.3f registered_median_us_per_thread=%.3f\n",
                n, reps, median(plain), median(plain) * 1000.0 / n, median(reg),
                median(reg) * 1000.0 / n);
    return 0;
}

// The live heap of the static probe: pointers on the main thread's stack, so that the objects
// stay reachable whatever the collector does with the static data.
constexpr int liveObjects = 10000;

struct Obj {
    void* field;
    char bytes[24];
};

// static_walk sums the writable, committed MEM_IMAGE bytes of the process, the pages that
// GC_register_dynamic_libraries pushes. It also counts the regions.
void static_walk(unsigned long long* bytes, int* regions) {
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    MEMORY_BASIC_INFORMATION buf;
    *bytes = 0;
    *regions = 0;
    auto p = static_cast<char*>(si.lpMinimumApplicationAddress);
    while (p < static_cast<char*>(si.lpMaximumApplicationAddress)) {
        if (VirtualQuery(p, &buf, sizeof(buf)) != sizeof(buf)) {
            break;
        }
        DWORD protect = buf.Protect;
        bool writable = protect == PAGE_EXECUTE_READWRITE || protect == PAGE_EXECUTE_WRITECOPY ||
                        protect == PAGE_READWRITE || protect == PAGE_WRITECOPY;
        if (buf.State == MEM_COMMIT && writable && buf.Type == MEM_IMAGE) {
            *bytes += buf.RegionSize;
            (*regions)++;
        }
        p += buf.RegionSize;
    }
}

// add_main_module_roots adds the writable MEM_IMAGE pages of this executable, the "executable's
// own sections" of the plan's mitigation. It walks the regions of the main module's allocation
// base only, so no DLL's data is added.
void add_main_module_roots() {
    auto base = reinterpret_cast<char*>(GetModuleHandleW(nullptr));
    MEMORY_BASIC_INFORMATION buf;
    for (auto p = base;;) {
        if (VirtualQuery(p, &buf, sizeof(buf)) != sizeof(buf) || buf.AllocationBase != base) {
            break;
        }
        DWORD protect = buf.Protect;
        bool writable = protect == PAGE_EXECUTE_READWRITE || protect == PAGE_EXECUTE_WRITECOPY ||
                        protect == PAGE_READWRITE || protect == PAGE_WRITECOPY;
        if (buf.State == MEM_COMMIT && writable && buf.Type == MEM_IMAGE) {
            GC_add_roots(p, p + buf.RegionSize);
        }
        p += buf.RegionSize;
    }
}

// The global object of the static probe, and the disguised weak link to it (a plain pointer in a
// global would itself be a root).
void* global_object;
void* weak_slot;
constexpr std::uintptr_t weak_mask = 0x5a5a5a5a5a5a5a5aULL;
bool weak_cleared;

// make_global_object runs in a thread that exits, so that no dead register or stack slot of the
// main thread keeps the object alive.
void make_global_object() {
    GC_stack_base sb;
    if (GC_get_stack_base(&sb) != GC_SUCCESS || GC_register_my_thread(&sb) != GC_SUCCESS) {
        std::fprintf(stderr, "FAIL: the weak-link thread could not register\n");
        std::exit(1);
    }
    global_object = GC_malloc(64);
    std::memset(global_object, 0, 64);
    weak_slot =
        reinterpret_cast<void*>(reinterpret_cast<std::uintptr_t>(global_object) ^ weak_mask);
    if (GC_general_register_disappearing_link(&weak_slot, global_object) != GC_SUCCESS) {
        std::fprintf(stderr, "FAIL: the weak link could not be registered\n");
        std::exit(1);
    }
    if (GC_unregister_my_thread() != GC_SUCCESS) {
        std::fprintf(stderr, "FAIL: the weak-link thread could not unregister\n");
        std::exit(1);
    }
}

int probe_static(const char* mode, int reps) {
    bool noDls = std::strcmp(mode, "nodls") == 0 || std::strcmp(mode, "own") == 0;
    if (std::strcmp(mode, "default") != 0 && std::strcmp(mode, "nodls") != 0 &&
        std::strcmp(mode, "own") != 0) {
        std::fprintf(stderr, "static: unknown mode %s (default, nodls or own)\n", mode);
        return 2;
    }
    unsigned long long walkBytes = 0;
    int walkRegions = 0;
    static_walk(&walkBytes, &walkRegions);
    if (std::strcmp(mode, "own") == 0) {
        add_main_module_roots();
    }
    GC_set_no_dls(noDls ? 1 : 0);

    // The live set: 10000 objects of 32 bytes (320 KB), reachable from this stack. The array
    // itself must be on the stack: an array in malloc memory is not scanned by bdwgc (§2.4), so
    // the collector would free the objects behind the probe's back.
    Obj* keep[liveObjects];
    for (int i = 0; i < liveObjects; i++) {
        keep[i] = static_cast<Obj*>(GC_malloc(sizeof(Obj)));
        std::memset(keep[i], 0, sizeof(Obj));
    }
    // The object that only the static data reaches.
    std::thread(make_global_object).join();

    // Warm up: the first collections expand the heap and start the marker threads.
    for (int i = 0; i < 10; i++) {
        GC_gcollect();
    }
    std::vector<double> times;
    for (int r = 0; r < reps; r++) {
        auto t0 = Clock::now();
        GC_gcollect();
        auto t1 = Clock::now();
        times.push_back(millis(t1 - t0));
    }
    // Read the stack array after the collections, so that the compiler keeps it live.
    unsigned long long touch = 0;
    for (int i = 0; i < liveObjects; i += 997) {
        touch += reinterpret_cast<std::uintptr_t>(keep[i]->field);
    }
    sink.fetch_add(touch, std::memory_order_relaxed);

    // Read the global after the collections: without a read, the compiler is free to drop the
    // store that makes the object reachable from the static data (and the probe would measure
    // nothing).
    sink.fetch_add(reinterpret_cast<std::uintptr_t>(global_object), std::memory_order_relaxed);
    // The static roots: the weak link stays registered while its object survives.
    GC_gcollect();
    weak_cleared = weak_slot == nullptr;

    std::vector<double> sorted = times;
    std::sort(sorted.begin(), sorted.end());
    std::printf("static mode=%s reps=%d median_ms=%.3f min_ms=%.3f max_ms=%.3f "
                "image_writable_bytes=%llu image_writable_regions=%d "
                "global_survived=%s no_dls=%d\n",
                mode, reps, median(times), sorted.front(), sorted.back(), walkBytes, walkRegions,
                weak_cleared ? "no" : "yes", GC_get_no_dls());
    std::fflush(stdout);
    return 0;
}

int probe_info() {
    unsigned version = GC_get_version();
    std::printf("gc_version=%u.%u.%u processors=%u\n", (version >> 16) & 0xff,
                (version >> 8) & 0xff, version & 0xff,
                static_cast<unsigned>(std::thread::hardware_concurrency()));
    unsigned long long bytes = 0;
    int regions = 0;
    static_walk(&bytes, &regions);
    std::printf("info image_writable_bytes=%llu image_writable_regions=%d\n", bytes, regions);
    return 0;
}

} // namespace

int main(int argc, char** argv) {
    if (argc < 2) {
        std::fprintf(stderr,
                     "usage: %s threads <N> <reps> | static <default|nodls|own> <reps> | info\n",
                     argv[0]);
        return 2;
    }
    std::string mode = argv[1];

    GC_INIT();
    GC_allow_register_threads();
    if (mode == "info") {
        return probe_info();
    }
    if (argc < 4) {
        std::fprintf(stderr, "%s: %s needs two arguments\n", argv[0], mode.c_str());
        return 2;
    }
    int reps = std::atoi(argv[3]);
    if (reps <= 0) {
        std::fprintf(stderr, "%s: bad rep count %s\n", argv[0], argv[3]);
        return 2;
    }
    int n = std::atoi(argv[2]);
    if (mode == "threads" && n <= 0) {
        std::fprintf(stderr, "%s: bad thread count %s\n", argv[0], argv[2]);
        return 2;
    }

    if (mode == "threads") {
        return probe_threads(n, reps);
    }
    if (mode == "static") {
        return probe_static(argv[2], reps);
    }
    std::fprintf(stderr, "%s: unknown mode %s\n", argv[0], mode.c_str());
    return 2;
}
