// phase: 08
// ExitCode: 66
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found ([6-9][0-9]|[1-9][0-9][0-9]) data race\(s\)

// Sixty variables, each with two goroutines that race on it: every report is written (Go prints them all),
// and the count is right.
package main

import "fmt"

var v0 int
var v1 int
var v2 int
var v3 int
var v4 int
var v5 int
var v6 int
var v7 int
var v8 int
var v9 int
var v10 int
var v11 int
var v12 int
var v13 int
var v14 int
var v15 int
var v16 int
var v17 int
var v18 int
var v19 int
var v20 int
var v21 int
var v22 int
var v23 int
var v24 int
var v25 int
var v26 int
var v27 int
var v28 int
var v29 int
var v30 int
var v31 int
var v32 int
var v33 int
var v34 int
var v35 int
var v36 int
var v37 int
var v38 int
var v39 int
var v40 int
var v41 int
var v42 int
var v43 int
var v44 int
var v45 int
var v46 int
var v47 int
var v48 int
var v49 int
var v50 int
var v51 int
var v52 int
var v53 int
var v54 int
var v55 int
var v56 int
var v57 int
var v58 int
var v59 int

func main() {
	done := make(chan bool)
	go func() { for j := 0; j < 100; j++ { v0++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v0-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v1++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v1-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v2++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v2-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v3++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v3-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v4++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v4-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v5++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v5-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v6++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v6-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v7++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v7-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v8++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v8-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v9++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v9-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v10++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v10-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v11++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v11-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v12++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v12-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v13++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v13-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v14++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v14-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v15++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v15-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v16++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v16-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v17++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v17-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v18++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v18-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v19++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v19-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v20++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v20-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v21++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v21-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v22++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v22-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v23++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v23-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v24++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v24-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v25++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v25-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v26++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v26-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v27++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v27-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v28++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v28-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v29++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v29-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v30++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v30-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v31++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v31-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v32++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v32-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v33++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v33-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v34++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v34-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v35++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v35-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v36++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v36-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v37++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v37-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v38++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v38-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v39++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v39-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v40++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v40-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v41++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v41-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v42++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v42-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v43++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v43-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v44++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v44-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v45++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v45-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v46++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v46-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v47++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v47-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v48++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v48-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v49++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v49-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v50++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v50-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v51++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v51-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v52++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v52-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v53++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v53-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v54++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v54-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v55++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v55-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v56++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v56-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v57++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v57-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v58++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v58-- }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v59++ }; done <- true }()
	go func() { for j := 0; j < 100; j++ { v59-- }; done <- true }()
	for k := 0; k < 120; k++ {
		<-done
	}
	fmt.Println("done")
}

// Output:
// done
