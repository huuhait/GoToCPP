// Struct layouts of the GC engine, checked: the C++ structs that the emitter writes for Go
// structs (the fields in source order), with Go's sizes, alignments and offsets.
//
// GENERATED from internal/check/gcsizes.go (gcLayoutSrc): don't edit it, run go test
// ./internal/check -run TestGCSizesHeader -update-sizes. runtime/layout_test.go compiles it with
// g++ and clang++, and TestGCLayoutShapes checks the Go numbers against the real standard
// types. It has no code to run.
#include <gocpp/gc/rt.hpp>
#include <gocpp/gc/sizes.hpp>

#include <cstddef>
#include <cstdint>

namespace gc_layout_test {

struct empty {
    static constexpr auto gp_fields = gp::gc::fields();
};
GP_GC_LAYOUT(empty, 0, 1);
static_assert(gp::gc::detail::is_comparable_v<empty> == true, "empty: Go comparability");

struct zLast {
    int64_t A;
    [[no_unique_address]] gp::gc::zero_field<1> Z;
    gp::gc::padding<1> gp_tail;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&zLast::A, "A"), gp::gc::field(&zLast::Z, "Z"));
};
GP_GC_LAYOUT(zLast, 16, 8);
static_assert(gp::gc::detail::is_comparable_v<zLast> == true, "zLast: Go comparability");
GP_GC_FIELD_AT(zLast, A, 0);

struct zFirst {
    [[no_unique_address]] gp::gc::zero_field<0> Z;
    int64_t A;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&zFirst::Z, "Z"), gp::gc::field(&zFirst::A, "A"));
};
GP_GC_LAYOUT(zFirst, 8, 8);
static_assert(gp::gc::detail::is_comparable_v<zFirst> == true, "zFirst: Go comparability");
GP_GC_FIELD_AT(zFirst, A, 0);

struct zz {
    [[no_unique_address]] gp::gc::zero_field<0> Z1;
    [[no_unique_address]] gp::gc::zero_field<1> Z2;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&zz::Z1, "Z1"), gp::gc::field(&zz::Z2, "Z2"));
};
GP_GC_LAYOUT(zz, 0, 1);
static_assert(gp::gc::detail::is_comparable_v<zz> == true, "zz: Go comparability");

struct arrZLast {
    int32_t A;
    gp::gc::padding<4> gp_pad1;
    [[no_unique_address]] gp::gc::zero_field<1, 8> Z;
    gp::gc::padding<1> gp_tail;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&arrZLast::A, "A"), gp::gc::field(&arrZLast::Z, "Z"));
};
GP_GC_LAYOUT(arrZLast, 16, 8);
static_assert(gp::gc::detail::is_comparable_v<arrZLast> == true, "arrZLast: Go comparability");
GP_GC_FIELD_AT(arrZLast, A, 0);

struct arrZFirst {
    [[no_unique_address]] gp::gc::zero_field<0, 8> Z;
    int32_t A;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&arrZFirst::Z, "Z"), gp::gc::field(&arrZFirst::A, "A"));
};
GP_GC_LAYOUT(arrZFirst, 8, 8);
static_assert(gp::gc::detail::is_comparable_v<arrZFirst> == true, "arrZFirst: Go comparability");
GP_GC_FIELD_AT(arrZFirst, A, 0);

struct arrZMid {
    int32_t A;
    gp::gc::padding<4> gp_pad1;
    [[no_unique_address]] gp::gc::zero_field<1, 8> Z;
    int32_t B;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&arrZMid::A, "A"), gp::gc::field(&arrZMid::Z, "Z"),
                       gp::gc::field(&arrZMid::B, "B"));
};
GP_GC_LAYOUT(arrZMid, 16, 8);
static_assert(gp::gc::detail::is_comparable_v<arrZMid> == true, "arrZMid: Go comparability");
GP_GC_FIELD_AT(arrZMid, A, 0);
GP_GC_FIELD_AT(arrZMid, B, 8);

struct zeroSizeNamed {
    [[no_unique_address]] gp::gc::zero_field<0, 8> z;
    static constexpr auto gp_fields = gp::gc::fields(gp::gc::field(&zeroSizeNamed::z, "z"));
};
GP_GC_LAYOUT(zeroSizeNamed, 0, 8);
static_assert(gp::gc::detail::is_comparable_v<zeroSizeNamed> == true,
              "zeroSizeNamed: Go comparability");

