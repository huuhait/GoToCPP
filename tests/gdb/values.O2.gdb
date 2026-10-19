# What still works in an optimized build: no value may break the printers.
break values.gp:@show
run
#want (?m)^Breakpoint 1, at values\.gp:129$
gp locals
#want (?m)^ints \(\[\]int\) = (\[1 2 3\] \(len 3, cap 3\)|<optimized out>)$
#want (?m)^m1 \(map\[string\]int\) = (map\["a":1 "b":2\] \(len 2\)|<optimized out>)$
bt
#want (?m)^#\d+ +(0xADDR in )?main\.show \(
#want (?m)^#\d+ +0xADDR in main\.main \(\) at values\.gp:134$
