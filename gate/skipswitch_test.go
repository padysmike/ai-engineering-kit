package main

import (
	"slices"
	"strings"
	"testing"
)

const skipGateTestsVariable = "SKIP_GATE_TESTS"

// The Makefile switch is for local runs. If it reached the gate's checks, an
// exported variable on the host would silently weaken them.
func TestGateChecks_SkipSwitchSetOnTheHost_NotPassedOn(t *testing.T) {
	t.Setenv(skipGateTestsVariable, "1")
	t.Setenv("MAKEFLAGS", skipGateTestsVariable+"=1")
	probe := checkRun{dir: t.TempDir(), command: []string{"sh", "-c", `printf '%s|%s' "$SKIP_GATE_TESTS" "$MAKEFLAGS"`}}

	onHost, err := runCheck(sandbox{}, probe)
	inContainer := sandbox{image: "checks"}.dockerArgs(probe)

	if err != nil || onHost != "|" {
		t.Errorf("host checks see %q, err = %v, want nothing", onHost, err)
	}
	if slices.ContainsFunc(inContainer, func(arg string) bool { return strings.Contains(arg, skipGateTestsVariable+"=") }) {
		t.Errorf("container arguments pass the switch on: %v", inContainer)
	}
}