struct nestedZero {
    int32_t A;
    gp::gc::padding<4> gp_pad1;
    [[no_unique_address]] gp::gc::zero_field<1, 8> Z;
    int32_t B;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&nestedZero::A, "A"), gp::gc::field(&nestedZero::Z, "Z"),
                       gp::gc::field(&nestedZero::B, "B"));
};
GP_GC_LAYOUT(nestedZero, 16, 8);
static_assert(gp::gc::detail::is_comparable_v<nestedZero> == true, "nestedZero: Go comparability");
GP_GC_FIELD_AT(nestedZero, A, 0);
GP_GC_FIELD_AT(nestedZero, B, 8);

struct syncMutexInner {
    int32_t state;
    uint32_t sema;
    static constexpr auto gp_fields = gp::gc::fields(gp::gc::field(&syncMutexInner::state, "state"),
                                                     gp::gc::field(&syncMutexInner::sema, "sema"));
};
GP_GC_LAYOUT(syncMutexInner, 8, 4);
static_assert(gp::gc::detail::is_comparable_v<syncMutexInner> == true,
              "syncMutexInner: Go comparability");
GP_GC_FIELD_AT(syncMutexInner, state, 0);
GP_GC_FIELD_AT(syncMutexInner, sema, 4);

struct syncMutex {
    [[no_unique_address]] gp::gc::zero_field<0> noCopy;
    syncMutexInner mu;
    static constexpr auto gp_fields = gp::gc::fields(gp::gc::field(&syncMutex::noCopy, "noCopy"),
                                                     gp::gc::field(&syncMutex::mu, "mu"));
};
GP_GC_LAYOUT(syncMutex, 8, 4);
static_assert(gp::gc::detail::is_comparable_v<syncMutex> == true, "syncMutex: Go comparability");
GP_GC_FIELD_AT(syncMutex, mu, 0);

struct syncWaitGroup {
    [[no_unique_address]] gp::gc::zero_field<0> noCopy;
    uint64_t state;
    uint32_t sema;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&syncWaitGroup::noCopy, "noCopy"),
        gp::gc::field(&syncWaitGroup::state, "state"), gp::gc::field(&syncWaitGroup::sema, "sema"));
};
GP_GC_LAYOUT(syncWaitGroup, 16, 8);
static_assert(gp::gc::detail::is_comparable_v<syncWaitGroup> == true,
              "syncWaitGroup: Go comparability");
GP_GC_FIELD_AT(syncWaitGroup, state, 0);
GP_GC_FIELD_AT(syncWaitGroup, sema, 8);

struct syncRWMutex {
    syncMutexInner w;
    uint32_t writerSem;
    uint32_t readerSem;
    int32_t readerCount;
    int32_t readerWait;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&syncRWMutex::w, "w"), gp::gc::field(&syncRWMutex::writerSem, "writerSem"),
        gp::gc::field(&syncRWMutex::readerSem, "readerSem"),
        gp::gc::field(&syncRWMutex::readerCount, "readerCount"),
        gp::gc::field(&syncRWMutex::readerWait, "readerWait"));
};
GP_GC_LAYOUT(syncRWMutex, 24, 4);
static_assert(gp::gc::detail::is_comparable_v<syncRWMutex> == true,
              "syncRWMutex: Go comparability");
GP_GC_FIELD_AT(syncRWMutex, w, 0);
GP_GC_FIELD_AT(syncRWMutex, writerSem, 8);
GP_GC_FIELD_AT(syncRWMutex, readerSem, 12);
GP_GC_FIELD_AT(syncRWMutex, readerCount, 16);
GP_GC_FIELD_AT(syncRWMutex, readerWait, 20);

struct syncOnce {
    [[no_unique_address]] gp::gc::zero_field<0> noCopy;
    uint32_t done;
    syncMutexInner m;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&syncOnce::noCopy, "noCopy"),
                       gp::gc::field(&syncOnce::done, "done"), gp::gc::field(&syncOnce::m, "m"));
};
GP_GC_LAYOUT(syncOnce, 12, 4);
static_assert(gp::gc::detail::is_comparable_v<syncOnce> == true, "syncOnce: Go comparability");
GP_GC_FIELD_AT(syncOnce, done, 0);
GP_GC_FIELD_AT(syncOnce, m, 4);

