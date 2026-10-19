// Types for the err_embed_*.gp programs.
#pragma once
#include <vector>

struct Locked {
    int open_field = 1;
    void gone() = delete;
    int visible() const { return 1; }
private:
    int secret = 2;
protected:
    int guarded = 3;
};
struct Hidden : private Locked {};
struct Twin { int size() const { return 1; } };
