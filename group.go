package parallel

import (
	"container/list"
	"io"
	"sync"
)

type groupState int

const (
	groupIsZero groupState = iota
	groupIsAdding
	groupIsRunning
	groupIsWaiting
	groupIsDone
)

func (gs groupState) String() string {
	switch gs {
	case groupIsZero:
		return "groupIsZero"
	case groupIsAdding:
		return "groupIsAdding"
	case groupIsRunning:
		return "groupIsRunning"
	case groupIsWaiting:
		return "groupIsWaiting"
	case groupIsDone:
		return "groupIsDone"
	default:
		return "Invalid groupState"
	}
}

// Group manages a set of RunFuncs which are ultimately run in parallel by [Group.Run] and
// waited on by [Group.Wait]. A Group must be constructed with [NewGroup] otherwise a
// panic will occur when used.
//
// Group has a strict calling sequence: Multiple [Group.Add] calls followed by [Group.Run]
// followed by [Group.Wait] after which no calls to the Group are valid. Any deviation
// from this call sequence results in a panic.
//
// A Group cannot be reused, however multiple Groups can be created and used independently
// of each other.
//
// A Group is not concurrency-safe and must only be accessed by a single goroutine at a
// time. This does not imply anything about the concurrency of RunFuncs which normally are
// run concurrently and which are supplied with concurrency-safe io.Writers.
type Group struct {
	state   groupState // Ensure correct calling sequences
	runners *list.List // Appended in creation order

	// Shared across all runners
	outputMu sync.Mutex // Serialise access to config.stdout, config.stderr
	*config
	completed chan *list.Element // Element is contained in runners LL
}

// NewGroup constructs a [Group] ready for use. A [Group] must be constructed with this
// function as a "zero" struct causes panics. Multiple Options can be supplied to NewGroup
// to modify its behaviour e.g.:
//
//	group := parallel.NewGroup(WithSeparator("----\n"), OrderStderr(true))
//
// Unless otherwise set the [Group] is returned with OrderRunners(true) and
// StderrLast(false) which results in RunFuncs optimally writing to the Group output
// io.Writers (which in turn are normally [os.Stdout] and [os.Stderr]).
//
// NewGroup copies [os.Stdout] and [os.Stderr] to [Group] [io.Writers] for the purposes of
// writing the serialised output thus any subseqent changes to [os.Stdout] and [os.Stderr]
// have no impact on Group output.
//
// If an option is invalid or would result in a nonsensical Group an error is
// returned. Most errors are caused by Options which create the possibility that a
// [RunFunc] could stall indefinitely. See [Option] for more details.
func NewGroup(opts ...Option) (*Group, error) {
	cfg := newConfig()

	for _, opt := range opts {
		err := opt.apply(cfg)
		if err != nil {
			return nil, err
		}
	}

	// Make sure config is internally consistent
	err := cfg.checkConflicts()
	if err != nil {
		return nil, err
	}

	grp := &Group{state: groupIsAdding,
		completed: make(chan *list.Element),
		config:    cfg,
		runners:   list.New()}

	return grp, nil
}

// RunFunc is the caller supplied function added to a Group with [Group.Add]. Each RunFunc
// is run in a separate goroutine when [Group.Run] is called.
//
// A RunFunc *must* only write to the supplied io.Writers for stdout and stderr. It should
// never write to os.Stdout or os.Stderr directly unless that output is purposely intended
// to bypass this package (debug output being a possible example).
//
// Naturally each RunFunc instance should avoid interacting with other RunFunc instances
// unless suitable concurrency controls are used. This advice is mentioned because cli
// programs adopting this package may not have had to worry about concurrency previously
// but may have to do so now. For example, unprotected modification of global variables is
// not uncommon in serial cli programs.
//
// A RunFunc is free to create internal goroutines and concurrently write to the provided
// io.Writers... But all such goroutines must complete all writing before RunFunc returns
// and perhaps more importantly, concurrently writing to the assigned stdout/stderr
// io.Writers is likely to produce intermingled output, so it's not recommended.
//
// Typically a RunFunc is constructed as a closure to allow additional parameters to be
// passed to the desired function. This example shows a closure passing in a context and
// the individual command-line argument to each RunFunc.
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
//	defer cancel()
//	group := parallel.NewGroup()
//	for _, arg := range os.Args {
//	    argCopy := arg					   // (pre 1.21.1 semantics)
//	    group.Add("", "",
//		      func(stdout, stderr io.Writer) {		  // Closure function
//			  handleArg(ctx, argCopy, stdout, stderr) // Dispatch to handler
//		     })
//	}
//	group.Run()
//	group.Wait()
//
// An alternative strategy is to use a struct function which satisfies the [RunFunc]
// signature as shown here:
//
//	type file string
//
//	grp, _ := parallel.NewGroup()
//	for _, f := range os.Args {
//	    runner := file(f)
//	        grp.Add(f+"\t", "", runner.run)
//	    }
//	    grp.Run()
//	    grp.Wait()
//	}
//
//	func (f *file) run(stdout, stderr io.Writer) {
//	    fmt.Fprintln(stdout, "Running")
//	}
type RunFunc func(stdout, stderr io.Writer)

