//go:build !unix

package load

import "errors"

func mkfifo(path string) error { return errors.New("no named pipes here") }
