// phase: 05

// Go's %O of int values with every flag, width and precision: the integer formatting of
// fmt/format.go.
package main

import "fmt"

func main() {
	fmt.Printf("[%O]\n", 0)
	fmt.Printf("[%+O]\n", 0)
	fmt.Printf("[%-O]\n", 0)
	fmt.Printf("[% O]\n", 0)
	fmt.Printf("[%0O]\n", 0)
	fmt.Printf("[%#O]\n", 0)
	fmt.Printf("[%8O]\n", 0)
	fmt.Printf("[%-8O]\n", 0)
	fmt.Printf("[%08O]\n", 0)
	fmt.Printf("[%+8O]\n", 0)
	fmt.Printf("[%+08O]\n", 0)
	fmt.Printf("[% 8O]\n", 0)
	fmt.Printf("[%8.3O]\n", 0)
	fmt.Printf("[%.3O]\n", 0)
	fmt.Printf("[%.0O]\n", 0)
	fmt.Printf("[%#8O]\n", 0)
	fmt.Printf("[%#08O]\n", 0)
	fmt.Printf("[%-#8O]\n", 0)
	fmt.Printf("[%+.5O]\n", 0)
	fmt.Printf("[%0-8O]\n", 0)
	fmt.Printf("[%12.8O]\n", 0)
	fmt.Printf("[%-12.8O]\n", 0)
	fmt.Printf("[%#.4O]\n", 0)
	fmt.Printf("[%+#O]\n", 0)
	fmt.Printf("[%O]\n", 1)
	fmt.Printf("[%+O]\n", 1)
	fmt.Printf("[%-O]\n", 1)
	fmt.Printf("[% O]\n", 1)
	fmt.Printf("[%0O]\n", 1)
	fmt.Printf("[%#O]\n", 1)
	fmt.Printf("[%8O]\n", 1)
	fmt.Printf("[%-8O]\n", 1)
	fmt.Printf("[%08O]\n", 1)
	fmt.Printf("[%+8O]\n", 1)
	fmt.Printf("[%+08O]\n", 1)
	fmt.Printf("[% 8O]\n", 1)
	fmt.Printf("[%8.3O]\n", 1)
	fmt.Printf("[%.3O]\n", 1)
	fmt.Printf("[%.0O]\n", 1)
	fmt.Printf("[%#8O]\n", 1)
	fmt.Printf("[%#08O]\n", 1)
	fmt.Printf("[%-#8O]\n", 1)
	fmt.Printf("[%+.5O]\n", 1)
	fmt.Printf("[%0-8O]\n", 1)
	fmt.Printf("[%12.8O]\n", 1)
	fmt.Printf("[%-12.8O]\n", 1)
	fmt.Printf("[%#.4O]\n", 1)
	fmt.Printf("[%+#O]\n", 1)
	fmt.Printf("[%O]\n", -1)
	fmt.Printf("[%+O]\n", -1)
	fmt.Printf("[%-O]\n", -1)
	fmt.Printf("[% O]\n", -1)
	fmt.Printf("[%0O]\n", -1)
	fmt.Printf("[%#O]\n", -1)
	fmt.Printf("[%8O]\n", -1)
	fmt.Printf("[%-8O]\n", -1)
	fmt.Printf("[%08O]\n", -1)
	fmt.Printf("[%+8O]\n", -1)
	fmt.Printf("[%+08O]\n", -1)
	fmt.Printf("[% 8O]\n", -1)
	fmt.Printf("[%8.3O]\n", -1)
	fmt.Printf("[%.3O]\n", -1)
	fmt.Printf("[%.0O]\n", -1)
	fmt.Printf("[%#8O]\n", -1)
	fmt.Printf("[%#08O]\n", -1)
	fmt.Printf("[%-#8O]\n", -1)
	fmt.Printf("[%+.5O]\n", -1)
	fmt.Printf("[%0-8O]\n", -1)
	fmt.Printf("[%12.8O]\n", -1)
	fmt.Printf("[%-12.8O]\n", -1)
	fmt.Printf("[%#.4O]\n", -1)
	fmt.Printf("[%+#O]\n", -1)
	fmt.Printf("[%O]\n", 42)
	fmt.Printf("[%+O]\n", 42)
	fmt.Printf("[%-O]\n", 42)
	fmt.Printf("[% O]\n", 42)
	fmt.Printf("[%0O]\n", 42)
	fmt.Printf("[%#O]\n", 42)
	fmt.Printf("[%8O]\n", 42)
	fmt.Printf("[%-8O]\n", 42)
	fmt.Printf("[%08O]\n", 42)
	fmt.Printf("[%+8O]\n", 42)
	fmt.Printf("[%+08O]\n", 42)
	fmt.Printf("[% 8O]\n", 42)
	fmt.Printf("[%8.3O]\n", 42)
	fmt.Printf("[%.3O]\n", 42)
	fmt.Printf("[%.0O]\n", 42)
	fmt.Printf("[%#8O]\n", 42)
	fmt.Printf("[%#08O]\n", 42)
	fmt.Printf("[%-#8O]\n", 42)
	fmt.Printf("[%+.5O]\n", 42)
	fmt.Printf("[%0-8O]\n", 42)
	fmt.Printf("[%12.8O]\n", 42)
	fmt.Printf("[%-12.8O]\n", 42)
	fmt.Printf("[%#.4O]\n", 42)
	fmt.Printf("[%+#O]\n", 42)
	fmt.Printf("[%O]\n", -42)
	fmt.Printf("[%+O]\n", -42)
	fmt.Printf("[%-O]\n", -42)
	fmt.Printf("[% O]\n", -42)
	fmt.Printf("[%0O]\n", -42)
	fmt.Printf("[%#O]\n", -42)
	fmt.Printf("[%8O]\n", -42)
	fmt.Printf("[%-8O]\n", -42)
	fmt.Printf("[%08O]\n", -42)
	fmt.Printf("[%+8O]\n", -42)
	fmt.Printf("[%+08O]\n", -42)
	fmt.Printf("[% 8O]\n", -42)
	fmt.Printf("[%8.3O]\n", -42)
	fmt.Printf("[%.3O]\n", -42)
	fmt.Printf("[%.0O]\n", -42)
	fmt.Printf("[%#8O]\n", -42)
	fmt.Printf("[%#08O]\n", -42)
	fmt.Printf("[%-#8O]\n", -42)
	fmt.Printf("[%+.5O]\n", -42)
	fmt.Printf("[%0-8O]\n", -42)
	fmt.Printf("[%12.8O]\n", -42)
	fmt.Printf("[%-12.8O]\n", -42)
	fmt.Printf("[%#.4O]\n", -42)
	fmt.Printf("[%+#O]\n", -42)
	fmt.Printf("[%O]\n", 1234567)
	fmt.Printf("[%+O]\n", 1234567)
	fmt.Printf("[%-O]\n", 1234567)
	fmt.Printf("[% O]\n", 1234567)
	fmt.Printf("[%0O]\n", 1234567)
	fmt.Printf("[%#O]\n", 1234567)
	fmt.Printf("[%8O]\n", 1234567)
	fmt.Printf("[%-8O]\n", 1234567)
	fmt.Printf("[%08O]\n", 1234567)
	fmt.Printf("[%+8O]\n", 1234567)
	fmt.Printf("[%+08O]\n", 1234567)
	fmt.Printf("[% 8O]\n", 1234567)
	fmt.Printf("[%8.3O]\n", 1234567)
	fmt.Printf("[%.3O]\n", 1234567)
	fmt.Printf("[%.0O]\n", 1234567)
	fmt.Printf("[%#8O]\n", 1234567)
	fmt.Printf("[%#08O]\n", 1234567)
	fmt.Printf("[%-#8O]\n", 1234567)
	fmt.Printf("[%+.5O]\n", 1234567)
	fmt.Printf("[%0-8O]\n", 1234567)
	fmt.Printf("[%12.8O]\n", 1234567)
	fmt.Printf("[%-12.8O]\n", 1234567)
	fmt.Printf("[%#.4O]\n", 1234567)
	fmt.Printf("[%+#O]\n", 1234567)
	fmt.Printf("[%O]\n", -1234567)
	fmt.Printf("[%+O]\n", -1234567)
	fmt.Printf("[%-O]\n", -1234567)
	fmt.Printf("[% O]\n", -1234567)
	fmt.Printf("[%0O]\n", -1234567)
	fmt.Printf("[%#O]\n", -1234567)
	fmt.Printf("[%8O]\n", -1234567)
	fmt.Printf("[%-8O]\n", -1234567)
	fmt.Printf("[%08O]\n", -1234567)
	fmt.Printf("[%+8O]\n", -1234567)
	fmt.Printf("[%+08O]\n", -1234567)
	fmt.Printf("[% 8O]\n", -1234567)
	fmt.Printf("[%8.3O]\n", -1234567)
	fmt.Printf("[%.3O]\n", -1234567)
	fmt.Printf("[%.0O]\n", -1234567)
	fmt.Printf("[%#8O]\n", -1234567)
	fmt.Printf("[%#08O]\n", -1234567)
	fmt.Printf("[%-#8O]\n", -1234567)
	fmt.Printf("[%+.5O]\n", -1234567)
	fmt.Printf("[%0-8O]\n", -1234567)
	fmt.Printf("[%12.8O]\n", -1234567)
	fmt.Printf("[%-12.8O]\n", -1234567)
	fmt.Printf("[%#.4O]\n", -1234567)
	fmt.Printf("[%+#O]\n", -1234567)
}

