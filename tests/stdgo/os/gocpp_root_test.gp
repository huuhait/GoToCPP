package os

import (
	"errors"
	"io/fs"
)

// Root stands in for os.Root, which gocpp's os does not have (ADR-067): the tests of other
// functions name *os.Root in the signatures of their helpers, and pass nil (testMaybeRooted runs
// only the case without a Root). OpenRoot fails, so the few tests that open a Root fail.
type Root struct{}

func OpenRoot(name string) (*Root, error) { return nil, errors.ErrUnsupported }

func (r *Root) Close() error                               { return errors.ErrUnsupported }
func (r *Root) Create(name string) (*File, error)          { return nil, errors.ErrUnsupported }
func (r *Root) Open(name string) (*File, error)            { return nil, errors.ErrUnsupported }
func (r *Root) OpenFile(name string, flag int, perm FileMode) (*File, error) {
	return nil, errors.ErrUnsupported
}
func (r *Root) Link(oldname, newname string) error         { return errors.ErrUnsupported }
func (r *Root) Symlink(oldname, newname string) error      { return errors.ErrUnsupported }
func (r *Root) Stat(name string) (FileInfo, error)         { return nil, errors.ErrUnsupported }
func (r *Root) Lstat(name string) (FileInfo, error)        { return nil, errors.ErrUnsupported }
func (r *Root) Readlink(name string) (string, error)       { return "", errors.ErrUnsupported }
func (r *Root) FS() fs.FS                                  { return nil }
func (r *Root) Remove(name string) error                   { return errors.ErrUnsupported }
func (r *Root) MkdirAll(name string, perm FileMode) error  { return errors.ErrUnsupported }
