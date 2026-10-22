# Optimized: the same.
gp catch panic
run
python print(gdb.selected_frame().name())
#want (?m)^gpkg_main::boom$
