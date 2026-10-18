// phase: 05

// Go's %G of float64 values (round) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%G]\n", 2.675)
	fmt.Printf("[%+G]\n", 2.675)
	fmt.Printf("[%-G]\n", 2.675)
	fmt.Printf("[% G]\n", 2.675)
	fmt.Printf("[%0G]\n", 2.675)
	fmt.Printf("[%#G]\n", 2.675)
	fmt.Printf("[%10G]\n", 2.675)
	fmt.Printf("[%-10G]\n", 2.675)
	fmt.Printf("[%010G]\n", 2.675)
	fmt.Printf("[%+010G]\n", 2.675)
	fmt.Printf("[%10.3G]\n", 2.675)
	fmt.Printf("[%.0G]\n", 2.675)
	fmt.Printf("[%.1G]\n", 2.675)
	fmt.Printf("[%.3G]\n", 2.675)
	fmt.Printf("[%.10G]\n", 2.675)
	fmt.Printf("[%#.0G]\n", 2.675)
	fmt.Printf("[%#10.3G]\n", 2.675)
	fmt.Printf("[%+.2G]\n", 2.675)
	fmt.Printf("[% 10.2G]\n", 2.675)
	fmt.Printf("[%-#10.2G]\n", 2.675)
	fmt.Printf("[%20.15G]\n", 2.675)
	fmt.Printf("[%.20G]\n", 2.675)
	fmt.Printf("[%0-10G]\n", 2.675)
	fmt.Printf("[%G]\n", 1.005)
	fmt.Printf("[%+G]\n", 1.005)
	fmt.Printf("[%-G]\n", 1.005)
	fmt.Printf("[% G]\n", 1.005)
	fmt.Printf("[%0G]\n", 1.005)
	fmt.Printf("[%#G]\n", 1.005)
	fmt.Printf("[%10G]\n", 1.005)
	fmt.Printf("[%-10G]\n", 1.005)
	fmt.Printf("[%010G]\n", 1.005)
	fmt.Printf("[%+010G]\n", 1.005)
	fmt.Printf("[%10.3G]\n", 1.005)
	fmt.Printf("[%.0G]\n", 1.005)
	fmt.Printf("[%.1G]\n", 1.005)
	fmt.Printf("[%.3G]\n", 1.005)
	fmt.Printf("[%.10G]\n", 1.005)
	fmt.Printf("[%#.0G]\n", 1.005)
	fmt.Printf("[%#10.3G]\n", 1.005)
	fmt.Printf("[%+.2G]\n", 1.005)
	fmt.Printf("[% 10.2G]\n", 1.005)
	fmt.Printf("[%-#10.2G]\n", 1.005)
	fmt.Printf("[%20.15G]\n", 1.005)
	fmt.Printf("[%.20G]\n", 1.005)
	fmt.Printf("[%0-10G]\n", 1.005)
	fmt.Printf("[%G]\n", 0.125)
	fmt.Printf("[%+G]\n", 0.125)
	fmt.Printf("[%-G]\n", 0.125)
	fmt.Printf("[% G]\n", 0.125)
	fmt.Printf("[%0G]\n", 0.125)
	fmt.Printf("[%#G]\n", 0.125)
	fmt.Printf("[%10G]\n", 0.125)
	fmt.Printf("[%-10G]\n", 0.125)
	fmt.Printf("[%010G]\n", 0.125)
	fmt.Printf("[%+010G]\n", 0.125)
	fmt.Printf("[%10.3G]\n", 0.125)
	fmt.Printf("[%.0G]\n", 0.125)
	fmt.Printf("[%.1G]\n", 0.125)
	fmt.Printf("[%.3G]\n", 0.125)
	fmt.Printf("[%.10G]\n", 0.125)
	fmt.Printf("[%#.0G]\n", 0.125)
	fmt.Printf("[%#10.3G]\n", 0.125)
	fmt.Printf("[%+.2G]\n", 0.125)
	fmt.Printf("[% 10.2G]\n", 0.125)
	fmt.Printf("[%-#10.2G]\n", 0.125)
	fmt.Printf("[%20.15G]\n", 0.125)
	fmt.Printf("[%.20G]\n", 0.125)
	fmt.Printf("[%0-10G]\n", 0.125)
	fmt.Printf("[%G]\n", 0.375)
	fmt.Printf("[%+G]\n", 0.375)
	fmt.Printf("[%-G]\n", 0.375)
	fmt.Printf("[% G]\n", 0.375)
	fmt.Printf("[%0G]\n", 0.375)
	fmt.Printf("[%#G]\n", 0.375)
	fmt.Printf("[%10G]\n", 0.375)
	fmt.Printf("[%-10G]\n", 0.375)
	fmt.Printf("[%010G]\n", 0.375)
	fmt.Printf("[%+010G]\n", 0.375)
	fmt.Printf("[%10.3G]\n", 0.375)
	fmt.Printf("[%.0G]\n", 0.375)
	fmt.Printf("[%.1G]\n", 0.375)
	fmt.Printf("[%.3G]\n", 0.375)
	fmt.Printf("[%.10G]\n", 0.375)
	fmt.Printf("[%#.0G]\n", 0.375)
	fmt.Printf("[%#10.3G]\n", 0.375)
	fmt.Printf("[%+.2G]\n", 0.375)
	fmt.Printf("[% 10.2G]\n", 0.375)
	fmt.Printf("[%-#10.2G]\n", 0.375)
	fmt.Printf("[%20.15G]\n", 0.375)
	fmt.Printf("[%.20G]\n", 0.375)
	fmt.Printf("[%0-10G]\n", 0.375)
	fmt.Printf("[%G]\n", 2.5)
	fmt.Printf("[%+G]\n", 2.5)
	fmt.Printf("[%-G]\n", 2.5)
	fmt.Printf("[% G]\n", 2.5)
	fmt.Printf("[%0G]\n", 2.5)
	fmt.Printf("[%#G]\n", 2.5)
	fmt.Printf("[%10G]\n", 2.5)
	fmt.Printf("[%-10G]\n", 2.5)
	fmt.Printf("[%010G]\n", 2.5)
	fmt.Printf("[%+010G]\n", 2.5)
	fmt.Printf("[%10.3G]\n", 2.5)
	fmt.Printf("[%.0G]\n", 2.5)
	fmt.Printf("[%.1G]\n", 2.5)
	fmt.Printf("[%.3G]\n", 2.5)
	fmt.Printf("[%.10G]\n", 2.5)
	fmt.Printf("[%#.0G]\n", 2.5)
	fmt.Printf("[%#10.3G]\n", 2.5)
	fmt.Printf("[%+.2G]\n", 2.5)
	fmt.Printf("[% 10.2G]\n", 2.5)
	fmt.Printf("[%-#10.2G]\n", 2.5)
	fmt.Printf("[%20.15G]\n", 2.5)
	fmt.Printf("[%.20G]\n", 2.5)
	fmt.Printf("[%0-10G]\n", 2.5)
	fmt.Printf("[%G]\n", 3.5)
	fmt.Printf("[%+G]\n", 3.5)
	fmt.Printf("[%-G]\n", 3.5)
	fmt.Printf("[% G]\n", 3.5)
	fmt.Printf("[%0G]\n", 3.5)
	fmt.Printf("[%#G]\n", 3.5)
	fmt.Printf("[%10G]\n", 3.5)
	fmt.Printf("[%-10G]\n", 3.5)
	fmt.Printf("[%010G]\n", 3.5)
	fmt.Printf("[%+010G]\n", 3.5)
	fmt.Printf("[%10.3G]\n", 3.5)
	fmt.Printf("[%.0G]\n", 3.5)
	fmt.Printf("[%.1G]\n", 3.5)
	fmt.Printf("[%.3G]\n", 3.5)
	fmt.Printf("[%.10G]\n", 3.5)
	fmt.Printf("[%#.0G]\n", 3.5)
	fmt.Printf("[%#10.3G]\n", 3.5)
	fmt.Printf("[%+.2G]\n", 3.5)
	fmt.Printf("[% 10.2G]\n", 3.5)
	fmt.Printf("[%-#10.2G]\n", 3.5)
	fmt.Printf("[%20.15G]\n", 3.5)
	fmt.Printf("[%.20G]\n", 3.5)
	fmt.Printf("[%0-10G]\n", 3.5)
	fmt.Printf("[%G]\n", -2.5)
	fmt.Printf("[%+G]\n", -2.5)
	fmt.Printf("[%-G]\n", -2.5)
	fmt.Printf("[% G]\n", -2.5)
	fmt.Printf("[%0G]\n", -2.5)
	fmt.Printf("[%#G]\n", -2.5)
	fmt.Printf("[%10G]\n", -2.5)
	fmt.Printf("[%-10G]\n", -2.5)
	fmt.Printf("[%010G]\n", -2.5)
	fmt.Printf("[%+010G]\n", -2.5)
	fmt.Printf("[%10.3G]\n", -2.5)
	fmt.Printf("[%.0G]\n", -2.5)
	fmt.Printf("[%.1G]\n", -2.5)
	fmt.Printf("[%.3G]\n", -2.5)
	fmt.Printf("[%.10G]\n", -2.5)
	fmt.Printf("[%#.0G]\n", -2.5)
	fmt.Printf("[%#10.3G]\n", -2.5)
	fmt.Printf("[%+.2G]\n", -2.5)
	fmt.Printf("[% 10.2G]\n", -2.5)
	fmt.Printf("[%-#10.2G]\n", -2.5)
	fmt.Printf("[%20.15G]\n", -2.5)
	fmt.Printf("[%.20G]\n", -2.5)
	fmt.Printf("[%0-10G]\n", -2.5)
	fmt.Printf("[%G]\n", 0.045)
	fmt.Printf("[%+G]\n", 0.045)
	fmt.Printf("[%-G]\n", 0.045)
	fmt.Printf("[% G]\n", 0.045)
	fmt.Printf("[%0G]\n", 0.045)
	fmt.Printf("[%#G]\n", 0.045)
	fmt.Printf("[%10G]\n", 0.045)
	fmt.Printf("[%-10G]\n", 0.045)
	fmt.Printf("[%010G]\n", 0.045)
	fmt.Printf("[%+010G]\n", 0.045)
	fmt.Printf("[%10.3G]\n", 0.045)
	fmt.Printf("[%.0G]\n", 0.045)
	fmt.Printf("[%.1G]\n", 0.045)
	fmt.Printf("[%.3G]\n", 0.045)
	fmt.Printf("[%.10G]\n", 0.045)
	fmt.Printf("[%#.0G]\n", 0.045)
	fmt.Printf("[%#10.3G]\n", 0.045)
	fmt.Printf("[%+.2G]\n", 0.045)
	fmt.Printf("[% 10.2G]\n", 0.045)
	fmt.Printf("[%-#10.2G]\n", 0.045)
	fmt.Printf("[%20.15G]\n", 0.045)
	fmt.Printf("[%.20G]\n", 0.045)
	fmt.Printf("[%0-10G]\n", 0.045)
	fmt.Printf("[%G]\n", 1e22)
	fmt.Printf("[%+G]\n", 1e22)
	fmt.Printf("[%-G]\n", 1e22)
	fmt.Printf("[% G]\n", 1e22)
	fmt.Printf("[%0G]\n", 1e22)
	fmt.Printf("[%#G]\n", 1e22)
	fmt.Printf("[%10G]\n", 1e22)
	fmt.Printf("[%-10G]\n", 1e22)
	fmt.Printf("[%010G]\n", 1e22)
	fmt.Printf("[%+010G]\n", 1e22)
	fmt.Printf("[%10.3G]\n", 1e22)
	fmt.Printf("[%.0G]\n", 1e22)
	fmt.Printf("[%.1G]\n", 1e22)
	fmt.Printf("[%.3G]\n", 1e22)
	fmt.Printf("[%.10G]\n", 1e22)
	fmt.Printf("[%#.0G]\n", 1e22)
	fmt.Printf("[%#10.3G]\n", 1e22)
	fmt.Printf("[%+.2G]\n", 1e22)
	fmt.Printf("[% 10.2G]\n", 1e22)
	fmt.Printf("[%-#10.2G]\n", 1e22)
	fmt.Printf("[%20.15G]\n", 1e22)
	fmt.Printf("[%.20G]\n", 1e22)
	fmt.Printf("[%0-10G]\n", 1e22)
	fmt.Printf("[%G]\n", 999999.5)
	fmt.Printf("[%+G]\n", 999999.5)
	fmt.Printf("[%-G]\n", 999999.5)
	fmt.Printf("[% G]\n", 999999.5)
	fmt.Printf("[%0G]\n", 999999.5)
	fmt.Printf("[%#G]\n", 999999.5)
	fmt.Printf("[%10G]\n", 999999.5)
	fmt.Printf("[%-10G]\n", 999999.5)
	fmt.Printf("[%010G]\n", 999999.5)
	fmt.Printf("[%+010G]\n", 999999.5)
	fmt.Printf("[%10.3G]\n", 999999.5)
	fmt.Printf("[%.0G]\n", 999999.5)
	fmt.Printf("[%.1G]\n", 999999.5)
	fmt.Printf("[%.3G]\n", 999999.5)
	fmt.Printf("[%.10G]\n", 999999.5)
	fmt.Printf("[%#.0G]\n", 999999.5)
	fmt.Printf("[%#10.3G]\n", 999999.5)
	fmt.Printf("[%+.2G]\n", 999999.5)
	fmt.Printf("[% 10.2G]\n", 999999.5)
	fmt.Printf("[%-#10.2G]\n", 999999.5)
	fmt.Printf("[%20.15G]\n", 999999.5)
	fmt.Printf("[%.20G]\n", 999999.5)
	fmt.Printf("[%0-10G]\n", 999999.5)
	fmt.Printf("[%G]\n", 0.9999995)
	fmt.Printf("[%+G]\n", 0.9999995)
	fmt.Printf("[%-G]\n", 0.9999995)
	fmt.Printf("[% G]\n", 0.9999995)
	fmt.Printf("[%0G]\n", 0.9999995)
	fmt.Printf("[%#G]\n", 0.9999995)
	fmt.Printf("[%10G]\n", 0.9999995)
	fmt.Printf("[%-10G]\n", 0.9999995)
	fmt.Printf("[%010G]\n", 0.9999995)
	fmt.Printf("[%+010G]\n", 0.9999995)
	fmt.Printf("[%10.3G]\n", 0.9999995)
	fmt.Printf("[%.0G]\n", 0.9999995)
	fmt.Printf("[%.1G]\n", 0.9999995)
	fmt.Printf("[%.3G]\n", 0.9999995)
	fmt.Printf("[%.10G]\n", 0.9999995)
	fmt.Printf("[%#.0G]\n", 0.9999995)
	fmt.Printf("[%#10.3G]\n", 0.9999995)
	fmt.Printf("[%+.2G]\n", 0.9999995)
	fmt.Printf("[% 10.2G]\n", 0.9999995)
	fmt.Printf("[%-#10.2G]\n", 0.9999995)
	fmt.Printf("[%20.15G]\n", 0.9999995)
	fmt.Printf("[%.20G]\n", 0.9999995)
	fmt.Printf("[%0-10G]\n", 0.9999995)
}

