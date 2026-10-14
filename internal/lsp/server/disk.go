package server

import (
	"log"

	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
)

// openDisk opens the disk cache of the summaries (step 11.3b) that the
// environment asks for (filecache.FromEnv: GOCPP_LSP_CACHE, by default
// gocpp/lsp in the user's cache folder), or returns nil: the cache is off,
// or it can't be opened (then the server works without it, and logs why:
// also a relative GOCPP_LSP_CACHE). Its version is the hash of the gocpp
// binary; if the binary can't be hashed, the cache logs it once and misses.
func openDisk(logger *log.Logger) *filecache.Cache {
	opts, ok, err := filecache.FromEnv()
	if err == nil && ok {
		opts.Logf = func(format string, args ...any) { logger.Printf("cache of summaries: "+format, args...) }
		var c *filecache.Cache
		if c, err = filecache.Open(opts); err == nil {
			logger.Printf("the cache of summaries is in %s", c.Dir())
			return c
		}
	}
	if err != nil {
		logger.Printf("no cache of summaries on disk: %v", err)
	}
	return nil
}
