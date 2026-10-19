package cache

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// physicalMemory returns the memory of the machine in bytes, or 0 (the limit
// of a cgroup is not looked at: the default is a cap, not a reservation).
func physicalMemory() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "MemTotal:"); ok {
			fields := strings.Fields(rest)
			if len(fields) >= 1 {
				if kb, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
					return kb << 10
				}
			}
		}
	}
	return 0
}
