// The C++ twin of member_shadow.gp: the expected output is what this program prints.
#include <cstdio>
#include "../shadow.hpp"

int main() {
    PrivBase a;
    PubBase pub;
    AnonU b;
    AnonS c;
    StaticX d;
    FromTBase e;
    BitX f;
    MethX g;
    EnumX i;
    MultiBase j;
    TemplX k;
    j.l2.x = 500;
    std::printf("%d %d %d %d %d %d %d %d %d %d\n", (int)a.x, (int)pub.x, (int)b.x, (int)c.x, (int)d.x, (int)e.x, (int)f.x,
                (int)g.x(), (int)i.x, (int)j.x);
    a.x = 5;
    pub.x = 6;
    b.x = 7;
    c.x = 8;
    e.x = 9;
    f.x = 2;
    j.x = 4;
    std::printf("%d %d %d %d %d %d %d %d %d\n", (int)a.x, (int)pub.x, (int)b.x, (int)c.x, (int)e.x, (int)f.x, (int)j.x,
                (int)a.l.x, (int)j.l2.x);
    Plain p;
    p->x = 31;
    // k.tm is TemplX's, a.tm(2) is the pointee's (PrivBase has none): through operator->; a.get is BX's.
    std::printf("%d %d %d %d\n", k.tm(1), a->tm(2), a.get(3), a.get(p->x));
    PrivStd ps;
    ps.resize(p->x);
    std::printf("%d\n", (int)ps.size());
    Chain ch;
    ch->x = 32;
    std::printf("%d %d %d %d %d\n", (int)p->x, (int)p.l.x, p->get(), (int)ch->x, ch->get());
}
