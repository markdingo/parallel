package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/markdingo/parallel"
)

// The nested program demonstrates the use of multiple parallel Groups within a single
// program. This program starts a number of RunFuncs which create a parallel Group to
// recursively call itself to create yet more Groups and run yet more RunFuncs. At the
// bottom of the recursion is a trivial function which sleeps for a random amount of time
// and prints an identifier showing the recursion level and the RunFunc index.
//
// If working correctly with -k set, the output should be in strictly ascending order
// regardless of the results of the random sleep. Without -k set you should see random
// output order.
//
// If the parallel package is working correctly then:
//
// ./nested -k >/tmp/nested.raw
// sort </tmp/nested.raw >/tmp/nested.sorted
// diff /tmp/nested.raw /tmp/nested.sorted
//
// Should result in zero differences.
//
// go build nested.go
// ./nested [-h] [-k] [--depth n] [--width n]
//
// Where --depth is the recursion depth and --width is the number of RunFuncs per
// recursion depth. The -k option sets parallel.OrderRunners(true) which demonstrates how
// nested Groups still maintain apparent serial order.

const (
	programName = "nested"
)

type Opts struct {
	help      bool // -h Print usage and exit
	keepOrder bool // Output is printed in creation order
	depth     int
	width     int
}

var opts Opts

func fatal(messages ...string) {
	fmt.Fprintln(os.Stderr, "Fatal:", programName, strings.Join(messages, " "))
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, programName, "- recursively create multiple parallel Groups")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:", programName, "[-h] [--depth n] [--width n]")
	flag.PrintDefaults()
}

func main() {
	flag.BoolVar(&opts.help, "h", false, "Print usage message to stderr and exit(0)")
	flag.BoolVar(&opts.keepOrder, "k", false, "Output is printed in creation order")
	flag.IntVar(&opts.depth, "depth", 5, "Recursion depth")
	flag.IntVar(&opts.width, "width", 2, "RunFun width per recursion depth")

	flag.Parse()
	if opts.help {
		usage()
		return
	}

	args := flag.Args()
	if len(args) > 0 {
		fatal("Unexpected goop on the command line", strings.Join(args, " "))
	}

	grp, _ := parallel.NewGroup(parallel.OrderRunners(opts.keepOrder))
	grp.Add("", "", func(out, err io.Writer) {
		recurs("", 0, 0, out, err)
	})
	grp.Run()
	grp.Wait()
}

func recurs(prefix string, depth, widthIndex int, stdout, stderr io.Writer) {
	prefix += fmt.Sprintf("%d.", widthIndex)
	if depth == opts.depth {
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(250))) //  Upto 1/4s
		fmt.Fprintf(stdout, "%s %d-%d\n", prefix, depth, widthIndex)
		return
	}

	grp, _ := parallel.NewGroup(parallel.OrderRunners(opts.keepOrder),
		parallel.WithStdout(stdout), parallel.WithStderr(stderr))
	for ix := 0; ix < opts.width; ix++ {
		ix := ix // Pre 1.22 semantics
		grp.Add("", "", func(out, err io.Writer) {
			recurs(prefix, depth+1, ix, out, err)
		})
	}
	grp.Run()
	grp.Wait()
}
