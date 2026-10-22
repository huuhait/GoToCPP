# Boxed variables: the closure's n and p are heap boxes in the C++, gp locals and gp print show
# the Go variables (golden: boxed.golden). In counter the locals were moved into the closure.
break boxed.gp:@make
break boxed.gp:@inc
run
gp locals
#filter ^f 
continue
gp locals
#filter ^n 
gp print n
gp whatis n
gp print p.v
