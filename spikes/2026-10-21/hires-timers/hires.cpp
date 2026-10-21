// Throwaway spike: how late do different waits end on Windows?
#include <windows.h>
#include <algorithm>
#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <cstdio>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#ifndef CREATE_WAITABLE_TIMER_HIGH_RESOLUTION
#define CREATE_WAITABLE_TIMER_HIGH_RESOLUTION 0x2
#endif

using namespace std::chrono;
typedef LONG(NTAPI* NtQ)(PULONG, PULONG, PULONG);
typedef LONG(NTAPI* NtS)(ULONG, BOOLEAN, PULONG);
NtQ ntq;
NtS nts;
typedef UINT(WINAPI* tp)(UINT);
tp tbp, tep;

int64_t qpc_freq;
int64_t now_ns() {
    LARGE_INTEGER c;
    QueryPerformanceCounter(&c);
    return (c.QuadPart / qpc_freq) * 1000000000LL +
           (c.QuadPart % qpc_freq) * 1000000000LL / qpc_freq;
}

void resolution(const char* tag) {
    ULONG mn, mx, cur;
    ntq(&mn, &mx, &cur);
    printf("res %s min=%lu max=%lu cur=%lu (100ns units)\n", tag, mn, mx, cur);
}

HANDLE make_hr() {
    HANDLE h = CreateWaitableTimerExW(nullptr, nullptr, CREATE_WAITABLE_TIMER_HIGH_RESOLUTION,
                                      TIMER_ALL_ACCESS);
    if (!h) {
        printf("no hr timer: %lu\n", GetLastError());
        exit(2);
    }
    return h;
}

void set_hr(HANDLE h, int64_t ns) {
    LARGE_INTEGER due;
    due.QuadPart = -(ns / 100);
    if (due.QuadPart == 0)
        due.QuadPart = -1;
    SetWaitableTimerEx(h, &due, 0, nullptr, nullptr, nullptr, 0);
}

void report(const char* kind, int64_t want_ns, std::vector<int64_t> late, double cpu_ms_per_wait) {
    std::sort(late.begin(), late.end());
    size_t n = late.size();
    printf("%-14s %6lld %4zu p10=%5lld p25=%5lld p75=%5lld p90=%5lld ", kind,
           (long long)(want_ns / 1000), n, (long long)late[n / 10] / 1000,
           (long long)late[n / 4] / 1000, (long long)late[n * 3 / 4] / 1000,
           (long long)late[n * 9 / 10] / 1000);
    printf("median=%6lld p99=%6lld max=%6lld min=%6lld cpu/wait=%.3fms\n",
           (long long)late[n / 2] / 1000, (long long)late[(n * 99 + 99) / 100 - 1] / 1000,
           (long long)late.back() / 1000, (long long)late.front() / 1000, cpu_ms_per_wait);
}

double cpu_ms() {
    FILETIME c, e, k, u;
    GetProcessTimes(GetCurrentProcess(), &c, &e, &k, &u);
    auto v = [](FILETIME f) {
        return ((uint64_t(f.dwHighDateTime) << 32) | f.dwLowDateTime) / 10000.0;
    };
    return v(k) + v(u);
}

template <class F>
void run(const char* kind, int64_t want_ns, int n, F f) {
    std::vector<int64_t> late;
    double c0 = cpu_ms();
    for (int i = 0; i < n; i++) {
        int64_t start = now_ns();
        f(start, want_ns);
        late.push_back(now_ns() - start - want_ns);
    }
    report(kind, want_ns, late, (cpu_ms() - c0) / n);
}

std::mutex mu;
std::condition_variable cv;

void cv_wait(int64_t start, int64_t want) {
    std::unique_lock<std::mutex> lk(mu);
    for (;;) {
        int64_t left = start + want - now_ns();
        if (left <= 0)
            return;
        cv.wait_for(lk, nanoseconds(left));
    }
}

HANDLE hr;
void hr_wait(int64_t start, int64_t want) {
    int64_t left = start + want - now_ns();
    if (left <= 0)
        return;
    set_hr(hr, left);
    WaitForSingleObject(hr, INFINITE);
}

HANDLE ev;
void hr_multi(int64_t start, int64_t want) {
    int64_t left = start + want - now_ns();
    if (left <= 0)
        return;
    set_hr(hr, left);
    HANDLE hs[2] = {ev, hr};
    WaitForMultipleObjects(2, hs, FALSE, INFINITE);
}

// hybrid: cv for all but 32 ms, then hr timer on event pair
void hybrid(int64_t start, int64_t want) {
    const int64_t margin = 32'000'000;
    for (;;) {
        int64_t left = start + want - now_ns();
        if (left <= 0)
            return;
        if (left > 2 * margin) {
            std::unique_lock<std::mutex> lk(mu);
            cv.wait_for(lk, nanoseconds(left - margin));
            continue;
        }
        set_hr(hr, left);
        HANDLE hs[2] = {ev, hr};
        WaitForMultipleObjects(2, hs, FALSE, INFINITE);
        return;
    }
}