struct syncPool {
    [[no_unique_address]] gp::gc::zero_field<0> noCopy;
    int64_t* local;
    uint64_t localSize;
    int64_t* victim;
    uint64_t victimSize;
    gp::gc::func<gp::gc::eface()> New;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&syncPool::noCopy, "noCopy"), gp::gc::field(&syncPool::local, "local"),
        gp::gc::field(&syncPool::localSize, "localSize"),
        gp::gc::field(&syncPool::victim, "victim"),
        gp::gc::field(&syncPool::victimSize, "victimSize"), gp::gc::field(&syncPool::New, "New"));
};
GP_GC_LAYOUT(syncPool, 40, 8);
static_assert(gp::gc::detail::is_comparable_v<syncPool> == false, "syncPool: Go comparability");
GP_GC_FIELD_AT(syncPool, local, 0);
GP_GC_FIELD_AT(syncPool, localSize, 8);
GP_GC_FIELD_AT(syncPool, victim, 16);
GP_GC_FIELD_AT(syncPool, victimSize, 24);
GP_GC_FIELD_AT(syncPool, New, 32);

struct atomicPointer {
    [[no_unique_address]] gp::gc::zero_field<0, 8> z;
    [[no_unique_address]] gp::gc::zero_field<1> noCopy;
    int64_t* v;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&atomicPointer::z, "z"), gp::gc::field(&atomicPointer::noCopy, "noCopy"),
        gp::gc::field(&atomicPointer::v, "v"));
};
GP_GC_LAYOUT(atomicPointer, 8, 8);
static_assert(gp::gc::detail::is_comparable_v<atomicPointer> == true,
              "atomicPointer: Go comparability");
GP_GC_FIELD_AT(atomicPointer, v, 0);

struct atomicInt64 {
    [[no_unique_address]] gp::gc::zero_field<0> noCopy;
    [[no_unique_address]] gp::gc::zero_field<1, 8> align64;
    int64_t v;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&atomicInt64::noCopy, "noCopy"),
        gp::gc::field(&atomicInt64::align64, "align64"), gp::gc::field(&atomicInt64::v, "v"));
};
GP_GC_LAYOUT(atomicInt64, 8, 8);
static_assert(gp::gc::detail::is_comparable_v<atomicInt64> == true,
              "atomicInt64: Go comparability");
GP_GC_FIELD_AT(atomicInt64, v, 0);

struct weakPointer {
    [[no_unique_address]] gp::gc::zero_field<0, 8> z;
    int64_t* u;
    static constexpr auto gp_fields =
        gp::gc::fields(gp::gc::field(&weakPointer::z, "z"), gp::gc::field(&weakPointer::u, "u"));
};
GP_GC_LAYOUT(weakPointer, 8, 8);
static_assert(gp::gc::detail::is_comparable_v<weakPointer> == true,
              "weakPointer: Go comparability");
GP_GC_FIELD_AT(weakPointer, u, 0);

struct slogValue {
    [[no_unique_address]] gp::gc::zero_field<0, 8, false> noCompare;
    uint64_t num;
    gp::gc::eface any;
    static constexpr auto gp_fields = gp::gc::fields(
        gp::gc::field(&slogValue::noCompare, "noCompare"), gp::gc::field(&slogValue::num, "num"),
        gp::gc::field(&slogValue::any, "any"));
};
GP_GC_LAYOUT(slogValue, 24, 8);
static_assert(gp::gc::detail::is_comparable_v<slogValue> == false, "slogValue: Go comparability");
GP_GC_FIELD_AT(slogValue, num, 0);
GP_GC_FIELD_AT(slogValue, any, 8);

struct slogAttr {
    gp::gc::string Key;
    slogValue Value;
    static constexpr auto gp_fields = gp::gc::fields(gp::gc::field(&slogAttr::Key, "Key"),
                                                     gp::gc::field(&slogAttr::Value, "Value"));
};
GP_GC_LAYOUT(slogAttr, 40, 8);
static_assert(gp::gc::detail::is_comparable_v<slogAttr> == false, "slogAttr: Go comparability");
GP_GC_FIELD_AT(slogAttr, Key, 0);
GP_GC_FIELD_AT(slogAttr, Value, 16);

} // namespace gc_layout_test
