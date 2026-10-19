# The goroutine list (golden: goroutines.golden). The breakpoint waits until six goroutines are
# blocked, so the list never depends on timing.
break goroutines.gp:@poll if $gp_blocked() == 6
run
gp goroutines
info goroutines
thread 4
#filter Switching
gp goroutines
bt
#filter main\.
gp bt
#filter main\.
# The goroutine's frames end at its Go function: no std::apply, call_task::run, run_goroutine,
# worker_main, thread_main, start_thread or clone3 below it (the golden section is empty).
bt
#filter std::(apply|__invoke|__apply)|call_task|run_goroutine|worker_main|thread_main|start_thread|clone
