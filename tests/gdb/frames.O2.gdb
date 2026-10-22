# Optimized: the stop in the loop of main still names main.main and the .gp line, and the runtime's
# frame below it is there. (An inlined method keeps its name: inline.O2.gdb. T.Get and T.Inc of a
# local pointer that never escapes can't be the stop here: clang++ folds them into main, and g++
# has no line of its own for them at -O2.)
break frames.gp:@loop
run
#want (?m)^Breakpoint 1, at frames\.gp:56$
bt
#want (?m)^#0 +main\.main \(.*\) at frames\.gp:56$
#want (?m)^#\d+ +0xADDR in gp::run_main \(
