# gp catch panic (golden: panics.golden): a throw (direct), the panic statement of a function with
# defers (withDefer), runtime.Goexit in a goroutine with defers (goexit: one stop, at the .gp line),
# and a panic that nothing recovers (unrecovered): one stop at gp_panic, none at the throw that
# gp_run makes again. A second gp catch panic replaces the first one's stops (the
# numbers are 3 and 4), and the Go function is the selected frame at a stop.
gp catch panic
#filter ^Catchpoint 1 \(throw\)
gp catch panic
#filter ^Catchpoint 3 \(throw\)
run
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
bt
#filter main\.
continue
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
bt
#filter main\.
continue
#filter ^NEVER
python print([(b.number, b.hit_count) for b in gdb.breakpoints()])
bt
#filter main\.
continue
#filter ^NEVER
python print(gdb.selected_frame().name(), [(b.number, b.hit_count) for b in gdb.breakpoints()])
bt
#filter main\.
continue
#filter ^NEVER
python print([(b.number, b.hit_count) for b in gdb.breakpoints()])
