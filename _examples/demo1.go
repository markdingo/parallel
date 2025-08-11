package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/markdingo/parallel"
)

// This demo program creates runners which write a configured number of lines and sleep
// for 200ms after each write. The Group is configured to exhibit some of the subtle
// consequences of parallel. Specifically:
//
// 1. id:1 stalls after consuming its memory quota of 100 bytes, but id:0 keeps running
//    because it is in foreground mode and thus writing directly to os.Stdout. View this
//    with "go run demo1.go >/dev/null"
//
// 2. There are three runners created but concurrent runners are limited to just two
//    therefore you should see that only runners id:0 and id:1 write output until id:0
//    exits, then id:2 runs. View this with "go run demo1.go >/dev/null"
//
// 3. When a runner becomes the foreground runner, all buffered output is transferred to
//    os.Stdout and then all subsequent output is sent directly to os.Stdout. This can be
//    seen as the first 10 or so lines of output for id:1 and id2: being written without
//    any delay but then their subseqent lines are emitted every 200ms or so.  View this
//    with "go run demo1.go 2>/dev/null"
//
// Regardless of these consequences, as far as the viewer is concerned, each runner ran
// serially and visually, normally.

type runner struct {
	id      int
	howMany int
	line    string
}

func (rnr *runner) run(stdout, stderr io.Writer) {
	total := 0
	for count := range rnr.howMany {
		fmt.Fprintf(os.Stderr, "id:%d Write #%d for %d\n", rnr.id, count, len(rnr.line))
		n, _ := stdout.Write([]byte(rnr.line))
		total += n
		fmt.Fprintf(os.Stderr, "id:%d wrote %d total: %d\n", rnr.id, n, total)
		time.Sleep(time.Millisecond * 200)
	}

	fmt.Fprintf(os.Stderr, "id:%d Exiting. total: %d\n", rnr.id, total)
}

func main() {
	grp, err := parallel.NewGroup(parallel.OrderRunners(true),
		parallel.LimitMemoryPerRunner(100), parallel.LimitActiveRunners(2))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	r0 := &runner{id: 0, howMany: 10, line: "19 bytes + NLxxxxxx\n"} // 200 bytes in total
	r1 := &runner{id: 1, howMany: 15, line: "19 bytes + NLyyyyyy\n"} // 300 bytes in total
	r2 := &runner{id: 2, howMany: 20, line: "19 bytes + NLzzzzzz\n"} // 400 bytes in total
	grp.Add("id:0\t", "", r0.run)
	grp.Add("id:1\t", "", r1.run)
	grp.Add("id:2\t", "", r2.run)
	grp.Run()
	grp.Wait()
}
