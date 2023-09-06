package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/markdingo/parallel"
)

// A vastly simplified version of the GNU parallel program which demonstrate the use of
// the github.com/markdingo/parallel package.
//
// go build para.go
// ./para -k wc -l ::: *.go

const (
	programName = "para"
	magic       = ":::"
)

type Opts struct {
	help      bool   // -h Print usage and exit
	group     bool   // Output is only printed when the command finishes (stdout first)
	keepOrder bool   // Output is printed in options ordered
	tag       bool   // Prefix each output with its corresponding option (+\t)
	sep       string // Separator string between each command output

	command []string
}

var opts Opts

func fatal(messages ...string) {
	fmt.Fprintln(os.Stderr, "Fatal:", programName, strings.Join(messages, " "))
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, programName, "- execute shell command with arguments in parallel")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:", programName,
		"[-h] [gkt] [-s sep] shell-command [options] ::: arguments")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, `
Example:
    para -k wc -l ::: file1 file2 file3
`)
}

func main() {
	flag.BoolVar(&opts.help, "h", false, "Print usage message to stderr and exit(0)")
	flag.BoolVar(&opts.group, "g", false,
		"Output is only printed when the command finishes (stdout first)")
	flag.BoolVar(&opts.keepOrder, "k", false, "Output is printed in options order")
	flag.BoolVar(&opts.tag, "t", false, "Prefix each output with its argument (+'\\t')")
	flag.StringVar(&opts.sep, "s", "", "Separator string between each command output (+'\\n')")

	flag.Parse()
	if opts.help {
		usage()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fatal("Need to provide a 'shell-command' on the command line")
	}
	opts.command = append(opts.command, args[0])

	args = args[1:]

	// Search for our ":::" magic token. Anything prior to ":::" is part of the
	// command and anything subsequent are args for each RunFunc.

	magicSeen := -1
	for ix, a := range args {
		if a == magic {
			magicSeen = ix
			break
		}
	}
	if magicSeen == -1 {
		fatal("':::' delimiter not found after 'shell-command'")
	}

	// a0 a1 a2 ::: a4 a5

	if magicSeen > 0 {
		opts.command = append(opts.command, args[0:magicSeen]...)
	}
	if magicSeen == len(args)-1 {
		fatal("Need to provide at least one argument after ':::' delimiter")
	}
	args = args[magicSeen+1:]

	if len(opts.sep) > 0 {
		opts.sep = opts.sep + "\n"
	}
	grp, _ := parallel.NewGroup(parallel.WithStdoutSeparator(opts.sep),
		parallel.OrderRunners(opts.keepOrder),
		parallel.OrderStderr(opts.group),
	)

	// This usage of Group.Add uses a closure pass additional parameters to
	// runCommand()

	for _, a := range args {
		fArg := a
		gt := ""
		if opts.tag {
			gt = a + "\t"
		}
		grp.Add(gt, gt, func(out, err io.Writer) {
			cmd := append(opts.command, fArg)
			runCommand(cmd, out, err)
		})
	}

	grp.Run()
	grp.Wait()
}

func runCommand(args []string, stdout, stderr io.Writer) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
}
