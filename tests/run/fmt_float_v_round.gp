// phase: 05

// Go's %v of float64 values (round) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", 2.675)
	fmt.Printf("[%+v]\n", 2.675)
	fmt.Printf("[%-v]\n", 2.675)
	fmt.Printf("[% v]\n", 2.675)
	fmt.Printf("[%0v]\n", 2.675)
	fmt.Printf("[%#v]\n", 2.675)
	fmt.Printf("[%10v]\n", 2.675)
	fmt.Printf("[%-10v]\n", 2.675)
	fmt.Printf("[%010v]\n", 2.675)
	fmt.Printf("[%+010v]\n", 2.675)
	fmt.Printf("[%10.3v]\n", 2.675)
	fmt.Printf("[%.0v]\n", 2.675)
	fmt.Printf("[%.1v]\n", 2.675)
	fmt.Printf("[%.3v]\n", 2.675)
	fmt.Printf("[%.10v]\n", 2.675)
	fmt.Printf("[%#.0v]\n", 2.675)
	fmt.Printf("[%#10.3v]\n", 2.675)
	fmt.Printf("[%+.2v]\n", 2.675)
	fmt.Printf("[% 10.2v]\n", 2.675)
	fmt.Printf("[%-#10.2v]\n", 2.675)
	fmt.Printf("[%20.15v]\n", 2.675)
	fmt.Printf("[%.20v]\n", 2.675)
	fmt.Printf("[%0-10v]\n", 2.675)
	fmt.Printf("[%v]\n", 1.005)
	fmt.Printf("[%+v]\n", 1.005)
	fmt.Printf("[%-v]\n", 1.005)
	fmt.Printf("[% v]\n", 1.005)
	fmt.Printf("[%0v]\n", 1.005)
	fmt.Printf("[%#v]\n", 1.005)
	fmt.Printf("[%10v]\n", 1.005)
	fmt.Printf("[%-10v]\n", 1.005)
	fmt.Printf("[%010v]\n", 1.005)
	fmt.Printf("[%+010v]\n", 1.005)
	fmt.Printf("[%10.3v]\n", 1.005)
	fmt.Printf("[%.0v]\n", 1.005)
	fmt.Printf("[%.1v]\n", 1.005)
	fmt.Printf("[%.3v]\n", 1.005)
	fmt.Printf("[%.10v]\n", 1.005)
	fmt.Printf("[%#.0v]\n", 1.005)
	fmt.Printf("[%#10.3v]\n", 1.005)
	fmt.Printf("[%+.2v]\n", 1.005)
	fmt.Printf("[% 10.2v]\n", 1.005)
	fmt.Printf("[%-#10.2v]\n", 1.005)
	fmt.Printf("[%20.15v]\n", 1.005)
	fmt.Printf("[%.20v]\n", 1.005)
	fmt.Printf("[%0-10v]\n", 1.005)
	fmt.Printf("[%v]\n", 0.125)
	fmt.Printf("[%+v]\n", 0.125)
	fmt.Printf("[%-v]\n", 0.125)
	fmt.Printf("[% v]\n", 0.125)
	fmt.Printf("[%0v]\n", 0.125)
	fmt.Printf("[%#v]\n", 0.125)
	fmt.Printf("[%10v]\n", 0.125)
	fmt.Printf("[%-10v]\n", 0.125)
	fmt.Printf("[%010v]\n", 0.125)
	fmt.Printf("[%+010v]\n", 0.125)
	fmt.Printf("[%10.3v]\n", 0.125)
	fmt.Printf("[%.0v]\n", 0.125)
	fmt.Printf("[%.1v]\n", 0.125)
	fmt.Printf("[%.3v]\n", 0.125)
	fmt.Printf("[%.10v]\n", 0.125)
	fmt.Printf("[%#.0v]\n", 0.125)
	fmt.Printf("[%#10.3v]\n", 0.125)
	fmt.Printf("[%+.2v]\n", 0.125)
	fmt.Printf("[% 10.2v]\n", 0.125)
	fmt.Printf("[%-#10.2v]\n", 0.125)
	fmt.Printf("[%20.15v]\n", 0.125)
	fmt.Printf("[%.20v]\n", 0.125)
	fmt.Printf("[%0-10v]\n", 0.125)
	fmt.Printf("[%v]\n", 0.375)
	fmt.Printf("[%+v]\n", 0.375)
	fmt.Printf("[%-v]\n", 0.375)
	fmt.Printf("[% v]\n", 0.375)
	fmt.Printf("[%0v]\n", 0.375)
	fmt.Printf("[%#v]\n", 0.375)
	fmt.Printf("[%10v]\n", 0.375)
	fmt.Printf("[%-10v]\n", 0.375)
	fmt.Printf("[%010v]\n", 0.375)
	fmt.Printf("[%+010v]\n", 0.375)
	fmt.Printf("[%10.3v]\n", 0.375)
	fmt.Printf("[%.0v]\n", 0.375)
	fmt.Printf("[%.1v]\n", 0.375)
	fmt.Printf("[%.3v]\n", 0.375)
	fmt.Printf("[%.10v]\n", 0.375)
	fmt.Printf("[%#.0v]\n", 0.375)
	fmt.Printf("[%#10.3v]\n", 0.375)
	fmt.Printf("[%+.2v]\n", 0.375)
	fmt.Printf("[% 10.2v]\n", 0.375)
	fmt.Printf("[%-#10.2v]\n", 0.375)
	fmt.Printf("[%20.15v]\n", 0.375)
	fmt.Printf("[%.20v]\n", 0.375)
	fmt.Printf("[%0-10v]\n", 0.375)
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
	fmt.Printf("[%v]\n", 3.5)
	fmt.Printf("[%+v]\n", 3.5)
	fmt.Printf("[%-v]\n", 3.5)
	fmt.Printf("[% v]\n", 3.5)
	fmt.Printf("[%0v]\n", 3.5)
	fmt.Printf("[%#v]\n", 3.5)
	fmt.Printf("[%10v]\n", 3.5)
	fmt.Printf("[%-10v]\n", 3.5)
	fmt.Printf("[%010v]\n", 3.5)
	fmt.Printf("[%+010v]\n", 3.5)
	fmt.Printf("[%10.3v]\n", 3.5)
	fmt.Printf("[%.0v]\n", 3.5)
	fmt.Printf("[%.1v]\n", 3.5)
	fmt.Printf("[%.3v]\n", 3.5)
	fmt.Printf("[%.10v]\n", 3.5)
	fmt.Printf("[%#.0v]\n", 3.5)
	fmt.Printf("[%#10.3v]\n", 3.5)
	fmt.Printf("[%+.2v]\n", 3.5)
	fmt.Printf("[% 10.2v]\n", 3.5)
	fmt.Printf("[%-#10.2v]\n", 3.5)
	fmt.Printf("[%20.15v]\n", 3.5)
	fmt.Printf("[%.20v]\n", 3.5)
	fmt.Printf("[%0-10v]\n", 3.5)
	fmt.Printf("[%v]\n", -2.5)
	fmt.Printf("[%+v]\n", -2.5)
	fmt.Printf("[%-v]\n", -2.5)
	fmt.Printf("[% v]\n", -2.5)
	fmt.Printf("[%0v]\n", -2.5)
	fmt.Printf("[%#v]\n", -2.5)
	fmt.Printf("[%10v]\n", -2.5)
	fmt.Printf("[%-10v]\n", -2.5)
	fmt.Printf("[%010v]\n", -2.5)
	fmt.Printf("[%+010v]\n", -2.5)
	fmt.Printf("[%10.3v]\n", -2.5)
	fmt.Printf("[%.0v]\n", -2.5)
	fmt.Printf("[%.1v]\n", -2.5)
	fmt.Printf("[%.3v]\n", -2.5)
	fmt.Printf("[%.10v]\n", -2.5)
	fmt.Printf("[%#.0v]\n", -2.5)
	fmt.Printf("[%#10.3v]\n", -2.5)
	fmt.Printf("[%+.2v]\n", -2.5)
	fmt.Printf("[% 10.2v]\n", -2.5)
	fmt.Printf("[%-#10.2v]\n", -2.5)
	fmt.Printf("[%20.15v]\n", -2.5)
	fmt.Printf("[%.20v]\n", -2.5)
	fmt.Printf("[%0-10v]\n", -2.5)
	fmt.Printf("[%v]\n", 0.045)
	fmt.Printf("[%+v]\n", 0.045)
	fmt.Printf("[%-v]\n", 0.045)
	fmt.Printf("[% v]\n", 0.045)
	fmt.Printf("[%0v]\n", 0.045)
	fmt.Printf("[%#v]\n", 0.045)
	fmt.Printf("[%10v]\n", 0.045)
	fmt.Printf("[%-10v]\n", 0.045)
	fmt.Printf("[%010v]\n", 0.045)
	fmt.Printf("[%+010v]\n", 0.045)
	fmt.Printf("[%10.3v]\n", 0.045)
	fmt.Printf("[%.0v]\n", 0.045)
	fmt.Printf("[%.1v]\n", 0.045)
	fmt.Printf("[%.3v]\n", 0.045)
	fmt.Printf("[%.10v]\n", 0.045)
	fmt.Printf("[%#.0v]\n", 0.045)
	fmt.Printf("[%#10.3v]\n", 0.045)
	fmt.Printf("[%+.2v]\n", 0.045)
	fmt.Printf("[% 10.2v]\n", 0.045)
	fmt.Printf("[%-#10.2v]\n", 0.045)
	fmt.Printf("[%20.15v]\n", 0.045)
	fmt.Printf("[%.20v]\n", 0.045)
	fmt.Printf("[%0-10v]\n", 0.045)
	fmt.Printf("[%v]\n", 1e22)
	fmt.Printf("[%+v]\n", 1e22)
	fmt.Printf("[%-v]\n", 1e22)
	fmt.Printf("[% v]\n", 1e22)
	fmt.Printf("[%0v]\n", 1e22)
	fmt.Printf("[%#v]\n", 1e22)
	fmt.Printf("[%10v]\n", 1e22)
	fmt.Printf("[%-10v]\n", 1e22)
	fmt.Printf("[%010v]\n", 1e22)
	fmt.Printf("[%+010v]\n", 1e22)
	fmt.Printf("[%10.3v]\n", 1e22)
	fmt.Printf("[%.0v]\n", 1e22)
	fmt.Printf("[%.1v]\n", 1e22)
	fmt.Printf("[%.3v]\n", 1e22)
	fmt.Printf("[%.10v]\n", 1e22)
	fmt.Printf("[%#.0v]\n", 1e22)
	fmt.Printf("[%#10.3v]\n", 1e22)
	fmt.Printf("[%+.2v]\n", 1e22)
	fmt.Printf("[% 10.2v]\n", 1e22)
	fmt.Printf("[%-#10.2v]\n", 1e22)
	fmt.Printf("[%20.15v]\n", 1e22)
	fmt.Printf("[%.20v]\n", 1e22)
	fmt.Printf("[%0-10v]\n", 1e22)
	fmt.Printf("[%v]\n", 999999.5)
	fmt.Printf("[%+v]\n", 999999.5)
	fmt.Printf("[%-v]\n", 999999.5)
	fmt.Printf("[% v]\n", 999999.5)
	fmt.Printf("[%0v]\n", 999999.5)
	fmt.Printf("[%#v]\n", 999999.5)
	fmt.Printf("[%10v]\n", 999999.5)
	fmt.Printf("[%-10v]\n", 999999.5)
	fmt.Printf("[%010v]\n", 999999.5)
	fmt.Printf("[%+010v]\n", 999999.5)
	fmt.Printf("[%10.3v]\n", 999999.5)
	fmt.Printf("[%.0v]\n", 999999.5)
	fmt.Printf("[%.1v]\n", 999999.5)
	fmt.Printf("[%.3v]\n", 999999.5)
	fmt.Printf("[%.10v]\n", 999999.5)
	fmt.Printf("[%#.0v]\n", 999999.5)
	fmt.Printf("[%#10.3v]\n", 999999.5)
	fmt.Printf("[%+.2v]\n", 999999.5)
	fmt.Printf("[% 10.2v]\n", 999999.5)
	fmt.Printf("[%-#10.2v]\n", 999999.5)
	fmt.Printf("[%20.15v]\n", 999999.5)
	fmt.Printf("[%.20v]\n", 999999.5)
	fmt.Printf("[%0-10v]\n", 999999.5)
	fmt.Printf("[%v]\n", 0.9999995)
	fmt.Printf("[%+v]\n", 0.9999995)
	fmt.Printf("[%-v]\n", 0.9999995)
	fmt.Printf("[% v]\n", 0.9999995)
	fmt.Printf("[%0v]\n", 0.9999995)
	fmt.Printf("[%#v]\n", 0.9999995)
	fmt.Printf("[%10v]\n", 0.9999995)
	fmt.Printf("[%-10v]\n", 0.9999995)
	fmt.Printf("[%010v]\n", 0.9999995)
	fmt.Printf("[%+010v]\n", 0.9999995)
	fmt.Printf("[%10.3v]\n", 0.9999995)
	fmt.Printf("[%.0v]\n", 0.9999995)
	fmt.Printf("[%.1v]\n", 0.9999995)
	fmt.Printf("[%.3v]\n", 0.9999995)
	fmt.Printf("[%.10v]\n", 0.9999995)
	fmt.Printf("[%#.0v]\n", 0.9999995)
	fmt.Printf("[%#10.3v]\n", 0.9999995)
	fmt.Printf("[%+.2v]\n", 0.9999995)
	fmt.Printf("[% 10.2v]\n", 0.9999995)
	fmt.Printf("[%-#10.2v]\n", 0.9999995)
	fmt.Printf("[%20.15v]\n", 0.9999995)
	fmt.Printf("[%.20v]\n", 0.9999995)
	fmt.Printf("[%0-10v]\n", 0.9999995)
}

