package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	kindTests     = "tests"
	kindProtected = "protected"

	statePending  = "pending"
	stateApproved = "approved"
	stateDenied   = "denied"

	stateDirEnv     = "GATE_STATE_DIR"
	stateDirName    = ".gate"
	approvalsFolder = "approvals"
)

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

type approvalNeed struct {
	kind  string
	files []string
}

type approvalRecord struct {
	Branch      string    `json:"branch"`
	SHA         string    `json:"sha"`
	PR          int       `json:"pr,omitempty"`
	Kinds       []string  `json:"kinds"`
	Files       []string  `json:"files"`
	State       string    `json:"state"`
	Reason      string    `json:"reason,omitempty"`
	RequestedAt time.Time `json:"requestedAt"`
}

type approvalStore struct {
	dir string
}

func defaultStateDir() string {
	if dir := os.Getenv(stateDirEnv); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return stateDirName
	}
	return filepath.Join(home, stateDirName)
}

func (s approvalStore) path(branch, sha string) string {
	name := unsafeFileChars.ReplaceAllString(branch, "_") + "@" + sha + ".json"
	return filepath.Join(s.dir, approvalsFolder, name)
}

func (s approvalStore) lookup(branch, sha string) (approvalRecord, bool) {
	raw, err := os.ReadFile(s.path(branch, sha))
	if err != nil {
		return approvalRecord{}, false
	}
	var rec approvalRecord
	if json.Unmarshal(raw, &rec) != nil {
		return approvalRecord{}, false
	}
	return rec, true
}

func (s approvalStore) save(rec approvalRecord) error {
	path := s.path(rec.Branch, rec.SHA)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func (s approvalStore) list() ([]approvalRecord, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, approvalsFolder))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []approvalRecord
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(s.dir, approvalsFolder, entry.Name()))
		if err != nil {
			continue
		}
		var rec approvalRecord
		if json.Unmarshal(raw, &rec) == nil {
			records = append(records, rec)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].RequestedAt.Before(records[j].RequestedAt) })
	return records, nil
}

func (s approvalStore) pendingFor(ref string) (approvalRecord, error) {
	records, err := s.list()
	if err != nil {
		return approvalRecord{}, err
	}
	var match *approvalRecord
	for i := range records {
		rec := records[i]
		if rec.State == statePending && refersTo(rec, ref) {
			match = &records[i]
		}
	}
	if match == nil {
		return approvalRecord{}, fmt.Errorf("no pending approval for %q", ref)
	}
	return *match, nil
}

func refersTo(rec approvalRecord, ref string) bool {
	if n, err := strconv.Atoi(ref); err == nil {
		return rec.PR == n
	}
	return rec.Branch == ref
}

func newApprovalRecord(o options, sha string, needs []approvalNeed) approvalRecord {
	rec := approvalRecord{Branch: o.name(), SHA: sha, PR: o.prNumber, State: statePending, RequestedAt: time.Now().UTC()}
	for _, n := range needs {
		rec.Kinds = append(rec.Kinds, n.kind)
		rec.Files = append(rec.Files, n.files...)
	}
	rec.Files = unique(rec.Files)
	return rec
}

