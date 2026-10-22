# Optimized: the stop is in Bump, an inline frame of main with g++ and clang++ (gdb's #0 and #1), and
# the Go name of the inlined method is still main.(*Counter).Bump.
break inline.gp:@bump
run
#want (?m)^Breakpoint 1(\.\d+)?, at inline\.gp:15$
bt
#want (?m)^#0 +main\.\(\*Counter\)\.Bump \(.*\) at inline\.gp:15$
#want (?m)^#1 +(0xADDR in )?main\.main \(\) at inline\.gp:23$
