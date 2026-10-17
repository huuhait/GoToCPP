package load

import (
	"errors"
	"path/filepath"
)

// Where a module is when it is neither replaced by a local folder nor
// vendored (Phase 06 step 6.1b prepares what job 6.1c fills in).
//
// resolveRequire (modules.go) asks moduleDir for the folder of such a
// module. Job 6.1c (module downloads: the tagged archive, gocpp.sum, the
// read-only module cache) provides the real function behind this hook, by
// replacing the body of moduleDir below or by setting Config.ModuleDir:
//
//	func moduleDir(path, version string) (dir string, err error)
//
//   - path is the module path of the require line (or the new path of a
//     replace line that is not a local folder), version its version;
//   - dir is the absolute folder that holds the files of the module: its
//     packages and headers are in it, like in a replacement folder. It
//     needs no gocpp.mod file (a plain C or C++ repository has none), and
//     the loader never writes to it;
//   - err says why the module is not available: the loader reports it
//     with the module, the version and the import that needs it. A
//     function that can't download returns ErrNoDownload (or an error that
//     wraps it): the loader adds the hint to replace or vendor the module.
//
// The loader calls it once for each module (and version) per load.

// ErrNoDownload is the error of a module that is not in a local folder and
// that gocpp can't fetch (yet).
var ErrNoDownload = errors.New("gocpp can't download modules yet")

// ModuleDirFunc is the type of moduleDir and of Config.ModuleDir.
type ModuleDirFunc func(path, version string) (dir string, err error)

// moduleDir is the default of Config.ModuleDir. It is a stub until job
// 6.1c: no module is downloaded.
func moduleDir(path, version string) (dir string, err error) {
	return "", ErrNoDownload
}

// moduleDirFunc returns the function that finds the folder of a module that
// is neither replaced by a local folder nor vendored.
func (l *loader) moduleDirFunc() ModuleDirFunc {
	if l.cfg.ModuleDir != nil {
		return l.cfg.ModuleDir
	}
	return moduleDir
}

// downloadedModule returns the module path@version from moduleDir, or the
// reason why it is not available. hint is added to the message of
// ErrNoDownload.
func (l *loader) downloadedModule(path, version, hint string) (*Module, error) {
	dir, err := l.moduleDirFunc()(path, version)
	if err != nil {
		return nil, wrapNoDownload(err, hint)
	}
	if !filepath.IsAbs(dir) || !l.isDir(dir) {
		return nil, errors.New("the module folder " + dir + " is not an existing absolute folder")
	}
	return &Module{Path: path, Dir: filepath.Clean(dir), Version: version, cached: true}, nil
}

// noDownloadError is ErrNoDownload with the hint of what to do instead.
type noDownloadError struct{ hint string }

func (e *noDownloadError) Error() string { return ErrNoDownload.Error() + e.hint }
func (e *noDownloadError) Unwrap() error { return ErrNoDownload }

func wrapNoDownload(err error, hint string) error {
	if err == ErrNoDownload && hint != "" {
		return &noDownloadError{hint}
	}
	return err
}
