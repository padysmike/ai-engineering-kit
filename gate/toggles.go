package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	checkApprovedTestsPass = "approved-tests-pass"
	togglesFileName        = "disabled-checks.json"
	skippedLinePrefix      = "  [skip] "
	skippedReason          = " (disabled on the host)"
)

func switchableChecks() []string {
	return append(splitList(defaultTargets), checkApprovedTestsPass)
}

type toggleStore struct {
	dir string
}

func (s toggleStore) path() string {
	return filepath.Join(s.dir, togglesFileName)
}

func (s toggleStore) disabled() ([]string, error) {
	raw, err := os.ReadFile(s.path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the disabled checks: %w", err)
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.path(), err)
	}
	for _, name := range names {
		if err := checkSwitchable(name); err != nil {
			return nil, fmt.Errorf("reading %s: %w", s.path(), err)
		}
	}
	return names, nil
}

func (s toggleStore) save(names []string) error {
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	temporary := s.path() + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.path())
}

func checkSwitchable(name string) error {
	if slices.Contains(switchableChecks(), name) {
		return nil
	}
	return fmt.Errorf("unknown check %q; choose one of: %s", name, strings.Join(switchableChecks(), ", "))
}

func (o options) withDisabledChecks() (options, error) {
	if o.approvals == nil {
		return o, nil
	}
	var err error
	o.disabled, err = toggleStore{dir: o.approvals.dir}.disabled()
	return o, err
}

func (o options) isDisabled(check string) bool {
	return slices.Contains(o.disabled, check)
}

func skippedResult(check, label string) result {
	return result{name: label, ok: true, skippedCheck: check}
}

// Only the gate's own result lines start with the prefix: the output of a
// failed check is indented further, so a branch cannot add names here.
func skippedChecks(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		if rest, ok := strings.CutPrefix(line, skippedLinePrefix); ok {
			name, _, _ := strings.Cut(rest, " ")
			names = append(names, name)
		}
	}
	return unique(names)
}

func isToggleCommand(name string) bool {
	return name == "checks" || name == "disable" || name == "enable"
}

func runToggles(cmd string, args []string, out io.Writer) int {
	flags := flag.NewFlagSet("gate "+cmd, flag.ContinueOnError)
	flags.SetOutput(out)
	state := flags.String("state", defaultStateDir(), "the gate's state directory")
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	store := toggleStore{dir: *state}
	if err := applyToggle(store, cmd, flags.Args()); err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	return listChecks(store, out)
}

func applyToggle(store toggleStore, cmd string, names []string) error {
	if cmd == "checks" {
		if len(names) != 0 {
			return errors.New("usage: gate checks [--state DIR]")
		}
		return nil
	}
	if len(names) != 1 {
		return fmt.Errorf("usage: gate %s [--state DIR] <check>", cmd)
	}
	if err := checkSwitchable(names[0]); err != nil {
		return err
	}
	disabled, err := store.disabled()
	if err != nil {
		return err
	}
	disabled = slices.DeleteFunc(disabled, func(name string) bool { return name == names[0] })
	if cmd == "disable" {
		disabled = append(disabled, names[0])
	}
	return store.save(disabled)
}

func listChecks(store toggleStore, out io.Writer) int {
	disabled, err := store.disabled()
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	say(out, "checks (settings in %s):\n", store.path())
	for _, name := range switchableChecks() {
		state := "on"
		if slices.Contains(disabled, name) {
			state = "off"
		}
		say(out, "  %-3s  %s\n", state, name)
	}
	return exitPassed
}
