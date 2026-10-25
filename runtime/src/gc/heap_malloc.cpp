// gocpp/gc/heap_malloc.cpp: the malloc-backed backend of gp::gc::heap (phase-16 README §2.4).
//
// This is the spike backend of job 16.1c, not a collector: alloc and alloc_atomic are calloc,
// so memory is zeroed but never freed; a root is one malloc'd cell that holds the pointer;
// collect only counts; enter_thread, leave_thread and blocking do nothing, because there is no
// collector to register a thread with or to stop. The bdwgc backend of job 16.1d replaces this
// file behind the same interface (heap.hpp), and the walking skeleton's tests must then pass
// with it unchanged.
//
// The backend is not thread-safe: the counters are plain integers. A program of job 16.1c has
// one thread; job 16.4 registers threads and gives the counters their atomics.
#include <gocpp/nofortify.h>
#include <gocpp/gc/heap.hpp>

#include <cstdlib>
#include <new>

namespace gp::gc {

namespace {

// A root_cell is what make_root returns: a cell in the malloc heap that holds the pointer the
// collector must keep alive. bdwgc uses GC_MALLOC_UNCOLLECTABLE for the same shape.
struct root_cell {
    void* gp_pointer;
};

class malloc_heap final : public heap {
public:
    void* alloc(std::size_t size) override { return take(size); }
    void* alloc_atomic(std::size_t size) override { return take(size); }

    void enter_thread() override {}
    void leave_thread() override {}

    void blocking(blocking_fn fn, void* data) override { fn(data); }

    void* make_root(void* p) override { return new root_cell{p}; }
    void set_root(void* handle, void* p) override {
        static_cast<root_cell*>(handle)->gp_pointer = p;
    }
    void* root_value(void* handle) const override {
        return static_cast<const root_cell*>(handle)->gp_pointer;
    }
    void drop_root(void* handle) override { delete static_cast<root_cell*>(handle); }

    void collect() override { gp_stats.collections++; }
    statistics stats() const override { return gp_stats; }

    const char* name() const override { return "malloc"; }

private:
    // take allocates size zeroed bytes in the malloc heap. Go's zero value rule needs the
    // zeroing (README §2.4: pointer-free allocations are zeroed unless every byte is written).
    void* take(std::size_t size) {
        void* p = std::calloc(1, size == 0 ? 1 : size);
        if (p == nullptr) {
            throw std::bad_alloc();
        }
        gp_stats.allocs++;
        gp_stats.bytes += size;
        return p;
    }

    statistics gp_stats{};
};

} // namespace

// process_heap returns the one backend of the process. The static is a function-local one: a
// namespace-scope object of a polymorphic class would need a dynamic initializer (its vtable
// pointer), and the runtime's objects must add no static initializer (TestRuntimeObjectsHaveNo
// StaticInit). It is constant-initialized up to its vtable pointer, which is set on first use.
heap& process_heap() {
    static malloc_heap the_heap;
    return the_heap;
}

} // namespace gp::gc
