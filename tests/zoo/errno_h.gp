// phase: 04

// <errno.h>: errno is a macro, so it is written unqualified in C++; reading and assigning it,
// the E* constants, and strerror from <string.h>.
package main

import (
	"fmt"

	"<errno.h>"
	cstring "<string.h>"
	"<stdlib.h>"
)

func main() {
	errno.errno = 0
	f := stdlib.strtol("99999999999999999999999", nil, 10)
	fmt.Println(int64(f) > 0, errno.errno == errno.ERANGE)
	errno.errno = errno.ENOENT
	fmt.Println(C.GoString(cstring.strerror(errno.errno)), int(errno.errno))
}

// Output:
// true true
// No such file or directory 2
