// A C header of the corpus: structs, a typedef, functions, a macro and an
// enum, as a C library has them.
#ifndef CORPUS_GEOM_H
#define CORPUS_GEOM_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct point {
    double x, y;
} point;

struct rect {
    point min, max;
};

typedef int handle;

enum color { RED, GREEN, BLUE };

#define GEOM_MAX_POINTS 64

point make_point(double x, double y);
double point_dist(point a, point b);
struct rect* new_rect(point min, point max);
void free_rect(struct rect* r);
handle open_handle(const char* name);
int handle_count(void);

#ifdef __cplusplus
}
#endif

#endif
