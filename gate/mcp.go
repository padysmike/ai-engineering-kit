package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpTokenEnv       = "GATE_MCP_TOKEN"
	mcpTokensEnv      = "GATE_MCP_TOKENS"
	callerHeader      = "X-Gate-Caller"
	defaultCaller     = "default"
	stdioCaller       = "stdio"
	unknownCaller     = "unknown"
	headerReadTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

var branchName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

type action int

const (
	actionCheck action = iota
	actionMerge
)

type mcpConfig struct {
	repo       string
	base       string
	targets    string
	allowMerge bool
	remote     string
	github     *githubAPI
	approvals  *approvalStore
	log        io.Writer
	sandbox    sandbox
}

type caller struct {
	name      string
	client    string
	userAgent string
}

type gateService struct {
	cfg mcpConfig
}

type branchInput struct {
	Branch string `json:"branch" jsonschema:"branch to check"`
}

func runMCP(args []string) int {
	cfg, listen, err := parseMCPConfig(args)
	if err != nil {
		say(os.Stderr, "gate mcp: %v\n", err)
		return exitError
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server := newMCPServer(cfg)
	if listen == "" {
		return serveStdio(ctx, server)
	}
	tokens, err := loadTokens(os.Getenv(mcpTokenEnv), os.Getenv(mcpTokensEnv))
	if err != nil {
		say(os.Stderr, "gate mcp: %v\n", err)
		return exitError
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return serveHTTP(ctx, listen, bearerAuth(tokens, handler))
}

func parseMCPConfig(args []string) (mcpConfig, string, error) {
	cfg := mcpConfig{log: os.Stderr}
	var listen, slug, state string
	var box sandboxFlags
	fs := flag.NewFlagSet("gate mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.repo, "repo", ".", "path to the git repository")
	fs.StringVar(&cfg.base, "base", "main", "trusted base branch")
	fs.StringVar(&cfg.targets, "targets", defaultTargets, "comma-separated make targets")
	fs.BoolVar(&cfg.allowMerge, "allow-merge", false, "expose the gate_merge tool")
	fs.StringVar(&cfg.remote, "remote", "origin", "git remote that points at GitHub")
	fs.StringVar(&slug, "github", "", "GitHub repository as owner/repo; checks pull requests and posts the gate status")
	fs.StringVar(&state, "state", "", "directory for human approval requests; enables the approval flow when set")
	fs.StringVar(&listen, "listen", "", "serve MCP over HTTP on this address instead of stdio, for example 127.0.0.1:8765")
	box.register(fs)
	if err := fs.Parse(args); err != nil {
		return cfg, "", err
	}
	var err error
	if cfg.sandbox, err = box.choose(); err != nil {
		return cfg, "", err
	}
	if err = cfg.sandbox.forbidHostWith(flagUse{"state", state != ""}, flagUse{"github", slug != ""}, flagUse{"allow-merge", cfg.allowMerge}); err != nil {
		return cfg, "", err
	}
	if state != "" {
		cfg.approvals = &approvalStore{dir: state}
	}
	if slug == "" {
		return cfg, listen, nil
	}
	gh, err := newGithubAPI(slug, os.Getenv(tokenEnv))
	if err != nil {
		return cfg, "", err
	}
	cfg.github = &gh
	return cfg, listen, nil
}

func serveStdio(ctx context.Context, server *mcp.Server) int {
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		say(os.Stderr, "gate mcp: %v\n", err)
		return exitBlocked
	}
	return exitPassed
}

func serveHTTP(ctx context.Context, addr string, handler http.Handler) int {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: headerReadTimeout}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	say(os.Stderr, "gate mcp: listening on %s\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		say(os.Stderr, "gate mcp: %v\n", err)
		return exitBlocked
	}
	return exitPassed
}

func loadTokens(single, many string) (map[string]string, error) {
	tokens := map[string]string{}
	add := func(name, token string) error {
		if token == "" {
			return fmt.Errorf("empty token for %q", name)
		}
		if _, dup := tokens[token]; dup {
			return errors.New("the same token is used for two callers")
		}
		tokens[token] = name
		return nil
	}
	if single != "" {
		if err := add(defaultCaller, single); err != nil {
			return nil, err
		}
	}
	for _, pair := range splitList(many) {
		name, token, ok := strings.Cut(pair, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("%s must look like name=token,name=token", mcpTokensEnv)
		}
		if err := add(name, token); err != nil {
			return nil, err
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("set %s or %s when --listen is used", mcpTokenEnv, mcpTokensEnv)
	}
	return tokens, nil
}

func matchToken(tokens map[string]string, header string) (string, bool) {
	name, found := "", false
	for token, owner := range tokens {
		if subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+token)) == 1 {
			name, found = owner, true
		}
	}
	return name, found
}

