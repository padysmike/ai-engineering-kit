package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP clients give up on a tool call long before a gate run ends.
const defaultResultWait = 10 * time.Second

type gateRun struct {
	branch  string
	action  action
	started time.Time
	done    chan struct{}
	report  string
	code    int
}

func (r *gateRun) finished() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// await answers with the run's result, or says that it is still running if it
// does not end within the wait. report and code are read only after done.
func (r *gateRun) await(ctx context.Context, wait time.Duration) *mcp.CallToolResult {
	select {
	case <-r.done:
		return resultForExit(r.report, r.code)
	case <-time.After(wait):
	case <-ctx.Done():
	}
	text := fmt.Sprintf("RESULT: RUNNING\n%s for %s has run for %s. Call gate_result with the same branch to get its result.\n",
		r.action.toolName(), r.branch, time.Since(r.started).Round(time.Second))
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// gateRuns keeps the latest run of every branch, in memory only.
type gateRuns struct {
	mu     sync.Mutex
	latest map[string]*gateRun
}

// startOrJoin never starts a second run beside one that is still going, so a
// repeated request cannot merge twice.
func (s *gateRuns) startOrJoin(branch string, act action, work func() (string, int)) (*gateRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.latest[branch]; ok && !current.finished() {
		if current.action != act {
			return nil, fmt.Errorf("%s is still running for %s; get its result with gate_result, then try again", current.action.toolName(), branch)
		}
		return current, nil
	}
	next := &gateRun{branch: branch, action: act, started: time.Now(), done: make(chan struct{})}
	if s.latest == nil {
		s.latest = map[string]*gateRun{}
	}
	s.latest[branch] = next
	go func() {
		defer close(next.done)
		next.report, next.code = work()
	}()
	return next, nil
}

func (s *gateRuns) latestOf(branch string) (*gateRun, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.latest[branch]
	return run, ok
}