// Output:
// [2.675]
// [+2.675]
// [2.675]
// [ 2.675]
// [2.675]
// [2.67500]
// [     2.675]
// [2.675     ]
// [000002.675]
// [+00002.675]
// [      2.67]
// [3]
// [3]
// [2.67]
// [2.675]
// [3.]
// [      2.67]
// [+2.7]
// [       2.7]
// [2.7       ]
// [               2.675]
// [2.6749999999999998224]
// [2.675     ]
// [1.005]
// [+1.005]
// [1.005]
// [ 1.005]
// [1.005]
// [1.00500]
// [     1.005]
// [1.005     ]
// [000001.005]
// [+00001.005]
// [         1]
// [1]
// [1]
// [1]
// [1.005]
// [1.]
// [      1.00]
// [+1]
// [         1]
// [1.0       ]
// [               1.005]
// [1.0049999999999998934]
// [1.005     ]
// [0.125]
// [+0.125]
// [0.125]
// [ 0.125]
// [0.125]
// [0.125000]
// [     0.125]
// [0.125     ]
// [000000.125]
// [+00000.125]
// [     0.125]
// [0.1]
// [0.1]
// [0.125]
// [0.125]
// [0.1]
// [     0.125]
// [+0.12]
// [      0.12]
// [0.12      ]
// [               0.125]
// [0.125]
// [0.125     ]
// [0.375]
// [+0.375]
// [0.375]
// [ 0.375]
// [0.375]
// [0.375000]
// [     0.375]
// [0.375     ]
// [000000.375]
// [+00000.375]
// [     0.375]
// [0.4]
// [0.4]
// [0.375]
// [0.375]
// [0.4]
// [     0.375]
// [+0.38]
// [      0.38]
// [0.38      ]
// [               0.375]
// [0.375]
// [0.375     ]
// [2.5]
// [+2.5]
// [2.5]
// [ 2.5]
// [2.5]
// [2.50000]
// [       2.5]
// [2.5       ]
// [00000002.5]
// [+0000002.5]
// [       2.5]
// [2]
// [2]
// [2.5]
// [2.5]
// [2.]
// [      2.50]
// [+2.5]
// [       2.5]
// [2.5       ]
// [                 2.5]
// [2.5]
// [2.5       ]
// [3.5]
// [+3.5]
// [3.5]
// [ 3.5]
// [3.5]
// [3.50000]
// [       3.5]
// [3.5       ]
// [00000003.5]
// [+0000003.5]
// [       3.5]
// [4]
// [4]
// [3.5]
// [3.5]
// [4.]
// [      3.50]
// [+3.5]
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
// [-2.50000]
// [      -2.5]
// [-2.5      ]
// [-0000002.5]
// [-0000002.5]
// [      -2.5]
// [-2]
// [-2]
// [-2.5]
// [-2.5]
// [-2.]
// [     -2.50]
// [-2.5]
// [      -2.5]
// [-2.5      ]
// [                -2.5]
// [-2.5]
// [-2.5      ]
// [0.045]
// [+0.045]
// [0.045]
// [ 0.045]
// [0.045]
// [0.0450000]
// [     0.045]
// [0.045     ]
// [000000.045]
// [+00000.045]
// [     0.045]
// [0.04]
// [0.04]
// [0.045]
// [0.045]
// [0.04]
// [    0.0450]
// [+0.045]
// [     0.045]
// [0.045     ]
// [               0.045]
// [0.044999999999999998335]
// [0.045     ]
// [1E+22]
// [+1E+22]
// [1E+22]
// [ 1E+22]
// [1E+22]
// [1.00000E+22]
// [     1E+22]
// [1E+22     ]
// [000001E+22]
// [+00001E+22]
// [     1E+22]
// [1E+22]
// [1E+22]
// [1E+22]
// [1E+22]
// [1.E+22]
// [  1.00E+22]
// [+1E+22]
// [     1E+22]
// [1.0E+22   ]
// [               1E+22]
// [1E+22]
// [1E+22     ]
// [999999.5]
// [+999999.5]
// [999999.5]
// [ 999999.5]
// [999999.5]
// [999999.5]
// [  999999.5]
// [999999.5  ]
// [00999999.5]
// [+0999999.5]
// [     1E+06]
// [1E+06]
// [1E+06]
// [1E+06]
// [999999.5]
// [1.E+06]
// [  1.00E+06]
// [+1E+06]
// [     1E+06]
// [1.0E+06   ]
// [            999999.5]
// [999999.5]
// [999999.5  ]
// [0.9999995]
// [+0.9999995]
// [0.9999995]
// [ 0.9999995]
// [0.9999995]
// [0.9999995]
// [ 0.9999995]
// [0.9999995 ]
// [00.9999995]
// [+0.9999995]
// [         1]
// [1]
// [1]
// [1]
// [0.9999995]
// [1.]
// [      1.00]
// [+1]
// [         1]
// [1.0       ]
// [           0.9999995]
// [0.99999950000000004113]
// [0.9999995 ]