// Output:
// [0o0]
// [+0o0]
// [0o0]
// [ 0o0]
// [0o0]
// [0o0]
// [     0o0]
// [0o0     ]
// [0o00000000]
// [    +0o0]
// [+0o0000000]
// [     0o0]
// [   0o000]
// [0o000]
// []
// [     0o0]
// [0o00000000]
// [0o0     ]
// [+0o00000]
// [0o0     ]
// [  0o00000000]
// [0o00000000  ]
// [0o0000]
// [+0o0]
// [0o1]
// [+0o1]
// [0o1]
// [ 0o1]
// [0o1]
// [0o01]
// [     0o1]
// [0o1     ]
// [0o00000001]
// [    +0o1]
// [+0o0000001]
// [     0o1]
// [   0o001]
// [0o001]
// [0o1]
// [    0o01]
// [0o00000001]
// [0o01    ]
// [+0o00001]
// [0o1     ]
// [  0o00000001]
// [0o00000001  ]
// [0o0001]
// [+0o01]
// [-0o1]
// [-0o1]
// [-0o1]
// [-0o1]
// [-0o1]
// [-0o01]
// [    -0o1]
// [-0o1    ]
// [-0o0000001]
// [    -0o1]
// [-0o0000001]
// [    -0o1]
// [  -0o001]
// [-0o001]
// [-0o1]
// [   -0o01]
// [-0o0000001]
// [-0o01   ]
// [-0o00001]
// [-0o1    ]
// [ -0o00000001]
// [-0o00000001 ]
// [-0o0001]
// [-0o01]
// [0o52]
// [+0o52]
// [0o52]
// [ 0o52]
// [0o52]
// [0o052]
// [    0o52]
// [0o52    ]
// [0o00000052]
// [   +0o52]
// [+0o0000052]
// [    0o52]
// [   0o052]
// [0o052]
// [0o52]
// [   0o052]
// [0o00000052]
// [0o052   ]
// [+0o00052]
// [0o52    ]
// [  0o00000052]
// [0o00000052  ]
// [0o0052]
// [+0o052]
// [-0o52]
// [-0o52]
// [-0o52]
// [-0o52]
// [-0o52]
// [-0o052]
// [   -0o52]
// [-0o52   ]
// [-0o0000052]
// [   -0o52]
// [-0o0000052]
// [   -0o52]
// [  -0o052]
// [-0o052]
// [-0o52]
// [  -0o052]
// [-0o0000052]
// [-0o052  ]
// [-0o00052]
// [-0o52   ]
// [ -0o00000052]
// [-0o00000052 ]
// [-0o0052]
// [-0o052]
// [0o4553207]
// [+0o4553207]
// [0o4553207]
// [ 0o4553207]
// [0o4553207]
// [0o04553207]
// [0o4553207]
// [0o4553207]
// [0o04553207]
// [+0o4553207]
// [+0o4553207]
// [ 0o4553207]
// [0o4553207]
// [0o4553207]
// [0o4553207]
// [0o04553207]
// [0o04553207]
// [0o04553207]
// [+0o4553207]
// [0o4553207]
// [  0o04553207]
// [0o04553207  ]
// [0o04553207]
// [+0o04553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o04553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o4553207]
// [-0o04553207]
// [-0o04553207]
// [-0o04553207]
// [-0o4553207]
// [-0o4553207]
// [ -0o04553207]
// [-0o04553207 ]
// [-0o04553207]
// [-0o04553207]
