package cache

import "testing"

func TestCheckBytesFor(t *testing.T) {
	for _, c := range []struct{ mem, want int64 }{
		{0, unknownMemoryCheckBytes},
		{256 << 20, minDefaultCheckBytes},
		{1 << 30, 102 << 20}, // 1 GiB / 10, in bytes (rounded down by the division)
		{2 << 30, 204 << 20},
		{3 << 30, maxDefaultCheckBytes},
		{64 << 30, maxDefaultCheckBytes},
	} {
		got := checkBytesFor(c.mem)
		if c.mem == 1<<30 && got != (1<<30)/10 || c.mem == 2<<30 && got != (2<<30)/10 {
			t.Errorf("checkBytesFor(%d) = %d", c.mem, got)
			continue
		}
		if c.mem != 1<<30 && c.mem != 2<<30 && got != c.want {
			t.Errorf("checkBytesFor(%d) = %d, want %d", c.mem, got, c.want)
		}
	}
	if d := DefaultMaxCheckBytes(); d < minDefaultCheckBytes || d > maxDefaultCheckBytes {
		t.Errorf("the default %d is outside [%d, %d]", d, minDefaultCheckBytes, maxDefaultCheckBytes)
	}
}
