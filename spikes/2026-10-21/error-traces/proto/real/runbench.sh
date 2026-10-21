# Timing of the four-call chain of bench.gp (mode 0 nil error, 1 fresh error, 2 shared sentinel, 3 fresh
# error through two errors.Wrapf levels); min and median of 7 runs pinned to one CPU. The binaries
# v/bin_<compiler>_{base,hop} are made by compiling the variants that insert_hops.py writes (from the C++
# that `gocpp emit` makes of bench.gp) together with hop_rt.cpp. Usage: bash runbench.sh [g++|clang++]
cd $(dirname $0)/v
cxx=${1:-g++}
for mode in 0 1 2 3; do it=50000000; [ $mode != 0 ] && it=2000000; [ $mode = 3 ] && it=500000
  for round in 1 2 3 4 5 6 7; do for v in base hop; do echo -n "mode$mode $v "; taskset -c 45 ./bin_${cxx}_$v $it $mode; done; done
done | awk '{k=$1" "$2; a[k]=a[k]" "$3} END{for(k in a){n=split(a[k],x," "); asort(x); printf "%s min %.2f median %.2f\n", k, x[1], x[int((n+1)/2)]}}' | sort
