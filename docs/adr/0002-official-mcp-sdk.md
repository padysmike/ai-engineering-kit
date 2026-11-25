# 0002. The gate serves MCP through the official Go SDK

- Status: accepted
- Date: 2026-11-25

## Context
Agents running in a sandbox need a way to request a check or a merge without holding merge rights. MCP is the interface the agent tooling already speaks. A hand-written JSON-RPC server was prototyped and worked, but it pinned one protocol version and offered no HTTP transport.

## Decision
Use `github.com/modelcontextprotocol/go-sdk` (the official SDK) in `gate/mcp.go`. The server exposes `gate_check` always and `gate_merge` only when started with `--allow-merge`. The agent supplies only a validated branch name.

## Consequences
- The SDK requires Go 1.26 at the time of writing, so `go.mod` moves to 1.26.
- Protocol negotiation and transports come from the SDK. HTTP transport can be added without redesign.
- Another dependency to keep updated; the gate has no other external dependencies.
