package parallel

import (
	"bytes"
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

func TestConfigAllowed(t *testing.T) {
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
	_, err := NewGroup(LimitMemoryPerRunner(100))
	if err == nil {
		t.Error("Expected LimitActiveRunners error")
	}

	_, err = NewGroup(LimitMemoryPerRunner(100), OrderRunners(false))
	if err == nil {
		t.Error("Expected OrderRunners error")
	}

	_, err = NewGroup(LimitMemoryPerRunner(100), OrderStderr(true))
	if err == nil {
		t.Error("Expected OrderStderr error")
	}

	_, err = NewGroup(Passthru(true), LimitMemoryPerRunner(100))
	if err == nil {
		t.Error("Expected LimitMemoryPerRunner error")
	}

	_, err = NewGroup(Passthru(true), OrderRunners(true))
	if err == nil {
		t.Error("Expected OrderRunners error")
	}

	_, err = NewGroup(Passthru(true), OrderStderr(true))
	if err == nil {
		t.Error("Expected OrderStderr error")
	}
}
