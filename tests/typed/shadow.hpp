// Types for member_shadow.gp and its C++ twin: classes with an operator-> whose own member has the
// name of a member of the pointee. The extracted member lists miss using-declarations and
// enumerators, so only the compiler can say that the class has the member.
#pragma once
#include <vector>

struct Leaf {
    int x = 1;
    int get() const { return 100; }
    template <typename T>
    int tm(T) const { return 0; }
    int resize(int n) const { return n; }
    int size() const { return 0; }
};

struct BX {
    int x = 99;
    int get(int v) const { return 1000 + v; }
};

struct PrivBase : private BX {
    using BX::x;
    using BX::get;
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct PubBase : BX {
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct AnonU {
    union {
        int x;
        float f;
    };
    Leaf l;
    Leaf* operator->() { return &l; }
    AnonU() { x = 77; }
};
struct AnonS {
    struct {
        int x;
        int z;
    };
    Leaf l;
    Leaf* operator->() { return &l; }
    AnonS() {
        x = 66;
        z = 1;
    }
};
struct StaticX {
    static inline int x = 55;
    Leaf l;
    Leaf* operator->() { return &l; }
};
template <typename T>
struct TBase {
    T x = 44;
};
struct FromTBase : TBase<int> {
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct BitX {
    int x : 4;
    Leaf l;
    Leaf* operator->() { return &l; }
    BitX() : x(3) {}
};
struct MethX {
    int x() const { return 33; }
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct EnumX {
    enum { x = 22 };
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct TemplX {
    template <typename T>
    int tm(T) const { return 12; }
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct MultiBase : Leaf {
    Leaf l2;
    Leaf* operator->() { return &l2; }
};

// No x of its own: x is the pointee's.
struct Plain {
    Leaf l;
    Leaf* operator->() { return &l; }
};
// A chain: Chain to Wrap to Leaf.
struct Wrap {
    Leaf l;
    Leaf* operator->() { return &l; }
};
struct Chain {
    Wrap w;
    Wrap& operator->() { return w; }
};

// A private base that the extraction does not follow (a class of the library), with its members
// brought in by using-declarations: PrivStd has resize and size, though its member list does not.
struct PrivStd : private std::vector<int> {
    using std::vector<int>::resize;
    using std::vector<int>::size;
    Leaf l;
    Leaf* operator->() { return &l; }
};
