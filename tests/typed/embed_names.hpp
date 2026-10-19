// Types for embed_names.gp and its C++ twin: members named like Go keywords and built-in functions.
#pragma once
struct Words {
    int type() const { return 1; }
    int range(int a, int b) const { return b - a; }
    int select(int k) { return k * 2; }
    int func = 3;
    int var = 4;
    int go = 5;
    int map = 6;
    int len() const { return 7; }
    int append(int v) { return v + 8; }
    int interface = 9;
    int defer = 10;
    int chan = 11;
    int fallthrough = 12;
};
struct More {
    int package = 13;
    int import = 14;
    int cap() const { return 15; }
    int make() const { return 16; }
};
