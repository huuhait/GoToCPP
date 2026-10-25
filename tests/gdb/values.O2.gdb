# What still works in an optimized build: no value may break the printers.
break values.gp:@show
run
# (An optimized line may have two locations, the inlined call and the function itself: g++ -O2 has
# the call of a descriptor's desc_get as a row of its own: the stop is "Breakpoint 1.1" or "1.2".)
#want (?m)^Breakpoint 1(\.[12])?, at values\.gp:129$
gp locals
#want (?m)^ints \(\[\]int\) = (\[1 2 3\] \(len 3, cap 3\)|<optimized out>)$
#want (?m)^m1 \(map\[string\]int\) = (map\["a":1 "b":2\] \(len 2\)|<optimized out>)$
bt
#want (?m)^#\d+ +(0xADDR in )?main\.show \(
#want (?m)^#\d+ +0xADDR in main\.main \(\) at values\.gp:134$
