<!-- Always newline after period so diffs are easier to read. -->
## Introduction

This `_examples` directory contains a number of programs which demonstrate different
facets of the `parallel` package. While all these demo programs compile and run, the major
purpose is to show coding examples. If you do want to run the demos, the easiest way to do
so is via the Makefile with `make demos`.

## Program Descriptions

### demo1

demo1 visually shows how RunFuncs are stalled when they exceed their memory limits and
also how they exhibit "liveliness" when they transition to foreground mode.

### para

para is a vastly simplified version of the GNU parallel command. It has *just* enough
functionality to demonstrate how the `parallel` package provides similar functionality
to the GNU command.

### sha256sum

Demonstrates how a command, when processing its command line argument concurrently with
`parallel` can significantly reduce its elapse time.

### structfunc

Demonstrates the use of a struct function to meet the RunFunc signature.
