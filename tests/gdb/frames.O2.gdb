# Optimized: T.Get and T.Inc are inlined into main, so main.main is the frame that stops.
break frames.gp:@get
run
#want (?m)^Breakpoint 1, at frames\.gp:12$
bt
#want (?m)^#0 +main\.(main|T\.Get) \(.*\) at frames\.gp:12$
#want (?m)^#\d+ +0xADDR in gp::run_main \(
