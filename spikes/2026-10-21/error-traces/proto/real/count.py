import gdb
gdb.execute("set pagination off")
gdb.execute("set confirm off")
gdb.execute("break gpkg_main::app")
gdb.execute("run 4 %s" % MODE)
# skip first hit (lazy binding noise), count the second call
gdb.execute("continue")
entry_sp = int(gdb.parse_and_eval("$sp"))
n = 0
while True:
    pc = int(gdb.parse_and_eval("$pc"))
    insn = gdb.selected_inferior().read_memory(pc, 1)[0]
    gdb.execute("stepi", to_string=True)
    n += 1
    if int(gdb.parse_and_eval("$sp")) > entry_sp and n > 5:
        break
print("INSTRUCTIONS", n)
