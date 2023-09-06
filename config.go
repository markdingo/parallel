package parallel

import (
	"errors"
	"io"
	"os"
)

// Config options are set for the NewGroup constructor. Because options are somewhat
// complex and likely to evolve over time, they are implemented as "functional
// options". This means that Option functions are passed as a list of parameters to
// NewGroup(), eg:
//
//	 group := parallel.NewGroup(WithStdoutSeparator("----\n"),
//		   OrderStderr(true), OrderRunners(true))
//
// For a really good exposition on functional options, check out Julien Cretel's GopherCon
// Europe 2023 presentation: https://www.youtube.com/watch?v=5uM6z7RnReE&t=1s
//
// To get the default config settings, the caller should use the newConfig constructor.
type config struct {
	stdout       io.Writer // Parent destination of all stdout
	stderr       io.Writer // Parent destination of all stderr
	outSep       []byte    // Printed to stdout between runners
	errSep       []byte    // Printed to stderr between runners (after outSep)
	limitMemory  uint64    // Maximum bytes buffered before stalling a background runner
	limitRunners uint      // Maximum concurrent runners allowed to run
	orderRunners bool      // All output is written in runner creation order
	orderStderr  bool      // For each runner, all stdout precedes all stderr
	passthru     bool      // Debug option: output is written as soon as it's seen
}

// The default config is one which makes the output appear as it would as if runners were
// run serially. This differs from GNU parallel which adopts a somewhat odd set of
// defaults which results in stderr coming last. I'm sure there are reasons, possibly
// historical, but I don't know what they are.
//
// For those wanting to mimic the defaults for GNU parallel, consider newGNUConfig.
func newConfig() *config {
	return &config{stdout: os.Stdout, stderr: os.Stderr,
		orderRunners: true}
}

// newGNUConfig creates a config which mimics the defaults of the GNU parallel
// command. Namely: --group and --keeporder. This function exists for pedagogic purposes
// only.
func newGNUConfig() *config {
	return &config{stdout: os.Stdout, stderr: os.Stderr,
		orderRunners: false, orderStderr: true}
}

// foregroundAllowed returns true if config allows runners to switch to foreground mode.
func (cfg *config) foregroundAllowed() bool {
	return cfg.orderRunners && !cfg.orderStderr && !cfg.passthru
}

// Option functions configure a [Group] when created with [NewGroup]. Each Option is
// documented separately. Some options cannot be mixed with others, primarily because that
// configuration could cause a [RunFunc] to stall forever. These limitations are described
// in each Option.
type Option interface {
	apply(cfg *config) error
}

type option func(*config) error

func (o option) apply(cfg *config) error {
	return o(cfg)
}

// LimitActiveRunners limits the number of “active” (or concurrent) RunFuncs running in a
// separate goroutine within a [Group] to the “max” value. It can be used in conjunction
// with [LimitMemoryPerRunner] to limit total buffer memory used by the [Group], or set
// independently when there is a risk that too many RunFuncs could be started
// concurrently. If set to zero, all RunFuncs run concurrently.
//
// A [RunFunc] is considered “active” until it returns, not when the output is sent to the
// Group io.Writers.
//
// While tens of thousands of goroutines can run concurrently on most systems, if they all
// contend for limited resources such as CPU, file descriptors, sockets or disk bandwidth,
// then constraining concurrency with this option is likely to improve aggregate system
// throughput or prevent a program from failing due to depleted system resources.
//
// It's sensible to set LimitActiveRunners such that in the general case a command-line
// program processes all arguments in parallel, but in the extreme case, the program still
// progresses towards completion without contending too much for limited resources. For
// example, if the RunFuncs rely on external network connections, perhaps a limit of 10-50
// might be a good setting. Conversely, if the RunFuncs are CPU-intensive, setting
// LimitActiveRunners to some proportion of [runtime.NumCPU] is likely a good strategy.
//
// LimitActiveRunners is intimately connected to [LimitMemoryPerRunner] in that the two
// combine to define the maximum memory resources any Group consumes while buffering
// output.
func LimitActiveRunners(maxActive uint) Option {
	f := func(cfg *config) error {
		cfg.limitRunners = maxActive

		return nil // No error possible
	}

	return option(f)
}

