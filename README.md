<!-- Always newline after period so diffs are easier to read. -->
# Substantially reduce latency for `go` command-line programs

## Introduction

`parallel` coordinates and serialises output written to stdout and stderr by concurrent
goroutines.
The goal is to make it easy for go command-line tools to process all their arguments in
parallel, thus reducing latency, while maintaining the illusion that each argument is
processed serially.

`parallel` is designed for commands which process multiple arguments similar to:

```
    $ grep pattern file1 file2...
    $ sha256 filea fileb filec...
    $ gzip --verbose --best jan.tar feb.tar mar.tar...
    $ checkzone --verbose domain1 domain2 domain3...
    $ wget -O all.html https://google.com https://yahoo.com https://apple.com
```

Normally such commands are constrained from running a goroutine-per-argument because their
output is randomly intermingled and thus rendered unintelligible.
This is unfortunate as go commands are well suited to a goroutine-per-argument style of
implementation.

`parallel` removes this constraint and enables a goroutine-per-argument approach by
ensuring output is not intermingled and that all output appears in serial argument order
with minimal changes to the command-line program.

For those familiar with [GNU parallel](https://www.gnu.org/software/parallel/), this
package achieves similar functionality within commands written in go.

### Project Status

[![Build Status](https://github.com/markdingo/parallel/actions/workflows/go.yml/badge.svg)](https://github.com/markdingo/parallel/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/markdingo/parallel/branch/main/graph/badge.svg)](https://codecov.io/gh/markdingo/parallel)
[![CodeQL](https://github.com/markdingo/parallel/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/markdingo/parallel/actions/workflows/codeql-analysis.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/markdingo/parallel)](https://goreportcard.com/report/github.com/markdingo/parallel)
[![Go Reference](https://pkg.go.dev/badge/github.com/markdingo/parallel.svg)](https://pkg.go.dev/github.com/markdingo/parallel)

`parallel` is known to compile and run on go versions 1.20 and beyond.

## Background

A key feature of `go` is the ease with which programs can use goroutines to reduce latency
as well as take advantage of modern multi-core CPUs.
Unfortunately these advantages are rarely taken up by command-line programs since they need
to present output to stdout and stderr in serial-processing order.
The end-result is that most `go` command-line programs revert to processing arguments
serially and thus incur much greater latency than they otherwise could.
This is particularly true of command-line programs which reach out across the network and
incur significant network delays.

`parallel` removes this impediment by allowing a command-line program to run a
goroutine-per-argument while still presenting their output in apparent serial-processing
order.

## Target Audience

`parallel` is designed for commands which process multiple independent arguments which
take a noticeable amount of time to complete; whether that be due to CPU time, network
latency or other external factors.
The general idea is that a command uses `parallel` to start a separate goroutine for each
command-line argument and these goroutines run in parallel to reduce latency for the total
run-time of the command.
For its part, `parallel` coordinates the output of these goroutines such that the illusion
of serial processing is maintained.

This latency reduction is particularly apparent for network-centric commands.
By using `parallel` the total latency is bound by the slowest argument, thus O(1) as
opposed to the total number of arguments which is O(n).
Clearly as `'n'` grows, `parallel` offers more latency reduction.

## Idiomatic Code

Assuming your current code serially processes command-line arguments something like this:

```
for _, arg := range os.Args {
    handleArg(arg, os.Stdout, os.Stderr)        // Dispatch to handler
}
```

then to process all arguments in parallel while still generating identical output, your
replacement code will look something like this:

```
group := parallel.NewGroup()

for _, arg := range os.Args {
    argCopy := arg                                      // (pre go 1.21.1 semantics)
    group.Add("", "",
              func(stdout, stderr io.Writer) {          // Use a Closure function
                  handleArg(argCopy, stdout, stderr)    // Dispatch to handler
              })
}

group.Run()
group.Wait()
```

Assuming `handleArg` is self-contained (which is to say that it does not modify global
data) and consistently refers to the provided io.Writers for stdout and stderr, no other
changes are required.

This example pretty much demonstrates all of the `parallel` functionality.  IOWs,
`parallel` is not a complicated package.
Nonetheless, for those interested in more detail, complete package documentation is
available at [parallel](https://pkg.go.dev/github.com/markdingo/parallel).

## Installation

When imported by your program, `github.com/markdingo/parallel` should automatically
install with `go mod tidy` or `go mod build`.

If not, try running:

```
go get github.com/markdingo/parallel
```

Once installed, you can run the package tests with:

```
 go test -v github.com/markdingo/parallel
```

as well as display the package documentation with:

```
 go doc github.com/markdingo/parallel
```


## Community

If you have any problems using `parallel` or suggestions on how it can do a better job,
don't hesitate to create an [issue](https://github.com/markdingo/parallel/issues) on
the project home page. This package can only improve with your feedback.

## Copyright and License

`parallel` is Copyright :copyright: 2023 Mark Delany. This software is licensed
under the BSD 2-Clause "Simplified" License.
