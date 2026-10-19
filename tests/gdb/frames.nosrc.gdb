# The sources are not found (the debug info says /trim/frames.gp, gdb has no substitute-path):
# the frames keep the names of the debug info (the receiver of Inc is not known to be a pointer,
# a closure of g++ is operator()<>) and nothing fails.
break frames.gp:@inc
break frames.gp:@inner
run
#want (?m)^Breakpoint 1, at frames\.gp:16$
bt
#want (?m)^#0 +main\.T\.Inc \(
#want (?m)^#1 +0xADDR in main\.main \(\) at frames\.gp:50$
continue
#want (?m)^Breakpoint 2, at frames\.gp:23$
bt
#want (?m)^#0 +(operator\(\)<>\(void\)|main\.outer\.[\w.]+) \(
