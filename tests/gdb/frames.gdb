# Go names in backtraces (golden: frames.golden).
break frames.gp:@global
break frames.gp:@inc
break frames.gp:@get
break frames.gp:@inner
break frames.gp:@defer
break frames.gp:@fact
run
bt
#filter main\.
continue
bt
#filter main\.
print t
continue
bt
#filter main\.
continue
bt
#filter main\.
gp bt
#filter main\.
continue
bt
#filter main\.|gp::defer_frame::gp_run
continue
bt
#filter main\.
gp locals
set gocpp names cpp
bt
#filter ^#\d+ +(0xADDR in )?gpkg_main::
set gocpp names go
frame 1
gp locals
