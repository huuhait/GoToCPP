# A breakpoint of the user's on gp::panic_constant (number 1) and a catch throw (2), then gp catch
# panic (3 is its catchpoint, 4 its breakpoint at gp_panic; golden: userstops.golden). Each stop prints
# the selected frame and what stopped. direct's panic: the user's breakpoint keeps the runtime
# frame; at its throw both catch throws stop, and gp catch panic's selects the Go frame; the
# panic statement of unrecovered stops at gp_panic (Go frame); its second throw is stopped only
# by the user's catch throw (the condition of gp catch panic's is false there): the frame stays
# __cxa_throw.
break gp::panic_constant
#filter ^NEVER
catch throw
gp catch panic
#filter ^Catchpoint 3 \(throw\)
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
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
continue
#filter ^NEVER
