package diff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanStale removes the entries of dir whose modification time is older than maxAge and whose
// name starts with one of prefixes (none: every entry), and returns how many it removed. A
// campaign produces a folder of generated C++ and an up-to-date stamp for every build of gocpp,
// and a temporary folder for every build, which nothing needs after a few minutes: the janitor
// keeps a million programs from filling the disk, while the object cache, which is useful across
// bundles (the standard library's objects), is bounded by GOCPP_OBJCACHE_SIZE.
func CleanStale(dir string, maxAge time.Duration, now time.Time, prefixes ...string) int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if len(prefixes) > 0 {
			ok := false
			for _, p := range prefixes {
				ok = ok || strings.HasPrefix(e.Name(), p)
			}
			if !ok {
				continue
			}
		}
		fi, err := e.Info()
		if err != nil || now.Sub(fi.ModTime()) < maxAge {
			continue
		}
		if os.RemoveAll(filepath.Join(dir, e.Name())) == nil {
			n++
		}
	}
	return n
}

// Sweep cleans what a campaign leaves behind: folders of builds that were killed (tmp/go-*,
// tmp/cpp-*), and the generated C++ and stamps of old builds in the cache of gocpp.
func (c *Campaign) Sweep(now time.Time) int {
	n := 0
	if c.cfg.TmpDir != "" {
		n += CleanStale(c.cfg.TmpDir, 30*time.Minute, now, "go-", "cpp-")
	}
	if c.cfg.CacheDir != "" {
		n += CleanStale(filepath.Join(c.cfg.CacheDir, "gen"), 45*time.Minute, now)
		n += CleanStale(filepath.Join(c.cfg.CacheDir, "stamp"), 45*time.Minute, now)
		n += CleanStale(filepath.Join(c.cfg.CacheDir, "compdb"), 45*time.Minute, now) // -release writes one per build
	}
	return n
}

// janitor sweeps every few minutes until ctx ends.
func (c *Campaign) janitor(ctx context.Context) {
	c.Sweep(time.Now())
	t := time.NewTicker(c.sweepEvery())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c.Sweep(now)
		}
	}
}

func (c *Campaign) sweepEvery() time.Duration {
	if c.cfg.SweepEvery > 0 {
		return c.cfg.SweepEvery
	}
	return 3 * time.Minute
}
