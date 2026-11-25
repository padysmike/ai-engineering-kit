package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeGitHub struct {
	server     *httptest.Server
	prHead     string
	prBase     string
	mergeCode  int
	statuses   []map[string]string
	statusPath string
	merged     []map[string]string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{mergeCode: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls":
			if f.prHead == "" {
				_, _ = w.Write([]byte("[]"))
				return
			}
			base := f.prBase
			if base == "" {
				base = "main"
			}
			_, _ = w.Write([]byte(`[{"number":7,"head":{"sha":"` + f.prHead + `"},"base":{"ref":"` + base + `"}}]`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/repos/o/r/statuses/"):
			var m map[string]string
			_ = json.Unmarshal(body, &m)
			f.statuses = append(f.statuses, m)
			f.statusPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/o/r/pulls/7/merge":
			var m map[string]string
			_ = json.Unmarshal(body, &m)
			f.merged = append(f.merged, m)
			w.WriteHeader(f.mergeCode)
			_, _ = w.Write([]byte(`{"message":"Head branch was modified"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) api() githubAPI {
	return githubAPI{owner: "o", repo: "r", token: "t", apiURL: f.server.URL, client: f.server.Client()}
}

func repoWithRemote(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	bare := t.TempDir()
	runGit(t, bare, "init", "-q", "--bare", "-b", "main")
	runGit(t, dir, "remote", "add", "origin", bare)
	runGit(t, dir, "push", "-q", "origin", "main")
	return dir
}

func pushBranch(t *testing.T, dir, name string, edit func()) string {
	t.Helper()
	branchWith(t, dir, name, edit)
	runGit(t, dir, "push", "-q", "origin", name)
	return strings.TrimSpace(runGit(t, dir, "rev-parse", name))
}

func prOptionsFor(dir, branch string, merge bool, gh githubAPI) prOptions {
	return prOptions{repoDir: dir, remote: "origin", base: "main", branch: branch, targets: testTargets, merge: merge, gh: gh}
}

func TestPullRequest_CleanBranchWithMerge_StatusSuccessThenMerged(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-1", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-1", true, fake.api()), &out)

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if len(fake.statuses) != 1 || fake.statuses[0]["state"] != "success" || fake.statuses[0]["context"] != "gate" {
		t.Fatalf("statuses: %v", fake.statuses)
	}
	if fake.statusPath != "/repos/o/r/statuses/"+sha {
		t.Errorf("status posted on %s", fake.statusPath)
	}
	if len(fake.merged) != 1 || fake.merged[0]["sha"] != sha {
		t.Fatalf("merge calls: %v", fake.merged)
	}
}

func TestPullRequest_BlockedBranch_StatusFailureAndNoMerge(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-2", func() { write(t, dir, "coverage.txt", "10\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-2", true, fake.api()), &out)

	if code != 1 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if len(fake.statuses) != 1 || fake.statuses[0]["state"] != "failure" {
		t.Fatalf("statuses: %v", fake.statuses)
	}
	if !strings.Contains(fake.statuses[0]["description"], "coverage") {
		t.Errorf("description %q lacks the failing check", fake.statuses[0]["description"])
	}
	if len(fake.merged) != 0 {
		t.Errorf("merge was called: %v", fake.merged)
	}
}

func TestPullRequest_CheckOnly_StatusPostedWithoutMerge(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-3", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-3", false, fake.api()), &out)

	if code != 0 || len(fake.statuses) != 1 || len(fake.merged) != 0 {
		t.Fatalf("exit %d, statuses %v, merged %v\n%s", code, fake.statuses, fake.merged, out.String())
	}
}

func TestPullRequest_NoOpenPullRequest_FailsWithoutStatus(t *testing.T) {
	dir := repoWithRemote(t)
	pushBranch(t, dir, "ticket-4", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-4", true, fake.api()), &out)

	if code != 2 || !strings.Contains(out.String(), "no open pull request") || len(fake.statuses) != 0 {
		t.Fatalf("exit %d, statuses %v\n%s", code, fake.statuses, out.String())
	}
}

func TestPullRequest_HeadDiffersFromFetchedBranch_NothingPosted(t *testing.T) {
	dir := repoWithRemote(t)
	pushBranch(t, dir, "ticket-5", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	fake.prHead = strings.Repeat("a", 40)
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-5", true, fake.api()), &out)

	if code != 1 || len(fake.statuses) != 0 || len(fake.merged) != 0 {
		t.Fatalf("exit %d, statuses %v, merged %v\n%s", code, fake.statuses, fake.merged, out.String())
	}
}

func TestPullRequest_GitHubRejectsMerge_ExitOne(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-6", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	fake.mergeCode = http.StatusConflict
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-6", true, fake.api()), &out)

	if code != 1 || !strings.Contains(out.String(), "409") {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
}

func TestPullRequest_InvalidBranch_Rejected(t *testing.T) {
	dir := repoWithRemote(t)
	fake := newFakeGitHub(t)
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "--upload-pack=x", true, fake.api()), &out)

	if code != 2 || len(fake.statuses) != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
}

func TestNewGithubAPI_BadInput_Rejected(t *testing.T) {
	cases := map[string][2]string{
		"missing slash": {"repo", "t"},
		"empty owner":   {"/repo", "t"},
		"extra segment": {"a/b/c", "t"},
		"no token":      {"a/b", ""},
	}
	for name, c := range cases {
		t.Run("WHEN "+name+" THEN construction fails", func(t *testing.T) {
			if _, err := newGithubAPI(c[0], c[1]); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestFirstFailure_Output_ReturnsFirstFailLine(t *testing.T) {
	output := "  [ok]   vet\n  [FAIL] coverage\n           coverage 50% < 70%\n"

	if got := firstFailure(output); got != "coverage" {
		t.Fatalf("got %q", got)
	}
}

func TestPullRequest_ChangedScenario_BlockedUnlessOperatorApproves(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-7", func() { write(t, dir, "scenarios/health.feature", "Feature: reworded\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	var blockedOut, approvedOut strings.Builder

	blockedCode := runPullRequest(prOptionsFor(dir, "ticket-7", false, fake.api()), &blockedOut)
	approved := prOptionsFor(dir, "ticket-7", false, fake.api())
	approved.allowTestChanges = true
	approvedCode := runPullRequest(approved, &approvedOut)

	if blockedCode != 1 || approvedCode != 0 {
		t.Fatalf("blocked %d, approved %d\n%s\n%s", blockedCode, approvedCode, blockedOut.String(), approvedOut.String())
	}
	if len(fake.statuses) != 2 || fake.statuses[0]["state"] != "failure" || fake.statuses[1]["state"] != "success" {
		t.Fatalf("statuses: %v", fake.statuses)
	}
}

func TestPullRequest_TargetsOtherBranch_NothingCheckedOrPosted(t *testing.T) {
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, "ticket-8", func() { write(t, dir, "app/feature.go", "package main\n") })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	fake.prBase = "release"
	var out strings.Builder

	code := runPullRequest(prOptionsFor(dir, "ticket-8", true, fake.api()), &out)

	if code != exitBlocked || len(fake.statuses) != 0 || len(fake.merged) != 0 || !strings.Contains(out.String(), `targets "release"`) {
		t.Fatalf("exit %d, statuses %v, merged %v\n%s", code, fake.statuses, fake.merged, out.String())
	}
}
