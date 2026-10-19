# Channels (golden: chans.golden).
break chans.gp:@chans
run
gp locals
print ch
print unbuf
print closed
print nilCh
print $gp_len(chs)
print $gp_index(chs, 0)
print chs
print m
print a