func bearerAuth(tokens map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := matchToken(tokens, strings.TrimSpace(r.Header.Get("Authorization")))
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Header.Set(callerHeader, name)
		next.ServeHTTP(w, r)
	})
}

func callerOf(req *mcp.CallToolRequest) caller {
	c := caller{name: stdioCaller}
	if req.Extra != nil && req.Extra.Header != nil {
		c.name = req.Extra.Header.Get(callerHeader)
		c.userAgent = req.Extra.Header.Get("User-Agent")
	}
	if c.name == "" {
		c.name = unknownCaller
	}
	if req.Session != nil {
		if p := req.Session.InitializeParams(); p != nil && p.ClientInfo != nil {
			c.client = p.ClientInfo.Name + "/" + p.ClientInfo.Version
		}
	}
	return c
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	say(w, "%s "+format+"\n", append([]any{time.Now().UTC().Format(time.RFC3339)}, args...)...)
}

func (a action) toolName() string {
	if a == actionMerge {
		return "gate_merge"
	}
	return "gate_check"
}

func newMCPServer(cfg mcpConfig) *mcp.Server {
	svc := &gateService{cfg: cfg}
	server := mcp.NewServer(&mcp.Implementation{Name: "gate", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gate_check",
		Description: "Run the merge gate on a branch against the trusted base branch. Changes nothing except the gate status on the pull request when GitHub is configured.",
	}, svc.handler(actionCheck))
	if cfg.allowMerge {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "gate_merge",
			Description: "Run the merge gate and merge the branch only if every check passes.",
		}, svc.handler(actionMerge))
	}
	return server
}

func (s *gateService) handler(act action) mcp.ToolHandlerFor[branchInput, any] {
	return func(_ context.Context, req *mcp.CallToolRequest, in branchInput) (*mcp.CallToolResult, any, error) {
		who := callerOf(req)
		tool := act.toolName()
		if !branchName.MatchString(in.Branch) {
			logf(s.cfg.log, "rejected tool=%s caller=%q client=%q branch=%q", tool, who.name, who.client, in.Branch)
			return errorResult("invalid branch name"), nil, nil
		}
		logf(s.cfg.log, "request tool=%s branch=%s caller=%q client=%q agent=%q", tool, in.Branch, who.name, who.client, who.userAgent)
		start := time.Now()
		var buf strings.Builder
		code := s.execute(in.Branch, act, &buf)
		logf(s.cfg.log, "finished tool=%s branch=%s caller=%q exit=%d took=%s", tool, in.Branch, who.name, code, time.Since(start).Round(time.Millisecond))
		return resultForExit(buf.String(), code), nil, nil
	}
}

func (s *gateService) execute(branch string, act action, out *strings.Builder) int {
	cfg := s.cfg
	merge := act == actionMerge
	if cfg.github != nil {
		return runPullRequest(prOptions{
			repoDir: cfg.repo, remote: cfg.remote, base: cfg.base, branch: branch,
			targets: cfg.targets, merge: merge, gh: *cfg.github,
			approvals: cfg.approvals, notify: os.Stderr, sandbox: cfg.sandbox,
		}, out)
	}
	repo, err := filepath.Abs(cfg.repo)
	if err != nil {
		say(out, "gate: %v\n", err)
		return exitError
	}
	return execute(options{
		repo: repo, base: cfg.base, branch: branch, targets: splitList(cfg.targets),
		protected: splitList(defaultProtected), merge: merge,
		approvals: cfg.approvals, notify: os.Stderr, sandbox: cfg.sandbox,
	}, out)
}

func resultForExit(text string, code int) *mcp.CallToolResult {
	if code != exitPassed {
		return errorResult(text)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
}
