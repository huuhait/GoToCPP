// The C++ twin of adapt_voidout.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    Obj* obj = nullptr;
    void* raw = nullptr;
    make_object(&raw);
    obj = static_cast<Obj*>(raw);
    std::printf("%s %d\n", obj != nullptr ? "true" : "false", obj->v);

    Obj* field = nullptr;
    make_object(&raw);
    field = static_cast<Obj*>(raw);
    std::printf("%d\n", field->v);

    make_nothing(&raw);
    obj = static_cast<Obj*>(raw);
    std::printf("%s\n", obj == nullptr ? "true" : "false");

    make_object(&raw);
    field = static_cast<Obj*>(raw);
    std::printf("%d %d %d\n", mutable_value(field), object_value(field),
                object_value(static_cast<void*>(field)));
    std::printf("%d\n", object_value(nullptr));
}
