# Local closures (golden: closures.golden).
break closures.gp:@show
run
gp locals
#filter ^(add|greet|anon) 
print add
print anon
# The type of a closure is a closure only in a frame of a .gp file: in the runtime's frame the same
# unnamed type is not classified (a C++ program's unnamed structs are left to gdb).
python v = gdb.selected_frame().read_var("add")
python print(classify(unwrap(v.type)))
up
#filter ^NEVER
python print(classify(unwrap(v.type)))
# A variable that the local closure captures by reference: the value, formatted by gdb (/x too).
break closures.gp:@ref
continue
print k
print/x k
