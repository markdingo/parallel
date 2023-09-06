package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/markdingo/parallel"
)

// Calculate the sha256 of all files on the command line. Much like the sha256sum and
// shasum commands found on most Unixen systems. This demostrates concurrent running using
// the github.com/markdingo/parallel package. The sha256 function is run ``howMany'' times
// to simulate an intensive calculation.
//
// go build sha256sum.go
// time ./sha256sum *.go ../*.go
// time ./sha256sum -s *.go ../*.go

const (
	programName = "sha256"
)

type Opts struct {
	anyOrder bool // -a Runners can finish in any order
	help     bool // -h Print usage and exit
	limit    uint // -l Set LimitActiveRunners
	howMany  uint // -r repeat count
	serial   bool // -s Serialize - do not use parallel
}

var opts Opts

func fatal(messages ...string) {
	fmt.Fprintln(os.Stderr, "Fatal:", programName, strings.Join(messages, " "))
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, programName, "- calculate sha256 of files in parallel")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:", programName, "[-a] [-h] [-l n] [-r n] [-s] file1 [file2 ... filen]")
	flag.PrintDefaults()
}

func main() {
	flag.BoolVar(&opts.anyOrder, "a", false, "Runners can finish in any order")
	flag.BoolVar(&opts.help, "h", false, "Print usage message to stderr and exit(0)")
	flag.UintVar(&opts.limit, "l", 0, "Set LimitActiveRunners to 'n'")
	flag.UintVar(&opts.howMany, "r", 16000, "Repeat sha256 'n' times per file")
	flag.BoolVar(&opts.serial, "s", false, "Bypass parallel and serially process")

	flag.Parse()
	if opts.help {
		usage()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fatal("Need to provide at least one filename on the command line")
	}

	fmt.Println("howMany", opts.howMany, "Active", opts.limit)
	if opts.serial {
		var f sha256File
		for _, filename := range args {
			f.filename = filename
			f.run(os.Stdout, os.Stderr)
		}
		return
	}

	grp, _ := parallel.NewGroup(parallel.OrderRunners(!opts.anyOrder),
		parallel.LimitActiveRunners(opts.limit))

	// This usage of Group.Add uses a structure function to pass additional parameters
	// to calcsha256().

	for _, a := range args {
		f := &sha256File{filename: a}
		grp.Add("", "", f.run)
	}

	grp.Run()
	grp.Wait()
}

type sha256File struct {
	filename string
}

func (f *sha256File) run(stdout, stderr io.Writer) {
	of, err := os.Open(f.filename)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return
	}
	defer of.Close()

	data, err := io.ReadAll(of)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return
	}
	var md []byte
	var rdr bytes.Reader
	for ix := opts.howMany; ix > 0; ix-- {
		h := sha256.New()
		rdr.Reset(data)
		if _, err := io.Copy(h, &rdr); err != nil {
			fmt.Fprintln(stderr, err)
			return
		}
		md = h.Sum(nil)
	}

	fmt.Fprintf(stdout, "%x  %s\n", md, f.filename)
}
