package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	tokenEnv         = "GATE_GITHUB_TOKEN"
	statusContext    = "gate"
	defaultAPIURL    = "https://api.github.com"
	maxStatusMessage = 140
	requestTimeout   = 30 * time.Second
	maxResponseBytes = 1 << 20
)

type githubAPI struct {
	owner  string
	repo   string
	token  string
	apiURL string
	client *http.Client
}

type pullRequest struct {
	number  int
	headSHA string
	baseRef string
}

type prOptions struct {
	repoDir string
	remote  string
	base    string
	branch  string
	targets string
	merge   bool
	gh      githubAPI

	allowTestChanges bool
	allowProtected   bool
	approvals        *approvalStore
	notify           io.Writer
	sandbox          sandbox
}

func newGithubAPI(slug, token string) (githubAPI, error) {
	owner, repo, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return githubAPI{}, fmt.Errorf("--github must look like owner/repo, got %q", slug)
	}
	if token == "" {
		return githubAPI{}, fmt.Errorf("%s is not set", tokenEnv)
	}
	return githubAPI{
		owner:  owner,
		repo:   repo,
		token:  token,
		apiURL: defaultAPIURL,
		client: &http.Client{Timeout: requestTimeout},
	}, nil
}

func (g githubAPI) do(method, path string, body, out any) error {
	req, err := g.newRequest(method, path, body)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiError(method, path, resp.Status, raw)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (g githubAPI) newRequest(method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, g.apiURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "ai-engineering-kit-gate")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func apiError(method, path, status string, raw []byte) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &payload)
	return fmt.Errorf("github %s %s: %s: %s", method, path, status, payload.Message)
}

func (g githubAPI) openPullRequest(branch string) (pullRequest, error) {
	query := url.Values{"state": {"open"}, "head": {g.owner + ":" + branch}}
	var found []struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls?%s", g.owner, g.repo, query.Encode())
	if err := g.do(http.MethodGet, path, nil, &found); err != nil {
		return pullRequest{}, err
	}
	if len(found) == 0 {
		return pullRequest{}, fmt.Errorf("no open pull request for branch %s", branch)
	}
	return pullRequest{number: found[0].Number, headSHA: found[0].Head.SHA, baseRef: found[0].Base.Ref}, nil
}

func (g githubAPI) postStatus(sha, state, description string) error {
	if len(description) > maxStatusMessage {
		description = description[:maxStatusMessage]
	}
	path := fmt.Sprintf("/repos/%s/%s/statuses/%s", g.owner, g.repo, sha)
	return g.do(http.MethodPost, path, map[string]string{
		"state":       state,
		"context":     statusContext,
		"description": description,
	}, nil)
}

func (g githubAPI) mergePullRequest(number int, sha string) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", g.owner, g.repo, number)
	return g.do(http.MethodPut, path, map[string]string{"sha": sha, "merge_method": "merge"}, nil)
}

func runPullRequest(o prOptions, out io.Writer) int {
	if !branchName.MatchString(o.branch) {
		say(out, "invalid branch name\n")
		return exitError
	}
	sha, pr, err := fetchPullRequest(o)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	if pr.baseRef != o.base {
		say(out, "gate: pull request #%d targets %q, but this gate guards %q; nothing was checked\n", pr.number, pr.baseRef, o.base)
		return exitBlocked
	}
	if pr.headSHA != sha {
		say(out, "gate: pull request head %s differs from the fetched branch %s; run the gate again\n", short(pr.headSHA), short(sha))
		return exitBlocked
	}

	gateOpts, err := o.gateOptions(pr.number)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	var buf bytes.Buffer
	code := execute(gateOpts, &buf)
	say(out, "%s", buf.String())

	state, description := statusFor(code, buf.String())
	if err := o.gh.postStatus(sha, state, description); err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	say(out, "status %q posted on %s: %s\n", statusContext, short(sha), state)
	if code != exitPassed || !o.merge {
		return code
	}
	return mergeAndReport(o.gh, pr.number, sha, out)
}