func (r approvalRecord) covers(kind string) bool {
	for _, k := range r.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (r approvalRecord) describe() string {
	pr := ""
	if r.PR != 0 {
		pr = fmt.Sprintf(" (PR #%d)", r.PR)
	}
	return fmt.Sprintf("%s%s @%s %s: %s [%s]", r.Branch, pr, short(r.SHA), r.State, strings.Join(r.Files, ", "), strings.Join(r.Kinds, ", "))
}

func applyApproval(o options, sha string) options {
	if o.approvals == nil {
		return o
	}
	rec, found := o.approvals.lookup(o.name(), sha)
	if !found || rec.State != stateApproved {
		return o
	}
	o.allowTestChanges = o.allowTestChanges || rec.covers(kindTests)
	o.allowProtected = o.allowProtected || rec.covers(kindProtected)
	return o
}

func approvalNeeds(results []result) ([]approvalNeed, bool) {
	var needs []approvalNeed
	for _, r := range results {
		if r.ok {
			continue
		}
		if r.approval == nil {
			return nil, false
		}
		needs = append(needs, *r.approval)
	}
	return needs, len(needs) > 0
}

func requestApproval(o options, oc outcome) outcome {
	if o.approvals == nil {
		return oc
	}
	needs, ok := approvalNeeds(oc.results)
	if !ok {
		return oc
	}
	rec, found := o.approvals.lookup(o.name(), oc.sha)
	switch {
	case found && rec.State == stateDenied:
		oc.results = annotateFailure(oc.results, "approval denied: "+rec.Reason)
	case found && rec.State == statePending:
		oc.pending = true
	case found:
		return oc
	default:
		oc = createRequest(o, oc, needs)
	}
	return oc
}

func createRequest(o options, oc outcome, needs []approvalNeed) outcome {
	rec := newApprovalRecord(o, oc.sha, needs)
	if err := o.approvals.save(rec); err != nil {
		oc.results = annotateFailure(oc.results, "could not record the approval request: "+err.Error())
		return oc
	}
	oc.pending = true
	if o.notify != nil {
		say(o.notify, "approval requested: %s\n  approve with: gate approve %s\n", rec.describe(), o.name())
	}
	return oc
}

func annotateFailure(results []result, line string) []result {
	for i := range results {
		if !results[i].ok {
			results[i].detail = append(results[i].detail, line)
			return results
		}
	}
	return results
}

func runApprovals(cmd string, args []string, out io.Writer) int {
	fs := flag.NewFlagSet("gate "+cmd, flag.ContinueOnError)
	fs.SetOutput(out)
	state := fs.String("state", defaultStateDir(), "directory that holds approval requests")
	reason := fs.String("reason", "", "reason shown to the agent when denying")
	refs, err := parseInAnyOrder(fs, args)
	if err != nil {
		return exitError
	}
	if err := checkApprovalArguments(cmd, refs, *reason); err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	store := approvalStore{dir: *state}
	ref := strings.Join(refs, "")
	switch cmd {
	case "approvals":
		return listApprovals(store, out)
	case "approve":
		return decideApproval(store, ref, stateApproved, "", out)
	default:
		return decideApproval(store, ref, stateDenied, *reason, out)
	}
}

// parseInAnyOrder also reads the flags written after an argument. The flag
// package stops at the first argument and would silently drop them.
func parseInAnyOrder(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		if flags.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, flags.Arg(0))
		args = flags.Args()[1:]
	}
}

func checkApprovalArguments(cmd string, refs []string, reason string) error {
	if cmd == "approvals" && len(refs) > 0 {
		return fmt.Errorf("approvals takes no branch or pull request, got %q", refs)
	}
	if len(refs) > 1 {
		return fmt.Errorf("name one branch or pull request number, got %q; put a reason with spaces in quotes", refs)
	}
	if cmd != "deny" && reason != "" {
		return fmt.Errorf("--reason only applies to deny")
	}
	return nil
}

func listApprovals(store approvalStore, out io.Writer) int {
	records, err := store.list()
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	if len(records) == 0 {
		say(out, "no approval requests\n")
		return exitPassed
	}
	for _, rec := range records {
		say(out, "%s\n", rec.describe())
	}
	return exitPassed
}

func decideApproval(store approvalStore, ref, state, reason string, out io.Writer) int {
	if ref == "" {
		say(out, "gate: a branch name or pull request number is required\n")
		return exitError
	}
	rec, err := store.pendingFor(ref)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	rec.State, rec.Reason = state, reason
	if err := store.save(rec); err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	say(out, "%s\n", rec.describe())
	return exitPassed
}
