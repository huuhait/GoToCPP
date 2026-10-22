# gp catch panic where there is no gp_panic (golden: nodefer.golden): the throw is the stop, and
# the Go function the selected frame.
gp catch panic
run
#filter ^NEVER
python print(gdb.selected_frame().name())
bt
#filter main\.
