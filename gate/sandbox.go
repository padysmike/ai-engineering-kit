package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	diffFileName    = "changes.diff"
	containerDiff   = "/gate/" + diffFileName
	containerWork   = "/work"
	containerMods   = "/go/pkg/mod"
	containerTmp    = "/tmp"
	sandboxMemory   = "4g"
	sandboxPids     = "512"
	imageFlagName   = "sandbox-image"
	hostFlagName    = "host-checks"
	unsafeHostUsage = "run the checks on the host without a container; the branch's code can then read host files, so use it only for code you trust"
)

var envAllowlist = []string{"PATH", "LANG", "LC_ALL", "TMPDIR", "GOPATH", "GOMODCACHE", "GOCACHE", "GOFLAGS", "GOPROXY", "GONOSUMDB", "GONOSUMCHECK", "GOSUMDB", "GOTOOLCHAIN", "GOROOT", "HOME"}

type sandbox struct {
	image string
}

type checkRun struct {
	dir      string
	target   string
	baseSHA  string
	diffFile string
	modCache string
	command  []string
}

func (r checkRun) argv() []string {
	if r.command != nil {
		return r.command
	}
	// Without -f, make prefers a GNUmakefile or makefile, which a branch could add.
	return []string{"make", "-s", r.target, "-f", trustedMakefile}
}

type sandboxFlags struct {
	image string
	host  bool
}

func (f *sandboxFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.image, imageFlagName, "", "docker image that runs the checks with no network and none of the gate's environment")
	fs.BoolVar(&f.host, hostFlagName, false, unsafeHostUsage)
}

func (f sandboxFlags) choose() (sandbox, error) {
	switch {
	case f.image != "" && f.host:
		return sandbox{}, fmt.Errorf("use either --%s or --%s, not both", imageFlagName, hostFlagName)
	case f.image == "" && !f.host:
		return sandbox{}, fmt.Errorf("the checks run the branch's code; pass --%s IMAGE, or --%s to accept the risk", imageFlagName, hostFlagName)
	}
	return sandbox{image: f.image}, nil
}

type flagUse struct {
	name string
	set  bool
}

func (s sandbox) forbidHostWith(uses ...flagUse) error {
	if s.enabled() {
		return nil
	}
	for _, use := range uses {
		if use.set {
			return fmt.Errorf("--%s cannot be combined with --%s: the branch's code would run as the host user and could approve or merge itself", hostFlagName, use.name)
		}
	}
	return nil
}

func (s sandbox) enabled() bool {
	return s.image != ""
}

func cleanEnv(extra ...string) []string {
	var env []string
	for _, name := range envAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(env, extra...)
}

func (s sandbox) command(ctx context.Context, run checkRun) *exec.Cmd {
	if !s.enabled() {
		argv := run.argv()
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = run.dir
		cmd.Env = cleanEnv("REQUIRE_LINT=1", "GATE_BASE_SHA="+run.baseSHA, "GATE_DIFF_FILE="+run.diffFile)
		return cmd
	}
	return exec.CommandContext(ctx, "docker", s.dockerArgs(run)...)
}

func (s sandbox) dockerArgs(run checkRun) []string {
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--pids-limit", sandboxPids,
		"--memory", sandboxMemory,
		"--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
		"--tmpfs", containerTmp + ":exec,size=2g",
		"-v", run.dir + ":" + containerWork + ":ro",
		"-v", run.diffFile + ":" + containerDiff + ":ro",
		"-v", run.modCache + ":" + containerMods + ":ro",
		"-w", containerWork,
		"-e", "HOME=" + containerTmp,
		"-e", "GOCACHE=" + containerTmp + "/gocache",
		"-e", "GOLANGCI_LINT_CACHE=" + containerTmp + "/lint",
		"-e", "GOMODCACHE=" + containerMods,
		"-e", "GOFLAGS=-buildvcs=false",
		"-e", "GOPROXY=off",
		"-e", "GOTOOLCHAIN=local",
		"-e", "REQUIRE_LINT=1",
		"-e", "GATE_BASE_SHA=" + run.baseSHA,
		"-e", "GATE_DIFF_FILE=" + containerDiff,
		s.image,
	}
	return append(args, run.argv()...)
}

func (s sandbox) prefetchModules(dir string) (string, error) {
	if !s.enabled() {
		return "", nil
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return emptyModCache(dir)
	}
	if err := hostGo(dir, "mod", "download"); err != nil {
		return "", fmt.Errorf("downloading modules: %w", err)
	}
	return goEnv(dir, "GOMODCACHE")
}

func emptyModCache(dir string) (string, error) {
	empty := filepath.Join(filepath.Dir(dir), "modcache")
	return empty, os.MkdirAll(empty, 0o755)
}

func hostGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = cleanEnv("GOTOOLCHAIN=local")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(out.String()))
	}
	return nil
}

func goEnv(dir, name string) (string, error) {
	cmd := exec.Command("go", "env", name)
	cmd.Dir = dir
	cmd.Env = cleanEnv("GOTOOLCHAIN=local")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func writeDiff(repo, baseSHA, branchSHA, path string) error {
	diff, err := git(repo, "diff", "-U0", "--no-color", baseSHA+"..."+branchSHA, "--", "*.go", ":!*_test.go")
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(diff), 0o600)
}

func (s sandbox) prepareRun(repo, worktree, baseSHA, branchSHA string) (checkRun, error) {
	run := checkRun{dir: worktree, baseSHA: baseSHA, diffFile: filepath.Join(filepath.Dir(worktree), diffFileName)}
	if err := writeDiff(repo, baseSHA, branchSHA, run.diffFile); err != nil {
		return run, err
	}
	modCache, err := s.prefetchModules(worktree)
	run.modCache = modCache
	return run, err
}
