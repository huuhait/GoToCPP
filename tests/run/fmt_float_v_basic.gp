// phase: 05

// Go's %v of float64 values (basic) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", 0.0)
	fmt.Printf("[%+v]\n", 0.0)
	fmt.Printf("[%-v]\n", 0.0)
	fmt.Printf("[% v]\n", 0.0)
	fmt.Printf("[%0v]\n", 0.0)
	fmt.Printf("[%#v]\n", 0.0)
	fmt.Printf("[%10v]\n", 0.0)
	fmt.Printf("[%-10v]\n", 0.0)
	fmt.Printf("[%010v]\n", 0.0)
	fmt.Printf("[%+010v]\n", 0.0)
	fmt.Printf("[%10.3v]\n", 0.0)
	fmt.Printf("[%.0v]\n", 0.0)
	fmt.Printf("[%.1v]\n", 0.0)
	fmt.Printf("[%.3v]\n", 0.0)
	fmt.Printf("[%.10v]\n", 0.0)
	fmt.Printf("[%#.0v]\n", 0.0)
	fmt.Printf("[%#10.3v]\n", 0.0)
	fmt.Printf("[%+.2v]\n", 0.0)
	fmt.Printf("[% 10.2v]\n", 0.0)
	fmt.Printf("[%-#10.2v]\n", 0.0)
	fmt.Printf("[%20.15v]\n", 0.0)
	fmt.Printf("[%.20v]\n", 0.0)
	fmt.Printf("[%0-10v]\n", 0.0)
	fmt.Printf("[%v]\n", 1.0)
	fmt.Printf("[%+v]\n", 1.0)
	fmt.Printf("[%-v]\n", 1.0)
	fmt.Printf("[% v]\n", 1.0)
	fmt.Printf("[%0v]\n", 1.0)
	fmt.Printf("[%#v]\n", 1.0)
	fmt.Printf("[%10v]\n", 1.0)
	fmt.Printf("[%-10v]\n", 1.0)
	fmt.Printf("[%010v]\n", 1.0)
	fmt.Printf("[%+010v]\n", 1.0)
	fmt.Printf("[%10.3v]\n", 1.0)
	fmt.Printf("[%.0v]\n", 1.0)
	fmt.Printf("[%.1v]\n", 1.0)
	fmt.Printf("[%.3v]\n", 1.0)
	fmt.Printf("[%.10v]\n", 1.0)
	fmt.Printf("[%#.0v]\n", 1.0)
	fmt.Printf("[%#10.3v]\n", 1.0)
	fmt.Printf("[%+.2v]\n", 1.0)
	fmt.Printf("[% 10.2v]\n", 1.0)
	fmt.Printf("[%-#10.2v]\n", 1.0)
	fmt.Printf("[%20.15v]\n", 1.0)
	fmt.Printf("[%.20v]\n", 1.0)
	fmt.Printf("[%0-10v]\n", 1.0)
	fmt.Printf("[%v]\n", -1.0)
	fmt.Printf("[%+v]\n", -1.0)
	fmt.Printf("[%-v]\n", -1.0)
	fmt.Printf("[% v]\n", -1.0)
	fmt.Printf("[%0v]\n", -1.0)
	fmt.Printf("[%#v]\n", -1.0)
	fmt.Printf("[%10v]\n", -1.0)
	fmt.Printf("[%-10v]\n", -1.0)
	fmt.Printf("[%010v]\n", -1.0)
	fmt.Printf("[%+010v]\n", -1.0)
	fmt.Printf("[%10.3v]\n", -1.0)
	fmt.Printf("[%.0v]\n", -1.0)
	fmt.Printf("[%.1v]\n", -1.0)
	fmt.Printf("[%.3v]\n", -1.0)
	fmt.Printf("[%.10v]\n", -1.0)
	fmt.Printf("[%#.0v]\n", -1.0)
	fmt.Printf("[%#10.3v]\n", -1.0)
	fmt.Printf("[%+.2v]\n", -1.0)
	fmt.Printf("[% 10.2v]\n", -1.0)
	fmt.Printf("[%-#10.2v]\n", -1.0)
	fmt.Printf("[%20.15v]\n", -1.0)
	fmt.Printf("[%.20v]\n", -1.0)
	fmt.Printf("[%0-10v]\n", -1.0)
	fmt.Printf("[%v]\n", 3.14159265358979)
	fmt.Printf("[%+v]\n", 3.14159265358979)
	fmt.Printf("[%-v]\n", 3.14159265358979)
	fmt.Printf("[% v]\n", 3.14159265358979)
	fmt.Printf("[%0v]\n", 3.14159265358979)
	fmt.Printf("[%#v]\n", 3.14159265358979)
	fmt.Printf("[%10v]\n", 3.14159265358979)
	fmt.Printf("[%-10v]\n", 3.14159265358979)
	fmt.Printf("[%010v]\n", 3.14159265358979)
	fmt.Printf("[%+010v]\n", 3.14159265358979)
	fmt.Printf("[%10.3v]\n", 3.14159265358979)
	fmt.Printf("[%.0v]\n", 3.14159265358979)
	fmt.Printf("[%.1v]\n", 3.14159265358979)
	fmt.Printf("[%.3v]\n", 3.14159265358979)
	fmt.Printf("[%.10v]\n", 3.14159265358979)
	fmt.Printf("[%#.0v]\n", 3.14159265358979)
	fmt.Printf("[%#10.3v]\n", 3.14159265358979)
	fmt.Printf("[%+.2v]\n", 3.14159265358979)
	fmt.Printf("[% 10.2v]\n", 3.14159265358979)
	fmt.Printf("[%-#10.2v]\n", 3.14159265358979)
	fmt.Printf("[%20.15v]\n", 3.14159265358979)
	fmt.Printf("[%.20v]\n", 3.14159265358979)
	fmt.Printf("[%0-10v]\n", 3.14159265358979)
	fmt.Printf("[%v]\n", 2.5)
	fmt.Printf("[%+v]\n", 2.5)
	fmt.Printf("[%-v]\n", 2.5)
	fmt.Printf("[% v]\n", 2.5)
	fmt.Printf("[%0v]\n", 2.5)
	fmt.Printf("[%#v]\n", 2.5)
	fmt.Printf("[%10v]\n", 2.5)
	fmt.Printf("[%-10v]\n", 2.5)
	fmt.Printf("[%010v]\n", 2.5)
	fmt.Printf("[%+010v]\n", 2.5)
	fmt.Printf("[%10.3v]\n", 2.5)
	fmt.Printf("[%.0v]\n", 2.5)
	fmt.Printf("[%.1v]\n", 2.5)
	fmt.Printf("[%.3v]\n", 2.5)
	fmt.Printf("[%.10v]\n", 2.5)
	fmt.Printf("[%#.0v]\n", 2.5)
	fmt.Printf("[%#10.3v]\n", 2.5)
	fmt.Printf("[%+.2v]\n", 2.5)
	fmt.Printf("[% 10.2v]\n", 2.5)
	fmt.Printf("[%-#10.2v]\n", 2.5)
	fmt.Printf("[%20.15v]\n", 2.5)
	fmt.Printf("[%.20v]\n", 2.5)
	fmt.Printf("[%0-10v]\n", 2.5)
	fmt.Printf("[%v]\n", 100.0)
	fmt.Printf("[%+v]\n", 100.0)
	fmt.Printf("[%-v]\n", 100.0)
	fmt.Printf("[% v]\n", 100.0)
	fmt.Printf("[%0v]\n", 100.0)
	fmt.Printf("[%#v]\n", 100.0)
	fmt.Printf("[%10v]\n", 100.0)
	fmt.Printf("[%-10v]\n", 100.0)
	fmt.Printf("[%010v]\n", 100.0)
	fmt.Printf("[%+010v]\n", 100.0)
	fmt.Printf("[%10.3v]\n", 100.0)
	fmt.Printf("[%.0v]\n", 100.0)
	fmt.Printf("[%.1v]\n", 100.0)
	fmt.Printf("[%.3v]\n", 100.0)
	fmt.Printf("[%.10v]\n", 100.0)
	fmt.Printf("[%#.0v]\n", 100.0)
	fmt.Printf("[%#10.3v]\n", 100.0)
	fmt.Printf("[%+.2v]\n", 100.0)
	fmt.Printf("[% 10.2v]\n", 100.0)
	fmt.Printf("[%-#10.2v]\n", 100.0)
	fmt.Printf("[%20.15v]\n", 100.0)
	fmt.Printf("[%.20v]\n", 100.0)
	fmt.Printf("[%0-10v]\n", 100.0)
	fmt.Printf("[%v]\n", 0.5)
	fmt.Printf("[%+v]\n", 0.5)
	fmt.Printf("[%-v]\n", 0.5)
	fmt.Printf("[% v]\n", 0.5)
	fmt.Printf("[%0v]\n", 0.5)
	fmt.Printf("[%#v]\n", 0.5)
	fmt.Printf("[%10v]\n", 0.5)
	fmt.Printf("[%-10v]\n", 0.5)
	fmt.Printf("[%010v]\n", 0.5)
	fmt.Printf("[%+010v]\n", 0.5)
	fmt.Printf("[%10.3v]\n", 0.5)
	fmt.Printf("[%.0v]\n", 0.5)
	fmt.Printf("[%.1v]\n", 0.5)
	fmt.Printf("[%.3v]\n", 0.5)
	fmt.Printf("[%.10v]\n", 0.5)
	fmt.Printf("[%#.0v]\n", 0.5)
	fmt.Printf("[%#10.3v]\n", 0.5)
	fmt.Printf("[%+.2v]\n", 0.5)
	fmt.Printf("[% 10.2v]\n", 0.5)
	fmt.Printf("[%-#10.2v]\n", 0.5)
	fmt.Printf("[%20.15v]\n", 0.5)
	fmt.Printf("[%.20v]\n", 0.5)
	fmt.Printf("[%0-10v]\n", 0.5)
	fmt.Printf("[%v]\n", 123456.789)
	fmt.Printf("[%+v]\n", 123456.789)
	fmt.Printf("[%-v]\n", 123456.789)
	fmt.Printf("[% v]\n", 123456.789)
	fmt.Printf("[%0v]\n", 123456.789)
	fmt.Printf("[%#v]\n", 123456.789)
	fmt.Printf("[%10v]\n", 123456.789)
	fmt.Printf("[%-10v]\n", 123456.789)
	fmt.Printf("[%010v]\n", 123456.789)
	fmt.Printf("[%+010v]\n", 123456.789)
	fmt.Printf("[%10.3v]\n", 123456.789)
	fmt.Printf("[%.0v]\n", 123456.789)
	fmt.Printf("[%.1v]\n", 123456.789)
	fmt.Printf("[%.3v]\n", 123456.789)
	fmt.Printf("[%.10v]\n", 123456.789)
	fmt.Printf("[%#.0v]\n", 123456.789)
	fmt.Printf("[%#10.3v]\n", 123456.789)
	fmt.Printf("[%+.2v]\n", 123456.789)
	fmt.Printf("[% 10.2v]\n", 123456.789)
	fmt.Printf("[%-#10.2v]\n", 123456.789)
	fmt.Printf("[%20.15v]\n", 123456.789)
	fmt.Printf("[%.20v]\n", 123456.789)
	fmt.Printf("[%0-10v]\n", 123456.789)
}

