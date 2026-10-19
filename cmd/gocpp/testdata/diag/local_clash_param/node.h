#pragma once
struct Node { int id; };
inline int take(Node n, const char *s) { return n.id + s[0]; }
inline int takeP(Node *n) { return n->id; }
inline int take2(int a, Node n) { return a; }
inline int over(Node n) { return 0; }
inline int over(double d, Node n) { return 0; }
