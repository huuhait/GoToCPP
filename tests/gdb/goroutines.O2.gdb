# Optimized: poll is inlined into main, so no line of it is a reliable stop; the runtime's select
# is (main is the only goroutine that gets there when six others are blocked). The Go frame
# of a goroutine may be missing, but never the list.
break gp::detail::select_run if $gp_blocked() == 6
run
#want (?m)^Thread N "goroutines" hit Breakpoint 1(\.\d+)?, at 
gp goroutines
#want (?m)^\*\s+1\s+running\s
#want (?m)^\s+2\s+waiting \[chan receive\]\s
#want (?m)^\s+3\s+waiting \[chan receive\]\s
#want (?m)^\s+4\s+waiting \[chan receive\]\s
#want (?m)^\s+5\s+waiting \[chan send\]\s
#want (?m)^\s+6\s+waiting \[select\]\s
#want (?m)^\s+7\s+waiting \[select \(no cases\)\]\s
