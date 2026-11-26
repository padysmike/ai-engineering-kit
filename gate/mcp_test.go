package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, cfg mcpConfig) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newMCPServer(cfg).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	})
	return clientSession
}

func testConfig(dir string, allowMerge bool) mcpConfig {
	return mcpConfig{repo: dir, base: "main", targets: testTargets, allowMerge: allowMerge}
}

func callTool(t *testing.T, session *mcp.ClientSession, tool, branch string) (string, bool, error) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tool,
		Arguments: map[string]any{"branch": branch},
	})
	if err != nil {
		return "", false, err
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError, nil
}

func TestMCP_ListTools_MergeHiddenUnlessAllowed(t *testing.T) {
	dir := newRepo(t)
	names := func(allowMerge bool) []string {
		res, err := connect(t, testConfig(dir, allowMerge)).ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, tool := range res.Tools {
			out = append(out, tool.Name)
		}
		return out
	}

	without, with := names(false), names(true)

	if strings.Join(without, ",") != "gate_check,gate_result" {
		t.Errorf("without merge: %v", without)
	}
	if len(with) != 3 {
		t.Errorf("with merge: %v", with)
	}
}

func TestMCP_CheckCleanBranch_Passes(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "app/feature.go", "package main\n") })

	text, isError, err := callTool(t, connect(t, testConfig(dir, false)), "gate_check", "ticket-1")

	if err != nil || isError || !strings.Contains(text, "RESULT: PASSED") {
		t.Fatalf("err = %v, isError = %v, text %s", err, isError, text)
	}
}

func TestMCP_CheckWeakenedBranch_ReportsBlocked(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-2", func() { write(t, dir, "coverage.txt", "10\n") })

	text, isError, err := callTool(t, connect(t, testConfig(dir, false)), "gate_check", "ticket-2")

	if err != nil || !isError || !strings.Contains(text, "RESULT: BLOCKED") {
		t.Fatalf("err = %v, isError = %v, text %s", err, isError, text)
	}
}

func TestMCP_MergeNotAllowed_ToolRejectedAndMainUntouched(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-3", func() { write(t, dir, "app/feature.go", "package main\n") })
	before := runGit(t, dir, "rev-parse", "main")

	_, isError, err := callTool(t, connect(t, testConfig(dir, false)), "gate_merge", "ticket-3")

	if err == nil && !isError {
		t.Fatal("want the call to be rejected")
	}
	if after := runGit(t, dir, "rev-parse", "main"); after != before {
		t.Error("main moved although merging was not allowed")
	}
}

func TestMCP_MergeAllowedAndPassed_Merges(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-4", func() { write(t, dir, "app/feature.go", "package main\n") })

	text, isError, err := callTool(t, connect(t, testConfig(dir, true)), "gate_merge", "ticket-4")

	if err != nil || isError || !strings.Contains(text, "merged ticket-4 into main") {
		t.Fatalf("err = %v, isError = %v, text %s", err, isError, text)
	}
}

func TestMCP_MergeAllowedAndBlocked_MainUntouched(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-5", func() { write(t, dir, "coverage.txt", "10\n") })
	before := runGit(t, dir, "rev-parse", "main")

	_, isError, err := callTool(t, connect(t, testConfig(dir, true)), "gate_merge", "ticket-5")

	if err != nil || !isError {
		t.Fatalf("err = %v, isError = %v, want a blocked result", err, isError)
	}
	if after := runGit(t, dir, "rev-parse", "main"); after != before {
		t.Error("main moved although the gate blocked the branch")
	}
}

func TestMCP_BranchLooksLikeFlagOrShell_Rejected(t *testing.T) {
	dir := newRepo(t)
	session := connect(t, testConfig(dir, true))
	for _, branch := range []string{"--allow-protected", "-x", "a b", "a;rm", ""} {
		text, isError, err := callTool(t, session, "gate_check", branch)

		if err != nil || !isError || text != "invalid branch name" {
			t.Errorf("branch %q: err = %v, isError = %v, text %q", branch, err, isError, text)
		}
	}
}

func TestBearerAuth_Requests_OnlyCorrectTokenPasses(t *testing.T) {
	handler := bearerAuth(map[string]string{"secret": "builder-1"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	cases := map[string]int{"Bearer secret": http.StatusNoContent, "Bearer wrong": http.StatusUnauthorized, "secret": http.StatusUnauthorized, "": http.StatusUnauthorized}
	for header, want := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != want {
			t.Errorf("header %q: status %d, want %d", header, rec.Code, want)
		}
	}
}

func TestBearerAuth_ValidToken_CallerNameReplacesClientSuppliedHeader(t *testing.T) {
	var seen string
	handler := bearerAuth(map[string]string{"s1": "builder-1", "s2": "builder-2"}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.Header.Get(callerHeader) }))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer s2")
	req.Header.Set(callerHeader, "spoofed")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "builder-2" {
		t.Fatalf("caller = %q", seen)
	}
}

func TestLoadTokens_Inputs_ParsedOrRejected(t *testing.T) {
	tests := []struct {
		name, single, many string
		wantCount          int
		wantErr            bool
	}{
		{"WHEN only a single token is set THEN it is named default", "abc", "", 1, false},
		{"WHEN named tokens are set THEN each gets its name", "", "a=1,b=2", 2, false},
		{"WHEN nothing is set THEN loading fails", "", "", 0, true},
		{"WHEN a pair has no equals sign THEN loading fails", "", "a1", 0, true},
		{"WHEN two callers share a token THEN loading fails", "", "a=1,b=1", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := loadTokens(tc.single, tc.many)

			if (err != nil) != tc.wantErr || (err == nil && len(tokens) != tc.wantCount) {
				t.Fatalf("tokens %v, err %v", tokens, err)
			}
		})
	}
}

func TestCallerOf_HeadersPresent_NamedFromHeader(t *testing.T) {
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{callerHeader: {"builder-1"}, "User-Agent": {"aimebu/1.2"}}}}

	who := callerOf(req)

	if who.name != "builder-1" || who.userAgent != "aimebu/1.2" {
		t.Fatalf("caller %+v", who)
	}
}

func TestCallerOf_NoTransportInfo_NamedStdio(t *testing.T) {
	who := callerOf(&mcp.CallToolRequest{})

	if who.name != stdioCaller {
		t.Fatalf("caller %+v", who)
	}
}

func TestMCP_Request_LoggedWithToolBranchClientAndOutcome(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "app/feature.go", "package main\n") })
	var logged strings.Builder
	cfg := testConfig(dir, false)
	cfg.log = &logged

	if _, _, err := callTool(t, connect(t, cfg), "gate_check", "ticket-1"); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"request tool=gate_check branch=ticket-1", `client="test/"`, "finished tool=gate_check", "exit="} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, logged.String())
		}
	}
}
