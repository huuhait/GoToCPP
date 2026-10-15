// phase: 04
//gocpp:extern-c ./no_such_header.h // ERROR "//gocpp:extern-c names header \./no_such_header\.h, which no file of the package imports"
//gocpp:extern-c <stdio.h>
//gocpp:no-extern-c <stdio.h> // ERROR "header <stdio\.h> has both //gocpp:extern-c and //gocpp:no-extern-c \(the other is at directive_externc\.gp:3\)"
//gocpp:no-extern-c <cstdio> // ERROR "//gocpp:no-extern-c names header <cstdio>, which no file of the package imports"
//gocpp:extern-c <stdlib.h>

// The extern-c and no-extern-c directives (Phase 04 §2.9): a header that no
// file imports is an error, and so are both directives on one header. A
// repeated directive is fine.
//gocpp:extern-c <stdlib.h>
package main

import (
	"<stdio.h>"
	"<stdlib.h>"
)

func main() {}
