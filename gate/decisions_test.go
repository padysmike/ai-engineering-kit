package main

import (
	"strings"
	"testing"
	"time"
)

func storeWithPendingRequest(t *testing.T) approvalStore {
	t.Helper()
	store := approvalStore{dir: t.TempDir()}
	pending := approvalRecord{Branch: "ticket-7", SHA: "abc", PR: 7, Kinds: []string{kindTests}, State: statePending, RequestedAt: time.Now()}
	if err := store.save(pending); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRunApprovals_DenyInAnyOrder_ReasonKept(t *testing.T) {
	const reason = "keep the old wording"
	for name, args := range map[string]func(state string) []string{
		"WHEN the reason follows the branch THEN it is kept":  func(s string) []string { return []string{"--state", s, "ticket-7", "--reason", reason} },
		"WHEN the reason precedes the branch THEN it is kept": func(s string) []string { return []string{"--state", s, "--reason", reason, "ticket-7"} },
		"WHEN every flag follows the number THEN all apply":   func(s string) []string { return []string{"7", "--reason=" + reason, "--state", s} },
	} {
		t.Run(name, func(t *testing.T) {
			store := storeWithPendingRequest(t)
			var out strings.Builder

			code := runApprovals("deny", args(store.dir), &out)

			got, found := store.lookup("ticket-7", "abc")
			if code != exitPassed || !found || got.State != stateDenied || got.Reason != reason {
				t.Errorf("exit %d, record %+v\n%s", code, got, out.String())
			}
		})
	}
}

func TestRunApprovals_UnclearArguments_RefusedAndNothingDecided(t *testing.T) {
	tests := []struct {
		name, cmd, want string
		args            []string
	}{
		{"WHEN the reason is not quoted THEN it is refused", "deny", "in quotes", []string{"ticket-7", "--reason", "keep", "the", "wording"}},
		{"WHEN approve gets a reason THEN it is refused", "approve", "only applies to deny", []string{"ticket-7", "--reason", "fine"}},
		{"WHEN approvals gets a branch THEN it is refused", "approvals", "takes no branch", []string{"ticket-7"}},
		{"WHEN a flag after the branch is unknown THEN it is refused", "deny", "flag provided but not defined", []string{"ticket-7", "--why", "x"}},
		{"WHEN the reason has no value THEN it is refused", "deny", "flag needs an argument", []string{"ticket-7", "--reason"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := storeWithPendingRequest(t)
			var out strings.Builder

			code := runApprovals(tc.cmd, append([]string{"--state", store.dir}, tc.args...), &out)

			got, _ := store.lookup("ticket-7", "abc")
			if code != exitError || got.State != statePending || !strings.Contains(out.String(), tc.want) {
				t.Errorf("exit %d, state %q, want %q in:\n%s", code, got.State, tc.want, out.String())
			}
		})
	}
}
