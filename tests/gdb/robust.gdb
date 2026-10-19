# Bad memory, boxed variables, Go expressions, skip rules and helper frames (golden: robust.golden).
gp break main.main
#filter ^NEVER
break robust.gp:@inc
break robust.gp:@locals
break robust.gp:@deferred
run
#filter ^NEVER
bt
#filter main\.
continue
gp locals
bt
#filter main\.
continue
# Boxed variables (count, err) show their Go types; a big [][]int prints a bounded text.
gp locals
print rates
python print(len(gdb.execute("print grid", to_string=True)) < 3000)
# Go expressions in gp print.
gp print acc.n
gp print *acc
gp print len(grid)
gp print grid[1][2]
gp whatis acc.n
# *p of an int and a float pointee (the program itself uses *p, so gdb could call operator*),
# and a boxed variable: gp print agrees with gp locals.
gp print *q
gp print *fl
gp print $gp_deref(q)
gp print count
gp whatis count
gp print r[1]
gp print len(r)
gp print fp[1]
gp print *fp
# A string with a negative length, and a damaged type descriptor (invalid UTF-8 in its name):
# the locals still print, one by one.
set var msg.gp_len_ = -1
print msg
gp locals
#filter ^(msg|anyv|count) 
set var msg.gp_len_ = 5
set $orig = anyv.gp_type_
set var fake.gp_elems._M_elems[0] = (long)&bad.gp_elems._M_elems[0]
set var anyv.gp_type_ = (const gp::type_desc*)&fake.gp_elems._M_elems[0]
print anyv
gp locals
#filter ^(msg|anyv|count) 
set var anyv.gp_type_ = $orig
print $gp_blocked()
print $gp_blocked(1, 2)
# The skip rules are on at load, once, and off on request.
python print(len([l for l in gdb.execute("info skip", to_string=True).split("\n") if l.endswith(" ^std::")]))
gp skip-runtime
gp skip-runtime
python print(len([l for l in gdb.execute("info skip", to_string=True).split("\n") if l.endswith(" ^std::")]))
gp skip-runtime off
python print(len([l for l in gdb.execute("info skip", to_string=True).split("\n") if l.endswith(" ^std::")]))
gp skip-runtime
python print(len([l for l in gdb.execute("info skip", to_string=True).split("\n") if l.endswith(" ^std::")]))
# The skip rule for the runtime's sources names its folder: no */rt/*/src glob that would match the user's own files.
python print(len([l for l in gdb.execute("info skip", to_string=True).split("\n") if "*/rt/*/src" in l]))
# step stays in the .gp file: no stop in the C++ library or the runtime.
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
step
#filter \.(h|hpp|cpp|tcc|inc):\d+
bt
#filter main\.
# A deferred call: no C++ helper frames in the backtrace.
continue
bt
#filter in (main\.|std::|gp::defer|gp::detail)|^#\d+ +(main\.|std::|gp::defer|gp::detail)
gp bt
#filter in (main\.|std::|gp::defer|gp::detail)|^#\d+ +(main\.|std::|gp::defer|gp::detail)
