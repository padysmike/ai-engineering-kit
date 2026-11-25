package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultTargets   = "fmt-check,vet,lint,test,bdd,coverage,diff-coverage,validate-skills"
	defaultProtected = "Makefile,scripts/,gate/,skills/,.claude/,.github/,.golangci.yml,AGENTS.md,CLAUDE.md"
	checkTimeout     = 10 * time.Minute
	detailLines      = 6
	trustedMakefile  = "Makefile"
	trustedScripts   = "scripts"

	exitPassed        = 0
	exitBlocked       = 1
	exitError         = 2
	exitNeedsApproval = 3
)

var gitMu sync.Mutex

type options struct {
	repo             string
	base             string
	branch           string
	targets          []string
	protected        []string
	allowTestChanges bool
	allowProtected   bool
	merge            bool
	label            string
	prNumber         int
	approvals        *approvalStore
	notify           io.Writer
	sandbox          sandbox
}

type result struct {
	name     string
	ok       bool
	detail   []string
	approval *approvalNeed
}

type outcome struct {
	results []result
	sha     string
	pending bool
}

func (o options) name() string {
	if o.label != "" {
		return o.label
	}
	return o.branch
}

type change struct {
	status string
	paths  []string
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(runMCP(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "pr" {
		os.Exit(runPR(os.Args[2:], os.Stdout))
	}
	if len(os.Args) > 1 && isApprovalCommand(os.Args[1]) {
		os.Exit(runApprovals(os.Args[1], os.Args[2:], os.Stdout))
	}
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	o, err := parseOptions(args, out)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	return execute(o, out)
}

func execute(o options, out io.Writer) int {
	oc, err := evaluate(o)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	return report(out, o, oc)
}

func isApprovalCommand(name string) bool {
	return name == "approvals" || name == "approve" || name == "deny"
}

func (o *options) resolve(box sandboxFlags, state string) error {
	var err error
	if o.sandbox, err = box.choose(); err != nil {
		return err
	}
	if state != "" {
		o.approvals = &approvalStore{dir: state}
		o.notify = os.Stderr
	}
	o.repo, err = filepath.Abs(o.repo)
	return err
}

func parseOptions(args []string, out io.Writer) (options, error) {
	var o options
	var targets, protected, state string
	var box sandboxFlags
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&o.repo, "repo", ".", "path to the git repository")
	fs.StringVar(&o.base, "base", "main", "trusted base branch")
	fs.StringVar(&o.branch, "branch", "", "branch to check (required)")
	fs.StringVar(&targets, "targets", defaultTargets, "comma-separated make targets, run from the base branch's Makefile")
	fs.StringVar(&protected, "protected", defaultProtected, "comma-separated protected paths (a trailing slash means a directory)")
	fs.BoolVar(&o.allowTestChanges, "allow-test-changes", false, "a human approved changes to existing tests and scenarios")
	fs.BoolVar(&o.allowProtected, "allow-protected", false, "a human approved changes to protected paths")
	fs.BoolVar(&o.merge, "merge", false, "merge the branch into the base branch if every check passes")
	fs.StringVar(&state, "state", "", "directory for human approval requests; enables the approval flow when set")
	box.register(fs)
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.branch == "" {
		return o, fmt.Errorf("--branch is required")
	}
	if err := o.resolve(box, state); err != nil {
		return o, err
	}
	o.targets = splitList(targets)
	o.protected = splitList(protected)
	return o, nil
}

func evaluate(o options) (outcome, error) {
	baseSHA, err := revParse(o.repo, o.base)
	if err != nil {
		return outcome{}, err
	}
	branchSHA, err := revParse(o.repo, o.branch)
	if err != nil {
		return outcome{}, err
	}
	o = applyApproval(o, branchSHA)
	raw, err := git(o.repo, "diff", "--name-status", "-z", "-M", baseSHA+"..."+branchSHA)
	if err != nil {
		return outcome{}, err
	}
	changes := parseDiff(raw)

	results := []result{checkProtected(changes, o), checkImmutable(changes, o)}
	results = append(results, runTargets(o, baseSHA, branchSHA)...)
	oc := requestApproval(o, outcome{results: results, sha: branchSHA})
	if o.merge && allOK(oc.results) {
		oc.results = append(oc.results, doMerge(o, branchSHA))
	}
	return oc, nil
}

func report(out io.Writer, o options, oc outcome) int {
	p := newPainter(out)
	say(out, "gate: %s -> %s\n", o.branch, o.base)
	for _, r := range oc.results {
		printResult(out, p, r)
	}
	switch {
	case allOK(oc.results):
		say(out, "%s\n", p.green("RESULT: PASSED"))
		return exitPassed
	case oc.pending:
		say(out, "%s\n", p.red("RESULT: NEEDS APPROVAL (waiting for a human on the host)"))
		return exitNeedsApproval
	default:
		say(out, "%s\n", p.red("RESULT: BLOCKED"))
		return exitBlocked
	}
}

func printResult(out io.Writer, p painter, r result) {
	if r.ok {
		say(out, "  %s %s\n", p.green("[ok]  "), r.name)
		return
	}
	say(out, "  %s %s\n", p.red("[FAIL]"), r.name)
	for _, line := range r.detail {
		say(out, "           %s\n", line)
	}
}

func checkProtected(changes []change, o options) result {
	var hits []string
	for _, c := range changes {
		for _, path := range c.paths {
			if matchesAny(path, o.protected) {
				hits = append(hits, path)
			}
		}
	}
	hits = unique(hits)
	if len(hits) == 0 || o.allowProtected {
		return result{name: "protected paths untouched", ok: true}
	}
	return result{name: "protected path changed: " + strings.Join(hits, ", "), approval: &approvalNeed{kind: kindProtected, files: hits}}
}

func checkImmutable(changes []change, o options) result {
	var hits []string
	for _, c := range changes {
		if c.status != "A" && isApprovedTest(c.paths[0]) {
			hits = append(hits, c.paths[0])
		}
	}
	hits = unique(hits)
	if len(hits) == 0 || o.allowTestChanges {
		return result{name: "approved tests and scenarios untouched", ok: true}
	}
	return result{name: "approved test or scenario changed: " + strings.Join(hits, ", "), approval: &approvalNeed{kind: kindTests, files: hits}}
}

func runTargets(o options, baseSHA, branchSHA string) []result {
	worktree, cleanup, err := trustedWorktree(o.repo, baseSHA, branchSHA)
	if err != nil {
		return []result{{name: "checks could not run", detail: []string{err.Error()}}}
	}
	defer cleanup()

	run, err := o.sandbox.prepareRun(o.repo, worktree, baseSHA, branchSHA)
	if err != nil {
		return []result{{name: "checks could not run", detail: []string{err.Error()}}}
	}
	var results []result
	for _, target := range o.targets {
		run.target = target
		results = append(results, runTarget(o.sandbox, run))
	}
	return results
}

func trustedWorktree(repo, baseSHA, branchSHA string) (string, func(), error) {
	if _, err := git(repo, "cat-file", "-e", baseSHA+":"+trustedMakefile); err != nil {
		return "", nil, fmt.Errorf("the base branch has no Makefile, so there is nothing trusted to run")
	}
	tmp, err := os.MkdirTemp("", "gate-")
	if err != nil {
		return "", nil, err
	}
	worktree := filepath.Join(tmp, "wt")
	cleanup := func() {
		gitMu.Lock()
		defer gitMu.Unlock()
		_, _ = git(repo, "worktree", "remove", "--force", worktree)
		_ = os.RemoveAll(tmp)
	}
	if err := prepareWorktree(repo, worktree, baseSHA, branchSHA); err != nil {
		cleanup()
		return "", nil, err
	}
	return worktree, cleanup, nil
}

func prepareWorktree(repo, worktree, baseSHA, branchSHA string) error {
	gitMu.Lock()
	defer gitMu.Unlock()
	if _, err := git(repo, "worktree", "add", "--detach", worktree, branchSHA); err != nil {
		return err
	}
	return restoreTrusted(repo, worktree, baseSHA)
}

func restoreTrusted(repo, worktree, baseSHA string) error {
	_ = os.Remove(filepath.Join(worktree, trustedMakefile))
	_ = os.RemoveAll(filepath.Join(worktree, trustedScripts))
	paths := []string{trustedMakefile}
	if _, err := git(repo, "cat-file", "-e", baseSHA+":"+trustedScripts); err == nil {
		paths = append(paths, trustedScripts)
	}
	_, err := git(worktree, append([]string{"checkout", baseSHA, "--"}, paths...)...)
	return err
}

func runTarget(box sandbox, run checkRun) result {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := box.command(ctx, run)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		return result{name: run.target, detail: tail(buf.String(), detailLines)}
	}
	return result{name: run.target, ok: true}
}

