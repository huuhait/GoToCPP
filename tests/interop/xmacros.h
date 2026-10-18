// A helper header for macro_collisions.gp: macros that collide with names that gocpp writes.
#pragma once

#define Bool int              /* like X11's: collides with the gocpp type Bool */
#define Println oops          /* collides with the runtime's gp::fmt::Println */
#define LIMIT 10              /* used through the header package: xmacros.LIMIT */
#define TWICE(x) ((x) * 2)    /* a function-like macro */
#define AS_BOOL(x) ((Bool)(x)) /* a macro that needs Bool: so Bool is not #undef'd */
