// A helper header for tag_names.gp: C tag names (struct point, enum mode).
#pragma once

struct point { int x, y; };
enum mode { MODE_A, MODE_B };
inline struct point make_point(int x, int y) { return point{x, y}; }
inline int point_sum(struct point p) { return p.x + p.y; }