func (o prOptions) gateOptions(prNumber int) (options, error) {
	repo, err := filepath.Abs(o.repoDir)
	if err != nil {
		return options{}, err
	}
	return options{
		repo:             repo,
		base:             o.remote + "/" + o.base,
		branch:           o.remote + "/" + o.branch,
		label:            o.branch,
		targets:          splitList(o.targets),
		protected:        splitList(defaultProtected),
		allowTestChanges: o.allowTestChanges,
		allowProtected:   o.allowProtected,
		prNumber:         prNumber,
		approvals:        o.approvals,
		notify:           o.notify,
		sandbox:          o.sandbox,
	}, nil
}

func fetchPullRequest(o prOptions) (string, pullRequest, error) {
	gitMu.Lock()
	defer gitMu.Unlock()
	if _, err := git(o.repoDir, "fetch", o.remote, o.base, o.branch); err != nil {
		return "", pullRequest{}, err
	}
	sha, err := revParse(o.repoDir, o.remote+"/"+o.branch)
	if err != nil {
		return "", pullRequest{}, err
	}
	pr, err := o.gh.openPullRequest(o.branch)
	return sha, pr, err
}

func statusFor(code int, output string) (string, string) {
	switch code {
	case exitPassed:
		return "success", "gate passed"
	case exitBlocked:
		return "failure", "gate blocked: " + firstFailure(output)
	case exitNeedsApproval:
		return "pending", "waiting for human approval: " + firstFailure(output)
	default:
		return "error", "gate could not run"
	}
}

func mergeAndReport(gh githubAPI, number int, sha string, out io.Writer) int {
	if err := gh.mergePullRequest(number, sha); err != nil {
		say(out, "gate: %v\n", err)
		return exitBlocked
	}
	say(out, "merged pull request #%d\n", number)
	return exitPassed
}

func runPR(args []string, out io.Writer) int {
	o, err := parsePROptions(args, out)
	if err != nil {
		say(out, "gate pr: %v\n", err)
		return exitError
	}
	return runPullRequest(o, out)
}

func parsePROptions(args []string, out io.Writer) (prOptions, error) {
	var o prOptions
	var slug, state string
	var box sandboxFlags
	fs := flag.NewFlagSet("gate pr", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&o.repoDir, "repo", ".", "path to the git checkout")
	fs.StringVar(&o.remote, "remote", "origin", "git remote that points at GitHub")
	fs.StringVar(&o.base, "base", "main", "trusted base branch on the remote")
	fs.StringVar(&o.branch, "branch", "", "pull request head branch (required)")
	fs.StringVar(&o.targets, "targets", defaultTargets, "comma-separated make targets")
	fs.StringVar(&slug, "github", "", "GitHub repository as owner/repo (required)")
	fs.BoolVar(&o.merge, "merge", false, "merge the pull request if every check passes")
	fs.BoolVar(&o.allowTestChanges, "allow-test-changes", false, "a human approved changes to existing tests and scenarios")
	fs.BoolVar(&o.allowProtected, "allow-protected", false, "a human approved changes to protected paths")
	fs.StringVar(&state, "state", "", "directory for human approval requests; enables the approval flow when set")
	box.register(fs)
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.branch == "" {
		return o, fmt.Errorf("--branch is required")
	}
	var err error
	if o.sandbox, err = box.choose(); err != nil {
		return o, err
	}
	if state != "" {
		o.approvals = &approvalStore{dir: state}
		o.notify = os.Stderr
	}
	gh, err := newGithubAPI(slug, os.Getenv(tokenEnv))
	o.gh = gh
	return o, err
}

func firstFailure(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if i := strings.Index(line, "[FAIL]"); i >= 0 {
			return strings.TrimSpace(line[i+len("[FAIL]"):])
		}
	}
	return "see the gate output"
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