func doMerge(o options, branchSHA string) result {
	gitMu.Lock()
	defer gitMu.Unlock()
	if reason := mergeBlocker(o); reason != "" {
		return result{name: "merge", detail: []string{reason}}
	}
	msg := fmt.Sprintf("Merge %s (gate passed)", o.branch)
	if _, err := git(o.repo, "merge", "--no-ff", "-m", msg, branchSHA); err != nil {
		return result{name: "merge", detail: []string{err.Error()}}
	}
	return result{name: "merged " + o.branch + " into " + o.base, ok: true}
}

func mergeBlocker(o options) string {
	head, err := git(o.repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err.Error()
	}
	if current := strings.TrimSpace(head); current != o.base {
		return fmt.Sprintf("check out %s first (currently on %s)", o.base, current)
	}
	status, err := git(o.repo, "status", "--porcelain")
	if err != nil {
		return err.Error()
	}
	if strings.TrimSpace(status) != "" {
		return "the working tree has uncommitted changes"
	}
	return ""
}

func revParse(repo, ref string) (string, error) {
	out, err := git(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("unknown ref %q", ref)
	}
	return strings.TrimSpace(out), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func parseDiff(raw string) []change {
	parts := strings.Split(raw, "\x00")
	var out []change
	for i := 0; i < len(parts); {
		status := parts[i]
		if status == "" {
			i++
			continue
		}
		pathCount := 1
		if status[0] == 'R' || status[0] == 'C' {
			pathCount = 2
		}
		if i+pathCount >= len(parts) {
			break
		}
		out = append(out, change{status: status[:1], paths: parts[i+1 : i+1+pathCount]})
		i += 1 + pathCount
	}
	return out
}

func isApprovedTest(path string) bool {
	return strings.HasPrefix(path, "scenarios/") || strings.HasSuffix(path, "_test.go")
}

func matchesAny(path string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
			continue
		}
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func tail(s string, n int) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func allOK(rs []result) bool {
	for _, r := range rs {
		if !r.ok {
			return false
		}
	}
	return true
}

func say(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }

type painter struct{ on bool }

func newPainter(w io.Writer) painter {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return painter{}
	}
	st, err := f.Stat()
	return painter{on: err == nil && st.Mode()&os.ModeCharDevice != 0}
}

func (p painter) green(s string) string { return p.wrap("32", s) }
func (p painter) red(s string) string   { return p.wrap("1;31", s) }

func (p painter) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
