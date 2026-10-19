# The values of values.gp as the printers show them (golden: values.golden).
break values.gp:@zero
break values.gp:@show
run
gp locals
continue
gp locals
print ints
print $gp_len(ints)
print $gp_index(ints, 1)
print m1
print $gp_len(m1)
print hello
print pp
print $gp_deref(pp)
print sh
print err
print clo
print out
gp print out
gp whatis list
gp whatis m4
gp whatis sh2
info locals
#filter ^(ints|m1|hello|out|pp|anyPt) =
bt
#filter main\.
gp bt
#filter main\.
set gocpp print-style tree
print pts
print m4
set gocpp print-style go
