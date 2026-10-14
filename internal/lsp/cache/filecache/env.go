package filecache

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The environment variables of the cache of "gocpp lsp".
const (
	// EnvDir is the root folder of the cache (an absolute name), or "off"
	// for no cache. The default is DefaultDir.
	EnvDir = "GOCPP_LSP_CACHE"
	// EnvSize is the size cap (Options.MaxSize), in bytes or with a
	// suffix K, M or G (powers of 1024): "512M".
	EnvSize = "GOCPP_LSP_CACHE_SIZE"
	// EnvAge is the age cap (Options.MaxAge), as a Go duration: "72h".
	EnvAge = "GOCPP_LSP_CACHE_AGE"
)

// DefaultDir returns the default root folder of the cache: gocpp/lsp in the
// user's cache folder (os.UserCacheDir: $XDG_CACHE_HOME or ~/.cache on
// Linux, %LocalAppData% on Windows).
func DefaultDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gocpp", "lsp"), nil
}

// FromEnv returns the options of the cache that the environment asks for
// (EnvDir, EnvSize, EnvAge), and false when EnvDir is "off".
func FromEnv() (Options, bool, error) {
	var opts Options
	dir := os.Getenv(EnvDir)
	switch dir {
	case "off":
		return opts, false, nil
	case "":
		d, err := DefaultDir()
		if err != nil {
			return opts, false, fmt.Errorf("no folder for the cache: %v (set %s)", err, EnvDir)
		}
		dir = d
	default:
		// A relative folder would be relative to the folder that the
		// editor starts the server in, which changes from run to run.
		if !filepath.IsAbs(dir) {
			return opts, false, fmt.Errorf("%s must be an absolute folder, not %q", EnvDir, dir)
		}
	}
	opts.Dir = dir
	if s := os.Getenv(EnvSize); s != "" {
		n, err := ParseSize(s)
		if err != nil {
			return opts, false, fmt.Errorf("%s: %v", EnvSize, err)
		}
		opts.MaxSize = n
	}
	if s := os.Getenv(EnvAge); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return opts, false, fmt.Errorf("%s: bad duration %q", EnvAge, s)
		}
		opts.MaxAge = d
	}
	return opts, true, nil
}

// ParseSize parses a size in bytes, with an optional suffix K, M or G
// (powers of 1024, also written KiB, MiB, GiB or KB, MB, GB).
func ParseSize(s string) (int64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	t = strings.TrimSuffix(strings.TrimSuffix(t, "B"), "I")
	mult := int64(1)
	if t != "" {
		switch t[len(t)-1] {
		case 'K':
			mult = 1 << 10
		case 'M':
			mult = 1 << 20
		case 'G':
			mult = 1 << 30
		}
		if mult > 1 {
			t = t[:len(t)-1]
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
	if err != nil || n <= 0 || n > (1<<62)/mult {
		return 0, fmt.Errorf("bad size %q", s)
	}
	return n * mult, nil
}