// Output:
// [0]
// [0]
// [0]
// [ 0]
// [0]
// [0]
// [         0]
// [0         ]
// [0000000000]
// [0000000000]
// [         0]
// [0]
// [0]
// [0]
// [0]
// [0]
// [         0]
// [0]
// [         0]
// [0         ]
// [                   0]
// [0]
// [0         ]
// [1]
// [1]
// [1]
// [ 1]
// [1]
// [1]
// [         1]
// [1         ]
// [0000000001]
// [0000000001]
// [         1]
// [1]
// [1]
// [1]
// [1]
// [1]
// [         1]
// [1]
// [         1]
// [1         ]
// [                   1]
// [1]
// [1         ]
// [-1]
// [-1]
// [-1]
// [-1]
// [-1]
// [-1]
// [        -1]
// [-1        ]
// [-000000001]
// [-000000001]
// [        -1]
// [-1]
// [-1]
// [-1]
// [-1]
// [-1]
// [        -1]
// [-1]
// [        -1]
// [-1        ]
// [                  -1]
// [-1]
// [-1        ]
// [3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [ 3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [3.14159265358979]
// [      3.14]
// [3]
// [3]
// [3.14]
// [3.141592654]
// [3]
// [      3.14]
// [3.1]
// [       3.1]
// [3.1       ]
// [    3.14159265358979]
// [3.1415926535897900074]
// [3.14159265358979]
// [2.5]
// [2.5]
// [2.5]
// [ 2.5]
// [2.5]
// [2.5]
// [       2.5]
// [2.5       ]
// [00000002.5]
// [00000002.5]
// [       2.5]
// [2]
// [2]
// [2.5]
// [2.5]
// [2]
// [       2.5]
// [2.5]
// [       2.5]
// [2.5       ]
// [                 2.5]
// [2.5]
// [2.5       ]
// [100]
// [100]
// [100]
// [ 100]
// [100]
// [100]
// [       100]
// [100       ]
// [0000000100]
// [0000000100]
// [       100]
// [1e+02]
// [1e+02]
// [100]
// [100]
// [1e+02]
// [       100]
// [1e+02]
// [     1e+02]
// [1e+02     ]
// [                 100]
// [100]
// [100       ]
// [0.5]
// [0.5]
// [0.5]
// [ 0.5]
// [0.5]
// [0.5]
// [       0.5]
// [0.5       ]
// [00000000.5]
// [00000000.5]
// [       0.5]
// [0.5]
// [0.5]
// [0.5]
// [0.5]
// [0.5]
// [       0.5]
// [0.5]
// [       0.5]
// [0.5       ]
// [                 0.5]
// [0.5]
// [0.5       ]
// [123456.789]
// [123456.789]
// [123456.789]
// [ 123456.789]
// [123456.789]
// [123456.789]
// [123456.789]
// [123456.789]
// [123456.789]
// [123456.789]
// [  1.23e+05]
// [1e+05]
// [1e+05]
// [1.23e+05]
// [123456.789]
// [1e+05]
// [  1.23e+05]
// [1.2e+05]
// [   1.2e+05]
// [1.2e+05   ]
// [          123456.789]
// [123456.78900000000431]
// [123456.789]
