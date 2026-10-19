// phase: 09

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
)

type writer interface{ Write([]byte) (int, error) }
type readWriteSeeker interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Seek(int64, int) (int64, error)
}

func main() {
	d, _ := os.MkdirTemp("", "gpos")
	defer os.RemoveAll(d)
	f, _ := os.Create(d + "/f")
	var w writer = f
	w.Write([]byte("abc"))
	var rws readWriteSeeker = f
	rws.Seek(0, 0)
	b := make([]byte, 3)
	rws.Read(b)
	fmt.Println(string(b))
	var rw io.ReadWriteSeeker = f
	var wc io.WriteCloser = f
	var rat io.ReaderAt = f
	var wat io.WriterAt = f
	var fsf fs.File = f
	var rdf fs.ReadDirFile = f
	fmt.Println(rw != nil, wc != nil, rat != nil, wat != nil, fsf != nil, rdf != nil)
	st, err := fsf.Stat()
	fmt.Println(st.Size(), err)
	var fi fs.FileInfo = st
	var fm fs.FileMode = fi.Mode()
	var om os.FileMode = fm
	fmt.Println(om.IsRegular(), os.FileMode(0o644))
	var de fs.DirEntry
	ents, _ := os.ReadDir(d)
	de = ents[0]
	fmt.Println(de.Name(), de.Type())
	f.Close()
}

// Output:
// abc
// true true true true true true
// 3 <nil>
// true -rw-r--r--
// f ----------