// timer thread hop: a thread waits on the hr timer, then signals the cv of the waiter.
struct hop_state {
    std::mutex m;
    std::condition_variable c;
    int64_t due = 0; // 0: none
    bool fired = false;
    bool stop = false;
    HANDLE timer;
    HANDLE wake; // timer thread's wake event
};
hop_state hs_;
void hop_thread() {
    HANDLE hs[2] = {hs_.wake, hs_.timer};
    for (;;) {
        DWORD r = WaitForMultipleObjects(2, hs, FALSE, INFINITE);
        std::unique_lock<std::mutex> lk(hs_.m);
        if (hs_.stop)
            return;
        if (r == WAIT_OBJECT_0 + 1) {
            hs_.fired = true;
            hs_.c.notify_one();
        }
    }
}
void hop_wait(int64_t start, int64_t want) {
    int64_t left = start + want - now_ns();
    if (left <= 0)
        return;
    std::unique_lock<std::mutex> lk(hs_.m);
    hs_.fired = false;
    set_hr(hs_.timer, left);
    hs_.c.wait(lk, [] { return hs_.fired; });
}

// spin on the last part
void cv_spin(int64_t start, int64_t want, int64_t spin_ns, bool yield) {
    for (;;) {
        int64_t left = start + want - now_ns();
        if (left <= 0)
            return;
        if (left > spin_ns + 32'000'000) {
            std::unique_lock<std::mutex> lk(mu);
            cv.wait_for(lk, nanoseconds(left - spin_ns - 32'000'000));
        } else if (left > spin_ns) {
            // Coarse part: ordinary tick, but ends within 15.6 ms: do it in cv of (left - spin -
            // 16ms)
            std::unique_lock<std::mutex> lk(mu);
            int64_t w = left - spin_ns - 16'000'000;
            if (w > 0)
                cv.wait_for(lk, nanoseconds(w));
            else {
                // spin phase
                while (start + want > now_ns()) {
                    if (yield)
                        SwitchToThread();
                }
                return;
            }
        } else {
            while (start + want > now_ns()) {
                if (yield)
                    SwitchToThread();
            }
            return;
        }
    }
}

// hr timer wait through the high-resolution timer for a cv-based waiter whose waker is another
// thread (measures wake latency of a notify across threads)
void notify_latency() {
    std::vector<int64_t> late;
    std::mutex m;
    std::condition_variable c;
    int64_t t_sent = 0;
    bool go = false;
    std::atomic<bool> stop{false};
    std::thread w([&] {
        std::unique_lock<std::mutex> lk(m);
        while (!stop) {
            c.wait(lk, [&] { return go || stop; });
            if (stop)
                break;
            late.push_back(now_ns() - t_sent);
            go = false;
        }
    });
    for (int i = 0; i < 200; i++) {
        Sleep(2);
        {
            std::lock_guard<std::mutex> lk(m);
            t_sent = now_ns();
            go = true;
            c.notify_one();
        }
    }
    Sleep(5);
    {
        std::lock_guard<std::mutex> lk(m);
        stop = true;
        c.notify_one();
    }
    w.join();
    report("cv-notify", 0, late, 0);
}