// Add appends the supplied RunFunc to the Group in anticipation of [Group.Run]. Typically
// a [RunFunc] is implemented as either a closure or a struct function so as to pass
// additional parameters to the underlying function. See [RunFunc] for details.
//
// Order of adding is important when the [OrderRunners] [Option] is set true as the
// current “front” RunFunc pipeline is arranged such that output typically goes directly
// to [os.Stdout] and [os.Stderr] which helps give the appearance of “liveliness”.
//
// The outTag and errTag strings are prepended to all output written by the RunFunc to
// stdout and stderr respectively and help mimic the “--tag” option in GNU parallel.
func (grp *Group) Add(outTag, errTag string, rFunc RunFunc) {
	grp.checkState(groupIsAdding)
	rnr := newRunner(outTag, errTag, rFunc)
	grp.runners.PushBack(rnr)
}

// Run starts each previously added [RunFunc] in a separate go routine and transitions the
// Group to being ready for a [Group.Wait] call.
//
// When a RunFunc pipeline percolates to the front of all “active” RunFuncs, it is set to
// foreground mode so that the output is potentially written directly to [os.Stdout] and
// [os.Stderr] thus affording “liveliness” output to the user.
//
// Run constrains the number of “active” RunFuncs to the [LimitActiveRunners] [Option]
// setting. If no limit is set, all RunFuncs are started immediately. A RunFunc is
// considered “active” only up until it returns, not when the output is sent to the Group
// io.Writers.
//
// Regardless of any [LimitActiveRunners] constraints, Run returns to the caller
// immediately with any outstanding RunFuncs started in the background as
// [LimitActiveRunners] allows.
func (grp *Group) Run() {
	grp.checkState(groupIsAdding)
	grp.state = groupIsRunning
	grp.buildPipelines()
	grp.startRunners()
}

func (grp *Group) buildPipelines() {
	first := true
	for e := grp.runners.Front(); e != nil; e = e.Next() {
		rnr := e.Value.(*runner)
		switch {
		case grp.passthru:
			rnr.buildPassthruPipeline(grp)
		case first && grp.foregroundAllowed(): // A max of one runner gets foreground
			rnr.buildQueuePipeline(grp)
			rnr.switchToForeground()
			first = false
		default: // The default is the queue pipeline
			rnr.buildQueuePipeline(grp)
		}
	}
}

// startRunners feeds RunFuncs to a pool of [LimitActiveRunners] workers. The flow of each
// *list.Element (a container for each runner) is:
//
// startRunners() -> todo chan -> worker() -> RunFunc() -> completed chan -> Wait() -> Remove
//
// Thus *list.Elements remains valid until processed by [Group.Wait] and removed from the
// list. This is important as [container/List.Remove] invalidates *list.Element.
//
// startRunners is normally called by the same goroutine which ultimately calls
// [Group.Wait] so it cannot stall, nor can it start goroutines which concurrently
// reference Group as Group is not concurrency-safe.
//
// To simplify the code path, startRunners is written as if [LimitActiveRunners] is always
// non-zero even tho it most often will be zero.
func (grp *Group) startRunners() {
	maxWorkers := grp.limitRunners // How many workers are started?
	if maxWorkers == 0 {           // If no configured limit, run them all at once
		maxWorkers = uint(grp.runners.Len())
	}

	todo := make(chan *list.Element) // Feeder writes, workers read
	for ; maxWorkers > 0; maxWorkers-- {
		go worker(todo, grp.completed)
	}

	// Copy runners to a separate container so that the feeder goroutine doesn't need
	// concurrent access to Group. We cannot clone to another container.List as that
	// moves the Elements over which in turn causes Elements to lose knowledge of
	// their original List which would break Wait. Thus a good ol' slice is used as a
	// container of RunFuncs to start.
	runners := make([]*list.Element, 0, grp.runners.Len())
	for e := grp.runners.Front(); e != nil; e = e.Next() {
		runners = append(runners, e)
	}

	// Start feeder goroutine
	go func() {
		for _, e := range runners {
			todo <- e
		}
		close(todo)
	}()
}

