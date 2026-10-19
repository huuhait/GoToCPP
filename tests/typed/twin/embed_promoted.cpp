// The C++ twin of embed_promoted.gp: the expected output is what this program prints.
#include <cstdio>
#include <vector>
#include "../embed_promoted.hpp"

struct Widget {
    Colored Colored_;
    std::string label;
};
struct Panel {
    Widget* Widget_;
    Tag Tag_;
    int id;
};
struct Outer {
    Panel Panel_;
    int n;
};
struct Stacked {
    Stack Stack_;
    std::vector<int> vector_;
};

int main() {
    Widget w{};
    w.label = "w";
    w.Colored_.grow(2);
    w.Colored_.rgb = 9;
    w.Colored_.w++;
    std::printf("%d %d %d %s\n", w.Colored_.area(), w.Colored_.color(), w.Colored_.w, w.label.c_str());

    Widget inner{};
    Panel p{&inner, {}, 1};
    p.Widget_->Colored_.grow(1);
    p.Tag_.set("hello");
    p.Widget_->Colored_.h += 5;
    std::printf("%d %d %s %d\n", p.Widget_->Colored_.area(), p.Tag_.length(), p.Tag_.text.c_str(), p.id);

    Outer o{p, 3};
    o.Panel_.Widget_->Colored_.grow(1);
    o.Panel_.Tag_.set("deep");
    std::printf("%d %d %d %d %d\n", o.Panel_.Widget_->Colored_.area(), o.Panel_.Tag_.length(), o.Panel_.Widget_->Colored_.rgb, o.n, p.Widget_->Colored_.area());

    Stacked s{};
    s.Stack_.push(4);
    s.Stack_.push(5);
    s.vector_.push_back(6);
    s.vector_.push_back(7);
    int t = s.Stack_.top();
    s.Stack_.top() = 8;
    int* q = &s.Stack_.top();
    *q += 1;
    int top = s.Stack_.top();
    int calls = s.Stack_.top_calls;
    std::printf("%d %d %d %d %d\n", t, top, calls, (int)s.vector_.size(), (int)s.Stack_.items.size());
    int b1 = Stack::bump();
    int b2 = Stack::bump();
    std::printf("%d %d %d %d %d\n", (int)Stack::Fast, (int)Stack::Slow, b1, b2, Stack::made);
    int last = s.Stack_.top();
    std::printf("%d %g %d %d %d\n", s.Stack_.twice(21), s.Stack_.twice(1.5), s.vector_.at(1), last, (int)s.vector_.size());
}
