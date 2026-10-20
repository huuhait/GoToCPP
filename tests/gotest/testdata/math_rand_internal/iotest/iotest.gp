// Package iotest stands in for testing/iotest (which gocpp does not have): the readers that Go's tests of the ported packages use.
package iotest

import "io"

type oneByteReader struct{ r io.Reader }

// OneByteReader returns a Reader that implements each non-empty Read by reading one byte from r.
func OneByteReader(r io.Reader) io.Reader { return &oneByteReader{r} }

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return r.r.Read(p[0:1])
}
