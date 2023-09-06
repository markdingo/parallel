package main

// Demonstrate the use of a struct function to meet the RunFunc signature
// go run structfunc.go a b c

import (
	"fmt"
	"io"
	"os"

	"github.com/markdingo/parallel"
)

type file string

func main() {
	grp, _ := parallel.NewGroup()

	for _, f := range os.Args {
		runner := file(f)
		grp.Add(f+"\t", "", runner.run)
	}
	grp.Run()
	grp.Wait()
}

func (f *file) run(stdout, stderr io.Writer) {
	fmt.Fprintln(stdout, "Running")
}
