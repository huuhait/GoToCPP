# gp catch panic with an ignore count on its gp_panic breakpoint (golden: ignorehit.golden): a user
# breakpoint on gp::panic_constant (number 1), a catch throw (2), gp catch panic (3 and 4, the
# breakpoint at gp_panic), and ignore 4 2: the two panic statements of withDefer and unrecovered
# are crossed without a stop. The first stop is the user's breakpoint in the runtime (the hit that
# was ignored left no mark: the runtime frame stays); then the throw of direct (gp catch panic's:
# the Go frame); then the second throw of unrecovered, where only the user's catch throw stops (the
# frame stays __cxa_throw, whatever the ignored hit before it left).
break gp::panic_constant
#filter ^NEVER
catch throw
#filter ^NEVER
gp catch panic
#filter ^Catchpoint 3 \(throw\)
ignore 4 2
run
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
continue
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
continue
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
continue
#filter ^NEVER