// LimitMemoryPerRunner limits the number of output bytes buffered for each [RunFunc] before
// being stalled on their Write() call. This setting is mostly of use when RunFuncs may
// generate multiple MBytes of output, otherwise the benefits are likely to be minimal.
//
// If LimitMemoryPerRunner is set, so too must [LimitActiveRunners], otherwise the
// total amount of memory consumed is unbounded.
//
// If you know your [RunFunc] only every generates minimal output then leaving this value
// at the default of zero is a reasonable choice. However, if your [RunFunc] potentially
// generates very large amounts of output which may exhaust available memory, then setting
// LimitMemoryPerRunner and [LimitActiveRunners] to non-zero values is a good strategy.
//
// This limit only affects background RunFuncs as the foreground [RunFunc] writes directly
// to the Group output io.Writers. Ultimately all background RunFuncs switched to
// foreground mode so reaching this limit only ever temporarily stalls a [RunFunc].
//
// LimitMemoryPerRunner cannot be set with [OrderStderr] == true or [OrderRunners] ==
// false as that could cause a [RunFunc] to stall indefinitely.
func LimitMemoryPerRunner(limit uint64) Option {
	f := func(cfg *config) error {
		cfg.limitMemory = limit

		return nil // No error possible
	}

	return option(f)
}

// OrderRunners causes output to being written in strict order of [RunFunc] addition to
// the [Group]. If set false output is in order of runner completion. This option exists
// to mimic the GNU parallel “--keep-order” option. The default is true (which differs
// from the default for “--keep-order”).
//
// When OrderRunners is set false, [LimitMemoryPerRunner] cannot be set non-zero as it
// creates a situation where all RunFuncs could stall indefinitely.
func OrderRunners(setting bool) Option {
	f := func(cfg *config) error {
		cfg.orderRunners = setting

		return nil // No error possible
	}

	return option(f)
}

// OrderStderr causes all stderr output to be written *after* all stdout output for each
// [RunFunc]. This can result in an output stream which differs from one written by a
// [RunFunc] run serially and writing directly to os.Stdout and os.Stderr. This option
// exists to mimic the GNU parallel “--group” option.
//
// When OrderStderr is set true, [LimitMemoryPerRunner] cannot be set true as it creates
// a situation when all runners could stall indefinitely.
func OrderStderr(setting bool) Option {
	f := func(cfg *config) error {
		cfg.orderStderr = setting

		return nil // No error possible
	}

	return option(f)
}

// Passthru is a debug setting. When set true all output is transferred more or less
// directly to the Group io.Writers. In effect, the Group pipeline plays a very limited
// part in managing the output stream.
//
// If this option is set true the following options cannot be set true:
// LimitMemoryPerRunner, OrderStderr and OrderRunners.
func Passthru(setting bool) Option {
	f := func(cfg *config) error {
		cfg.passthru = setting

		return nil // No error possible
	}

	return option(f)
}

// WithStderr sets the [Group] stderr destination to the supplied io.Writer replacing the
// default of [os.Stderr].
func WithStderr(wtr io.Writer) Option {
	f := func(cfg *config) error {
		cfg.stderr = wtr

		return nil // No error possible
	}

	return option(f)
}

// WithStderrSeparator sets the separator string printed to the [Group] stderr io.Writer
// between the output of each [RunFunc]. If [WithStdoutSeparator] is also set, that string
// is printed first. If WithStderrSeparator is set to a non-empty string it should
// normally include a trailing newline. The default is an empty string.
func WithStderrSeparator(sep string) Option {
	f := func(cfg *config) error {
		cfg.errSep = []byte(sep)

		return nil // No error possible
	}

	return option(f)
}

// WithStdout sets the [Group] stdout destination to the supplied io.Writer replacing the
// default of os.Stdout.
func WithStdout(wtr io.Writer) Option {
	f := func(cfg *config) error {
		cfg.stdout = wtr

		return nil // No error possible
	}

	return option(f)

}

// WithStdoutSeparator sets the separator string printed to the [Group] stdout io.Writer
// between the output of [RunFunc]. If WithStdoutSeparator is set to a non-empty string it
// should normally include a trailing newline. The default is an empty string.
func WithStdoutSeparator(sep string) Option {
	f := func(cfg *config) error {
		cfg.outSep = []byte(sep)

		return nil // No error possible
	}

	return option(f)
}

// Check that none of the config options conflict with each other and that none of them
// could cause a runner to stall indefinitely.
func (cfg *config) checkConflicts() error {
	if cfg.limitMemory > 0 {
		if cfg.limitRunners == 0 {
			return errors.New("Must set LimitActiveRunners when LimitMemoryPerRunner is set")
		}
		if !cfg.orderRunners {
			return errors.New("Cannot set LimitMemoryPerRunner with OrderRunners(false)")
		}
		if cfg.orderStderr {
			return errors.New("Cannot set LimitMemoryPerRunner with OrderStderr(true)")
		}
	}

	if cfg.passthru {
		if cfg.limitMemory > 0 {
			return errors.New("Cannot set LimitMemoryPerRunner with Passthru(true)")
		}
		if cfg.orderRunners {
			return errors.New("Cannot set OrderRunners with Passthru(true)")
		}
		if cfg.orderStderr {
			return errors.New("Cannot set OrderStderr with Passthru(true)")
		}
	}

	return nil
}
