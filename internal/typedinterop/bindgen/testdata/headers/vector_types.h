typedef int v4 __attribute__((vector_size(16)));
v4 addv(v4 a, v4 b);
typedef double v3d __attribute__((ext_vector_type(3)));
typedef float v3f __attribute__((ext_vector_type(3)));
typedef _BitInt(65) b65;
typedef _BitInt(8) b8;
typedef double v4d_aligned __attribute__((vector_size(32)));
v3d scale3(v3d a, v3f b, b65 c, b8 d, v4d_aligned e);