// Output:
// [2.675]
// [2.675]
// [2.675]
// [ 2.675]
// [2.675]
// [2.675]
// [     2.675]
// [2.675     ]
// [000002.675]
// [000002.675]
// [      2.67]
// [3]
// [3]
// [2.67]
// [2.675]
// [3]
// [      2.67]
// [2.7]
// [       2.7]
// [2.7       ]
// [               2.675]
// [2.6749999999999998224]
// [2.675     ]
// [1.005]
// [1.005]
// [1.005]
// [ 1.005]
// [1.005]
// [1.005]
// [     1.005]
// [1.005     ]
// [000001.005]
// [000001.005]
// [         1]
// [1]
// [1]
// [1]
// [1.005]
// [1]
// [         1]
// [1]
// [         1]
// [1         ]
// [               1.005]
// [1.0049999999999998934]
// [1.005     ]
// [0.125]
// [0.125]
// [0.125]
// [ 0.125]
// [0.125]
// [0.125]
// [     0.125]
// [0.125     ]
// [000000.125]
// [000000.125]
// [     0.125]
// [0.1]
// [0.1]
// [0.125]
// [0.125]
// [0.1]
// [     0.125]
// [0.12]
// [      0.12]
// [0.12      ]
// [               0.125]
// [0.125]
// [0.125     ]
// [0.375]
// [0.375]
// [0.375]
// [ 0.375]
// [0.375]
// [0.375]
// [     0.375]
// [0.375     ]
// [000000.375]
// [000000.375]
// [     0.375]
// [0.4]
// [0.4]
// [0.375]
// [0.375]
// [0.4]
// [     0.375]
// [0.38]
// [      0.38]
// [0.38      ]
// [               0.375]
// [0.375]
// [0.375     ]
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
// [3.5]
// [3.5]
// [3.5]
// [ 3.5]
// [3.5]
// [3.5]
// [       3.5]
// [3.5       ]
// [00000003.5]
// [00000003.5]
// [       3.5]
// [4]
// [4]
// [3.5]
// [3.5]
// [4]
// [       3.5]
// [3.5]
// [       3.5]
// [3.5       ]
// [                 3.5]
// [3.5]
// [3.5       ]
// [-2.5]
// [-2.5]
// [-2.5]
// [-2.5]
// [-2.5]
// [-2.5]
// [      -2.5]
// [-2.5      ]
// [-0000002.5]
// [-0000002.5]
// [      -2.5]
// [-2]
// [-2]
// [-2.5]
// [-2.5]
// [-2]
// [      -2.5]
// [-2.5]
// [      -2.5]
// [-2.5      ]
// [                -2.5]
// [-2.5]
// [-2.5      ]
// [0.045]
// [0.045]
// [0.045]
// [ 0.045]
// [0.045]
// [0.045]
// [     0.045]
// [0.045     ]
// [000000.045]
// [000000.045]
// [     0.045]
// [0.04]
// [0.04]
// [0.045]
// [0.045]
// [0.04]
// [     0.045]
// [0.045]
// [     0.045]
// [0.045     ]
// [               0.045]
// [0.044999999999999998335]
// [0.045     ]
// [1e+22]
// [1e+22]
// [1e+22]
// [ 1e+22]
// [1e+22]
// [1e+22]
// [     1e+22]
// [1e+22     ]
// [000001e+22]
// [000001e+22]
// [     1e+22]
// [1e+22]
// [1e+22]
// [1e+22]
// [1e+22]
// [1e+22]
// [     1e+22]
// [1e+22]
// [     1e+22]
// [1e+22     ]
// [               1e+22]
// [1e+22]
// [1e+22     ]
// [999999.5]
// [999999.5]
// [999999.5]
// [ 999999.5]
// [999999.5]
// [999999.5]
// [  999999.5]
// [999999.5  ]
// [00999999.5]
// [00999999.5]
// [     1e+06]
// [1e+06]
// [1e+06]
// [1e+06]
// [999999.5]
// [1e+06]
// [     1e+06]
// [1e+06]
// [     1e+06]
// [1e+06     ]
// [            999999.5]
// [999999.5]
// [999999.5  ]
// [0.9999995]
// [0.9999995]
// [0.9999995]
// [ 0.9999995]
// [0.9999995]
// [0.9999995]
// [ 0.9999995]
// [0.9999995 ]
// [00.9999995]
// [00.9999995]
// [         1]
// [1]
// [1]
// [1]
// [0.9999995]
// [1]
// [         1]
// [1]
// [         1]
// [1         ]
// [           0.9999995]
// [0.99999950000000004113]
// [0.9999995 ]
