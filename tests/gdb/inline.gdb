# The method at -O0 (golden: inline.golden).
break inline.gp:@bump
run
bt
#filter main\.