// Each worker accepts new work from the todo channel, runs the RunFunc then notifies the
// completion channel. It exits when the todo channel is closed.
func worker(todo chan *list.Element, completed chan *list.Element) {
	for e := range todo {
		rnr := e.Value.(*runner)
		rnr.rFunc(rnr.stdout, rnr.stderr)
		completed <- e
	}
}

// Wait waits for all RunFuncs started by [Group.Run] to complete before returning. If any
// RunFunc fails to complete, Wait will never return.
func (grp *Group) Wait() {
	grp.checkState(groupIsRunning)
	grp.state = groupIsWaiting

	defer func() {
		close(grp.completed)
		grp.state = groupIsDone
	}()

	// This loop is the core of the package logic. It waits on completed runners and
	// arranges the order that their output is sent to the Group io.Writers.
	//
	// This loop removes completed runners from the grp.runners List until Len() goes
	// to zero. An inconvenient side-effect of removing an Element is that the Next()
	// and Prev() values are invalidated thus loop iteration cannot rely on
	// Element.Next(); instead it relies on List.Front.

	for grp.runners.Len() > 0 { // Iterate until all runners have been removed
		e := <-grp.completed // Wait for completion
		rnr := e.Value.(*runner)
		rnr.canClose = true // Mark as eligible for closing by contiguous scanning

		// If OrderRunners(false) then closing and printing occurs as soon as a
		// runner completes, otherwise it remains a candidate and the runners list
		// is scanned from the front to close the first contiguous sequence of
		// eligible runners. The contiguous scan handles OrderRunners(true) when
		// runners complete in a different order from their creation order - which
		// one would expect to occur quite a lot.

		if !grp.orderRunners { // If any order of completion is ok,
			grp.closePrintRemove(e) // then close now
		} else {
			grp.closePrintRemoveContiguousFront() // Otherwise only eligible contigs
		}

		// Can the potentially new front RunFunc switch to foreground?

		if grp.runners.Len() > 0 && grp.foregroundAllowed() {
			e = grp.runners.Front()
			rnr := e.Value.(*runner)
			rnr.switchToForeground() // Switch if not already foreground
		}
	}
}

// Close and print all runners at the front of the list which have canClose set. This
// function is needed because it's entirely possible for a runner not at the front of the
// list to finish first. If OrderedRunners(true) then the output of that runner must be
// deferred until they are at the front.
//
// This function potentially removes runners by way of closePrintRemove() so the
// encapsulating for loop cannot rely on Element.Next()
func (grp *Group) closePrintRemoveContiguousFront() {
	nextE := grp.runners.Front()
	for e := nextE; e != nil; e = nextE {
		nextE = e.Next()
		rnr := e.Value.(*runner)
		if rnr.canClose != true { // Stop at first incomplete
			return
		}
		grp.closePrintRemove(e)
	}
}

// Finish up a runner that is ready to be printed, including any separators. Remove the
// runner from the Group list. Caller must be aware that *list.Element.Next() is invalid
// on return.
func (grp *Group) closePrintRemove(e *list.Element) {
	rnr := e.Value.(*runner)
	grp.runners.Remove(e)
	rnr.close()

	// Close and flush all writers
	if grp.runners.Len() > 0 { // If not the last runner, consider separators
		if len(grp.outSep) > 0 {
			grp.stdout.Write([]byte(grp.outSep))
		}
		if len(grp.errSep) > 0 {
			grp.stderr.Write([]byte(grp.errSep))
		}
	}
}

// Check that the Group is in the expected state for the caller. If not, panic.
func (grp *Group) checkState(expect groupState) {
	if grp.state == expect {
		return
	}
	panic("parallel.Group has wrong state:" +
		" Expect=" + expect.String() + " have " + grp.state.String())
}
