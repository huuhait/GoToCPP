#pragma once

struct Node { int id; };

inline int take(Node n, const char *s) { return n.id + s[0]; }
inline int takeP(Node *n) { return n->id; }