int main(int argc, char** argv) {
    LARGE_INTEGER f;
    QueryPerformanceFrequency(&f);
    qpc_freq = f.QuadPart;
    HMODULE nt = GetModuleHandleW(L"ntdll.dll");
    ntq = (NtQ)(void*)GetProcAddress(nt, "NtQueryTimerResolution");
    nts = (NtS)(void*)GetProcAddress(nt, "NtSetTimerResolution");
    HMODULE wm = LoadLibraryExW(L"winmm.dll", nullptr, LOAD_LIBRARY_SEARCH_SYSTEM32);
    tbp = (tp)(void*)GetProcAddress(wm, "timeBeginPeriod");
    tep = (tp)(void*)GetProcAddress(wm, "timeEndPeriod");
    std::string which = argc > 1 ? argv[1] : "all";
    int spinners = argc > 2 ? atoi(argv[2]) : 0;
    std::atomic<bool> stopspin{false};
    std::vector<std::thread> sp;
    for (int i = 0; i < spinners; i++)
        sp.emplace_back([&] {
            while (!stopspin) {
            }
        });
    printf("which=%s spinners=%d\n", which.c_str(), spinners);
    hr = make_hr();
    ev = CreateEventW(nullptr, FALSE, FALSE, nullptr);
    hs_.timer = make_hr();
    hs_.wake = CreateEventW(nullptr, FALSE, FALSE, nullptr);
    std::thread ht(hop_thread);
    resolution("start");

    const int64_t ms = 1'000'000;
    int64_t lens[] = {1 * ms, 2 * ms, 5 * ms, 15 * ms, 100 * ms};
    auto n_for = [](int64_t w) { return w >= 100'000'000 ? 20 : 200; };

    bool all = which == "all";
    if (all || which == "cv") {
        for (auto w : lens)
            run("cv", w, n_for(w), cv_wait);
        resolution("after cv");
    }
    if (all || which == "tbp") {
        tbp(1);
        resolution("tbp1");
        for (auto w : lens)
            run("cv+tbp1", w, n_for(w), cv_wait);
        tep(1);
    }
    if (all || which == "nt05") {
        ULONG cur;
        nts(5000, TRUE, &cur);
        resolution("nt0.5");
        for (auto w : lens)
            run("cv+nt0.5ms", w, n_for(w), cv_wait);
        nts(5000, FALSE, &cur);
        resolution("nt released");
    }
    if (all || which == "hr") {
        for (auto w : lens)
            run("hr", w, n_for(w), hr_wait);
        for (auto w : lens)
            run("hr-multi", w, n_for(w), hr_multi);
        resolution("after hr");
    }
    if (which == "v2") {
        for (auto w : {1 * ms, 5 * ms, 15 * ms})
            run("hr", w, 200, hr_wait);
        ULONG cur;
        nts(5000, TRUE, &cur);
        resolution("nt0.5 held");
        for (auto w : {1 * ms, 5 * ms, 15 * ms})
            run("hr+nt0.5", w, 200, hr_wait);
        nts(5000, FALSE, &cur);
        tbp(1);
        for (auto w : {1 * ms, 5 * ms, 15 * ms})
            run("hr+tbp1", w, 200, hr_wait);
        tep(1);
        // pre-roll: hr timer fires P early, spin to the deadline
        for (int64_t P : {250'000LL, 500'000LL, 1'000'000LL}) {
            for (auto w : {1 * ms, 5 * ms, 15 * ms}) {
                char name[32];
                snprintf(name, sizeof name, "hr-pre%lldus", (long long)P / 1000);
                run(name, w, 200, [P](int64_t s, int64_t ww) {
                    int64_t left = s + ww - now_ns() - P;
                    if (left > 0) {
                        set_hr(hr, left);
                        WaitForSingleObject(hr, INFINITE);
                    }
                    while (s + ww > now_ns())
                        YieldProcessor();
                });
            }
        }
        // a plain (non-hr) waitable timer with tbp(1)
        HANDLE plain = CreateWaitableTimerExW(nullptr, nullptr, 0, TIMER_ALL_ACCESS);
        tbp(1);
        for (auto w : {1 * ms, 5 * ms, 15 * ms})
            run("plaintimer+tbp", w, 200, [plain](int64_t s, int64_t ww) {
                LARGE_INTEGER due;
                due.QuadPart = -((s + ww - now_ns()) / 100);
                SetWaitableTimerEx(plain, &due, 0, nullptr, nullptr, nullptr, 0);
                WaitForSingleObject(plain, INFINITE);
            });
        tep(1);
        // wake latency: a blocked waiter woken by SetEvent vs by setting its hr timer to the past
        for (int mode = 0; mode < 2; mode++) {
            std::vector<int64_t> late;
            HANDLE wt = make_hr();
            HANDLE we = CreateEventW(nullptr, FALSE, FALSE, nullptr);
            HANDLE armed = CreateEventW(nullptr, FALSE, FALSE, nullptr);
            std::atomic<int64_t> t_sent{0};
            std::atomic<bool> stop{false};
            std::thread w([&] {
                for (;;) {
                    set_hr(wt, 10'000'000'000LL);
                    SetEvent(armed);
                    HANDLE hs[2] = {we, wt};
                    WaitForMultipleObjects(mode == 0 ? 2 : 1, mode == 0 ? hs : &wt, FALSE,
                                           INFINITE);
                    int64_t t = now_ns();
                    if (stop)
                        break;
                    late.push_back(t - t_sent);
                    SetEvent(armed); // done, will rearm
                    Sleep(0);
                }
            });
            for (int i = 0; i < 200; i++) {
                WaitForSingleObject(armed, INFINITE);
                Sleep(1);
                t_sent = now_ns();
                if (mode == 0)
                    SetEvent(we);
                else {
                    LARGE_INTEGER d;
                    d.QuadPart = 1;
                    SetWaitableTimerEx(wt, &d, 0, nullptr, nullptr, nullptr, 0);
                }
                WaitForSingleObject(armed, INFINITE); // consumed: wait for the rearm signal
            }
            stop = true;
            SetEvent(we);
            {
                LARGE_INTEGER d;
                d.QuadPart = 1;
                SetWaitableTimerEx(wt, &d, 0, nullptr, nullptr, nullptr, 0);
            }
            w.join();
            report(mode == 0 ? "wake-event" : "wake-timerpast", 0, late, 0);
        }
    }
    if (which == "v3") {
        // random phase: a busy jitter of 0..1.9 ms before every measured wait
        uint64_t rng = 88172645463325252ULL;
        auto rnd = [&]() {
            rng ^= rng << 13;
            rng ^= rng >> 7;
            rng ^= rng << 17;
            return rng;
        };
        for (int64_t P : {0LL, 250'000LL, 500'000LL, 750'000LL, 1'000'000LL}) {
            for (auto w : {1 * ms, 5 * ms, 15 * ms, 100 * ms}) {
                int n = w >= 100 * ms ? 60 : 300;
                std::vector<int64_t> late;
                int64_t spin_total = 0;
                for (int i = 0; i < n; i++) {
                    int64_t jit = now_ns() + (int64_t)(rnd() % 1900000);
                    while (now_ns() < jit)
                        YieldProcessor();
                    int64_t start = now_ns();
                    int64_t dl = start + w;
                    int64_t left = dl - start - P;
                    if (left > 0) {
                        set_hr(hr, left);
                        WaitForSingleObject(hr, INFINITE);
                    }
                    int64_t t0 = now_ns();
                    while (now_ns() < dl)
                        YieldProcessor();
                    int64_t t1 = now_ns();
                    spin_total += t1 - t0;
                    late.push_back(t1 - dl);
                }
                char name[32];
                snprintf(name, sizeof name, "rnd-pre%lldus", (long long)P / 1000);
                report(name, w, late, spin_total / 1e6 / n);
            }
        }
    }
    if (which == "v4") {
        uint64_t rng = 88172645463325252ULL;
        auto rnd = [&]() {
            rng ^= rng << 13;
            rng ^= rng >> 7;
            rng ^= rng << 17;
            return rng;
        };
        auto pct = [](std::vector<int64_t>& v, int p) {
            return v[std::min(v.size() - 1, v.size() * p / 100)] / 1000;
        };
        for (int round = 0; round < 2; round++)
            for (int64_t P : {0LL, 150'000LL, 300'000LL, 600'000LL, 1'000'000LL}) {
                int64_t w = 3 * ms;
                std::vector<int64_t> late, raw;
                int64_t spin_total = 0;
                for (int i = 0; i < 1500; i++) {
                    int64_t jit = now_ns() + (int64_t)(rnd() % 1900000);
                    while (now_ns() < jit)
                        YieldProcessor();
                    int64_t start = now_ns();
                    int64_t dl = start + w;
                    int64_t tdue = dl - P;
                    set_hr(hr, tdue - now_ns());
                    WaitForSingleObject(hr, INFINITE);
                    int64_t t0 = now_ns();
                    raw.push_back(t0 - tdue);
                    while (now_ns() < dl)
                        YieldProcessor();
                    int64_t t1 = now_ns();
                    spin_total += t1 - t0;
                    late.push_back(t1 - dl);
                }
                std::sort(late.begin(), late.end());
                std::sort(raw.begin(), raw.end());
                printf("P=%4lld final p50=%4lld p75=%4lld p90=%4lld p95=%4lld p99=%4lld | raw "
                       "p50=%4lld p90=%4lld p99=%4lld | spin avg %.0f us\n",
                       (long long)P / 1000, (long long)pct(late, 50), (long long)pct(late, 75),
                       (long long)pct(late, 90), (long long)pct(late, 95), (long long)pct(late, 99),
                       (long long)pct(raw, 50), (long long)pct(raw, 90), (long long)pct(raw, 99),
                       spin_total / 1000.0 / 1500);
            }
    }
    if (all || which == "hybrid") {
        for (auto w : lens)
            run("hybrid", w, n_for(w), hybrid);
    }
    if (all || which == "hop") {
        for (auto w : lens)
            run("hop", w, n_for(w), hop_wait);
        notify_latency();
    }
    if (all || which == "spin") {
        for (auto w : lens)
            run("cv+spin1ms", w, n_for(w),
                [](int64_t s, int64_t w2) { cv_spin(s, w2, 1'000'000, false); });
        for (auto w : lens)
            run("cv+yield1ms", w, n_for(w),
                [](int64_t s, int64_t w2) { cv_spin(s, w2, 1'000'000, true); });
    }
    stopspin = true;
    {
        std::lock_guard<std::mutex> lk(hs_.m);
        hs_.stop = true;
    }
    SetEvent(hs_.wake);
    ht.join();
    for (auto& t : sp)
        t.join();
    printf("done\n");
    return 0;
}
