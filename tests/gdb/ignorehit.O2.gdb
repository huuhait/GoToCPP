# Optimized: the same stops, the same frames.
break gp::panic_constant
catch throw
gp catch panic
ignore 4 2
run
python print(gdb.selected_frame().name())
#want (?m)^gp::panic_constant$
continue
python print(gdb.selected_frame().name())
#want (?m)^gpkg_main::direct$
continue
python print(gdb.selected_frame().name())
#want (?m)^__cxa_throw$
