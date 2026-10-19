#pragma once
template <class T> struct Node { T v; };
struct Item { int id; };
inline int takeI(Item i) { return i.id; }
namespace ns { struct Point { int x; }; inline int takeP(Point p) { return p.x; } }
inline int Point(int a) { return a; }
