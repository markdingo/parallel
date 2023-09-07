package parallel

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestConfigSet(t *testing.T) {
	cfg := &config{}

	out := &bytes.Buffer{}
	WithStdout(out).apply(cfg)
	if cfg.stdout != out {
		t.Error("stdout io.Writer not set")
	}

	err := &bytes.Buffer{}
	WithStderr(err).apply(cfg)
	if cfg.stderr != err {
		t.Error("stderr io.Writer not set")
	}

	sep := "====\n"
	WithStdoutSeparator(sep).apply(cfg)
	if string(cfg.outSep) != sep {
		t.Error("Separator not set", string(cfg.outSep), sep)
	}
	WithStderrSeparator(sep).apply(cfg)
	if string(cfg.errSep) != sep {
		t.Error("Separator not set", string(cfg.errSep), sep)
	}

	LimitMemoryPerRunner(1024).apply(cfg)
	if cfg.limitMemory != 1024 {
		t.Error("limitMemory not set", cfg.limitMemory)
	}

	LimitActiveRunners(15).apply(cfg)
	if cfg.limitRunners != 15 {
		t.Error("limitRunners not set", cfg.limitRunners)
	}

	OrderStderr(true).apply(cfg)
	if cfg.orderStderr != true {
		t.Error("orderStderr not set")
	}

	OrderRunners(true).apply(cfg)
	if cfg.orderRunners != true {
		t.Error("orderRunners not set")
	}

	Passthru(true).apply(cfg)
	if cfg.passthru != true {
		t.Error("passthru not set")
	}
}

func TestConfigForeground(t *testing.T) {
	grp, err := NewGroup()
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	if !grp.foregroundAllowed() {
		t.Error("foregroundAllowed should be the default setting of NewGroup()")
	}

	grp, err = NewGroup(OrderStderr(true))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	if grp.foregroundAllowed() {
		t.Error("foregroundAllowed should be false with OrderStderr(true)")
	}
	grp, err = NewGroup(OrderRunners(false))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	if grp.foregroundAllowed() {
		t.Error("foregroundAllowed should be false with OrderRunners(false)")
	}
}

func TestConfigValidCombos(t *testing.T) {
	grp, err := NewGroup()
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	grp, err = NewGroup(LimitMemoryPerRunner(100), LimitActiveRunners(1))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	if grp.limitMemory != 100 {
		t.Error("Default Group should allow a memory quota")
	}

	grp, err = NewGroup(LimitActiveRunners(10))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	if grp.limitRunners != 10 {
		t.Error("LimitActiveRunners(10) did not stick", grp.limitRunners)
	}
}

// Test illegal config combinations
func TestConfigConflicts(t *testing.T) {
	type testCase struct {
		limitMemory  uint64
		limitRunners uint
		orderRunners bool
		orderStderr  bool
		passthru     bool
		error        string
	}

	testCases := []testCase{
		/* 0 */ {0, 0, false, false, false, ""},
		/* 1 */ {100, 0, false, false, false, "Must set LimitActiveRunners"},
		/* 2 */ {100, 1, false, false, false, "LimitMemoryPerRunner with OrderRunners"},
		/* 3 */ {100, 1, true, false, false, ""},
		/* 4 */ {100, 1, true, true, false, "LimitMemoryPerRunner with OrderStderr"},
		/* 5 */ {100, 1, false, true, false, "LimitMemoryPerRunner with OrderRunners"},
		/* 6 */ {100, 1, true, false, true, "LimitMemoryPerRunner with Passthru"},
		/* 7 */ {100, 1, true, false, true, "LimitMemoryPerRunner with Passthru"},
		/* 8 */ {0, 0, true, false, true, "OrderRunners with Passthru"},
		/* 9 */ {0, 0, false, true, true, "OrderStderr with Passthru"},
	}

	for ix, tc := range testCases {
		_, err := NewGroup(LimitMemoryPerRunner(tc.limitMemory),
			LimitActiveRunners(tc.limitRunners),
			OrderRunners(tc.orderRunners),
			OrderStderr(tc.orderStderr),
			Passthru(tc.passthru))
		if err == nil {
			if len(tc.error) != 0 {
				t.Errorf("Case %d: Missing error. Expected '%s'", ix, tc.error)
			}
			continue
		}

		if len(tc.error) == 0 {
			t.Errorf("Case %d: Unexpected error '%s'", ix, err.Error())
			continue
		}

		if !strings.Contains(err.Error(), tc.error) {
			t.Errorf("Case %d: Wrong error. Expected '%s', got '%s'",
				ix, tc.error, err.Error())
		}
	}
}

func TestConfigNilWriters(t *testing.T) {
	cfg := &config{}
	err := WithStdout(os.Stdout).apply(cfg)
	if err != nil {
		t.Error("Unexpected error setting WithStdout", err)
	}
	err = WithStderr(os.Stderr).apply(cfg)
	if err != nil {
		t.Error("Unexpected error setting WithStderr", err)
	}

	err = WithStdout(nil).apply(cfg)
	if err == nil {
		t.Error("Expected error setting WithStdout(nil)")
	}
	err = WithStderr(nil).apply(cfg)
	if err == nil {
		t.Error("Expected error setting WithStderr(nil)", err)
	}
}
