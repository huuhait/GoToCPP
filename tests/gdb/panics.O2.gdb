# Optimized: the starts of a panic are still found, once each (the throw of direct, the panic
# statement of withDefer, the Goexit of a goroutine with defers, and the one of unrecovered, whose
# second throw is not reported).
gp catch panic
run
python print(gdb.selected_frame().name())
#want (?m)^gpkg_main::direct$
bt
#want (?m)^#\d+ +0xADDR in main\.direct \(\) at panics\.gp:17$
continue
python print(gdb.selected_frame().name())
#want (?m)^gpkg_main::withDefer$
continue
bt
#want (?m)^#\d+ +0xADDR in main\.goexit\.func1 \(\) at panics\.gp:36$
continue
python print(gdb.selected_frame().name())
#want (?m)^gpkg_main::unrecovered$
continue
python print([b.hit_count for b in gdb.breakpoints()])
#want (?m)^\[2, 2\]$
