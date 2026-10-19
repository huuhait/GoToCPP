# Generic instances: their names in backtraces, and the values they handle (golden: generics.golden).
break generics.gp:@max
break generics.gp:@map
break generics.gp:@show
run
bt
#filter main\.
gp locals
continue
bt
#filter main\.
gp locals
continue
bt
#filter main\.
gp locals
continue
gp locals
continue
bt
#filter main\.
gp locals
continue
bt
#filter main\.
gp locals
continue
