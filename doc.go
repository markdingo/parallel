/*
Package parallel coordinates and serialises output written to stdout and stderr by
concurrent goroutines.  The goal is to make it easy for go command-line tools to process
all arguments in parallel, thus reducing latency, while maintaining the illusion that each
argument is processed serially.

This package is designed for commands which process multiple arguments similar to:

	$ grep pattern file1 file2...
	$ sha256 filea fileb filec...
	$ gzip --verbose --best jan.tar feb.tar mar.tar...
	$ checkzone --verbose domain1 domain2 domain3...
	$ wget -O all.html https://google.com https://yahoo.com https://apple.com

Normally such commands are constrained from running a goroutine per argument because their
output is randomly intermingled and thus rendered unintelligible. This is unfortunate as
go commands are well suited to this style of implementation.

The parallel package removes this constraint and enables a goroutine per argument by
ensuring output is not intermingled and that all output appears in serial argument order.

For those familiar with the [GNU parallel], this package achieves similar functionality
within commands written in go.

# Comparison to [x/sync/errgroup]

Superficially “parallel” appears similar to [x/sync/errgroup] in the standard library,
however they perform quite different functions in that “errgroup” is designed to manage
goroutines working to achieve a common goal where a single failure causes a collective
failure. In contrast, “parallel” is designed to manage independent goroutines contained in
a command-line program. Most importantly, “parallel” is largely about coordinating output
to stdout and stderr whereas “errgroup” plays no part in that.

# Caveat

When adapting existing commands to use “parallel”, programmers needs to be aware of newly
created concurrent interactions between goroutines which may not have existed with the
original implementation. In this situation it is suggested that such commands initially be
built and tested with the “-race” option.

# How To Use

Idiomatic use is to populate a [Group] with a [RunFunc] for each command-line
argument. Once populated, [Group.Run] starts a goroutine for each [RunFunc] in the
Group. Following that, a call to [Group.Wait] is made to wait for completion of all
RunFuncs.

If your current code serially processes command-line arguments something like this:

	for _, arg := range os.Args {
	    handleArg(arg, os.Stdout, os.Stderr)	// Dispatch to handler
	}

then to process all arguments in parallel while still generating serially identical
output, your replacement code will look something like this:

	group := parallel.NewGroup()

	for _, arg := range os.Args {
	    argCopy := arg					// (pre 1.21.1 semantics)
	    group.Add("", "",
		      func(stdout, stderr io.Writer) {		// Closure function
			  handleArg(argCopy, stdout, stderr)	// Dispatch to handler
		      })
	}

	group.Run()
	group.Wait()

which in this case uses a closure to satisfy the [RunFunc] signature. An alternative is to
use a struct function to satisfy the signature, as described in [RunFunc].

The main change you have to make is to ensure that your [RunFunc] *always* uses the
io.Writers designated as stdout and stderr as *all* output must be written to these
io.Writers, never to os.Stdout or os.Stderr directly.

Further examples of how to use “parallel” can be found in the _examples sub-directory.

# Capturing references to os.Stdout and os.Stderr

If your code-base is large or complicated it may not be easy to find every relevant
reference to os.Stdout and os.Stderr, however since these variables can be modified it's
relatively easy to at least identify which output is still being written directly. One way
to do this is to replace os.Stdout and os.Stderr with your own os.File *after* calling
[NewGroup]. E.g.:

	grp,_ := parallel.NewGroup(...)
	os.Stdout, _ = os.OpenFile("out.missed", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	os.Stderr = os.Stdout

then examine "out.missed" after running your test suite.

(More sophisiticated blocking and capturing at the time of occurrence is possible with
[os.NewFile] and Unix named pipes.)

# Timeouts and error returns

Unlike [GNU parallel] this package does not support detecting [RunFunc] timeouts or
errors, nor does it offer retry attempts or job resumption. Firstly because this adds a
lot of opinionated complexity to the API and secondly because such features designed to
best suit individual applications can be readily added via a closure or a struct function.

As one example, if an application wants their [RunFunc] to stop the whole [Group] on error
somewhat like [x/sync/errgroup], one approach is to create a “terminate” channel which is
populated on error. Each [RunFunc] monitors this channel and terminates immediately if it
is written to.

	group := parallel.NewGroup()
	terminate := make(chan any, len(os.Args))

	for _, arg := range os.Args {
	    argCopy := arg
	    group.Add("", "",
	         func(stdout, stderr io.Writer) {
	            err := handleArg(terminate, argCopy, stdout, stderr)
	            if err != nil {
	                terminate <- any
	            }
	         })
	}

	group.Run()
	group.Wait()

# Concurrency

Serial processing command-line programs typically do not have to worry about concurrency
controls, but when adopting this package, they will now have to do so. Such programs
should be particularly aware of modifying shared data such as global counters, progress
meters and similar. All such modifications need to be concurrency protected. Naturally
access to read-only data structures such as option settings do not require any protection.

# Design

The parallel package achieves most of its functionality via a “Pipeline” assigned
to each [RunFunc]. Output is steered thru writers in the pipeline based on the Group
config options. Specific features are handled by different writers such as “head” and
“tagger”. The theory being that new writers which implement future functionality can
easily slot into the pipeline.

There are currently two types of Pipelines: Queue and Passthru.

# Queue Pipeline

The initial pipeline for each [RunFunc] is normally a Queue Pipeline which starts in
“background” mode because, much like a background command in a shell, it continues to run,
except that all output is buffered until the pipeline is switched to “foreground”
mode. This diagram illustrates the “writers” in a Queue Pipeline.

	    RunFunc
	(stdout,   stderr)
	   v         v
	   |         |
	  head      head        Adapts io.Writer to parallel.writer
	   |_       _|
	     |     |
	      queue             Buffers arrival order of outputs
	    _|     |_
	   |         |
	 tagger    tagger       Prefix each line with 'tag' if set
	   |         |
	  tail      tail        Serialises Group output access
	   |         |          Adapts parallel.writer to io.Writer
	 Group     Group
	 stdout    stderr
	   |         |
	   v         v

The “queue” writer buffers and tracks the arrival order of stdout and stderr
outputs. Arrival order has to be maintained to ensure correct transfer to the Group
io.Writers. This is necessary because the Group io.Writers may well be one and the same,
e.g. if a command is invoked with stderr re-directed to stdout or if both stdout and
stderr are a terminal.

When a [RunFunc] becomes a candidate for foreground output (because it has percolated to
the front of the queue with OrderRunners(true)), the “queue” buffered output is written to
the Group io.Writers and the Queue Pipeline is switched to "foreground" mode.

# Passthru Pipeline

Passthru is a skeletal pipeline intended as a diagnostic tool which bypasses most of the
“parallel” functionality. It is created when the Group is constructed with Passthru(true).

	    RunFunc
	(stdout,   stderr)
	   v	     v
	   |	     |
	  head	    head	Adapts io.Writer to parallel.writer
	   |	     |
	  tail	    tail	Serialises Group output access
	   |	     |		Adapts parallel.writer to io.Writer
	   |	     |
	 Group	   Group
	 stdout	   stderr
	   |	     |
	   v	     v

The main reason for using Passthru is for situations where you have suspicions about the
parallel package and want a relatively unfiltered view of what your RunFuncs are
generating.

[GNU parallel]: https://www.gnu.org/software/parallel/
[x/sync/errgroup]: https://pkg.go.dev/golang.org/x/sync/errgroup
*/
package parallel
