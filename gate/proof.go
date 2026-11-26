package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strings"
)

const (
	scenarioRunner = "TestFeatures"
	scenarioDir    = "scenarios/"
	scenarioSuffix = ".feature"
)

var (
	proofCommand     = []string{"go", "test", "-json", "-count=1", "./..."}
	exactScenarios   = []string{"Scenario:", "Example:"}
	outlineScenarios = []string{"Scenario Outline:", "Scenario Template:"}
)

type requiredTest struct {
	pkg          string
	name         string
	isNamePrefix bool
}

func (r requiredTest) satisfiedBy(pkg, test string) bool {
	if pkg != r.pkg {
		return false
	}
	if r.isNamePrefix {
		return strings.HasPrefix(test, r.name)
	}
	return test == r.name || strings.HasPrefix(test, r.name+"#")
}

func (r requiredTest) String() string {
	return r.pkg + " " + r.name
}

type testEvent struct {
	Action  string
	Package string
	Test    string
}

func proofOfPass(o options, run checkRun, branchSHA string) []result {
	if !slices.ContainsFunc(o.targets, isApprovedTarget) {
		return nil
	}
	required, err := approvedTestList(o, run.baseSHA, branchSHA)
	if err != nil {
		return []result{{name: "approved tests could not be listed", detail: []string{err.Error()}}}
	}
	if len(required) == 0 {
		return nil
	}
	run.command = proofCommand
	missing := missingPasses(required, passEvents(testJSONIgnoringExitCode(o.sandbox, run)))
	name := fmt.Sprintf("approved tests reported passing (%d of %d)", len(required)-len(missing), len(required))
	if len(missing) == 0 {
		return []result{{name: name, ok: true}}
	}
	return []result{{name: name, detail: describeMissing(missing)}}
}

func isApprovedTarget(target string) bool {
	return slices.Contains(approvedTargets, target)
}

func approvedTestList(o options, baseSHA, branchSHA string) ([]requiredTest, error) {
	required, err := requiredTests(o.repo, baseSHA)
	if err != nil || !o.allowTestChanges {
		return required, err
	}
	stillThere, err := requiredTests(o.repo, branchSHA)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(required, func(r requiredTest) bool { return !slices.Contains(stillThere, r) }), nil
}

func testJSONIgnoringExitCode(box sandbox, run checkRun) string {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := box.command(ctx, run)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run()
	return stdout.String()
}

func passEvents(output string) []testEvent {
	var passed []testEvent
	for _, line := range strings.Split(output, "\n") {
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Action == "pass" && ev.Test != "" {
			passed = append(passed, ev)
		}
	}
	return passed
}

func missingPasses(required []requiredTest, passed []testEvent) []requiredTest {
	var missing []requiredTest
	for _, r := range required {
		if !slices.ContainsFunc(passed, func(ev testEvent) bool { return r.satisfiedBy(ev.Package, ev.Test) }) {
			missing = append(missing, r)
		}
	}
	return missing
}

func describeMissing(missing []requiredTest) []string {
	lines := []string{"no pass reported for:"}
	for _, r := range missing {
		lines = append(lines, r.String())
	}
	return tail(strings.Join(lines, "\n"), detailLines)
}

func requiredTests(repo, sha string) ([]requiredTest, error) {
	if _, err := git(repo, "cat-file", "-e", sha+":go.mod"); err != nil {
		return nil, nil
	}
	files, err := treeFiles(repo, sha)
	if err != nil {
		return nil, err
	}
	module, err := modulePath(repo, sha)
	if err != nil {
		return nil, err
	}
	required, err := goTests(repo, sha, module, files)
	if err != nil {
		return nil, err
	}
	runner := slices.IndexFunc(required, func(r requiredTest) bool { return r.name == scenarioRunner })
	if runner < 0 {
		return required, nil
	}
	scenarios, err := scenarioTests(repo, sha, required[runner].pkg, files)
	return append(required, scenarios...), err
}

func treeFiles(repo, sha string) ([]string, error) {
	raw, err := git(repo, "ls-tree", "-r", "-z", "--name-only", sha)
	if err != nil {
		return nil, err
	}
	return splitList(strings.ReplaceAll(raw, "\x00", ",")), nil
}

func modulePath(repo, sha string) (string, error) {
	goMod, err := git(repo, "show", sha+":go.mod")
	if err != nil {
		return "", fmt.Errorf("reading go.mod of the base: %w", err)
	}
	for _, line := range strings.Split(goMod, "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(name), `"`), nil
		}
	}
	return "", fmt.Errorf("go.mod of the base has no module line")
}

func goTests(repo, sha, module string, files []string) ([]requiredTest, error) {
	var required []requiredTest
	for _, file := range files {
		if !isRunnableTestFile(file) {
			continue
		}
		src, err := git(repo, "show", sha+":"+file)
		if err != nil {
			return nil, err
		}
		names, err := testFuncs(file, src)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			required = append(required, requiredTest{pkg: importPath(module, path.Dir(file)), name: name})
		}
	}
	return required, nil
}

func isRunnableTestFile(file string) bool {
	return strings.HasSuffix(file, "_test.go") && !strings.Contains("/"+file, "/testdata/") && !strings.Contains("/"+file, "/vendor/")
}

func importPath(module, dir string) string {
	if dir == "." {
		return module
	}
	return module + "/" + dir
}

func testFuncs(file, src string) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parsing %s of the base: %w", file, err)
	}
	if hasBuildConstraint(parsed) {
		return nil, nil
	}
	var names []string
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && isTestName(fn.Name.Name) {
			names = append(names, fn.Name.Name)
		}
	}
	return names, nil
}

func hasBuildConstraint(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() < file.Package && slices.ContainsFunc(group.List, isBuildDirective) {
			return true
		}
	}
	return false
}

func isBuildDirective(c *ast.Comment) bool {
	return strings.HasPrefix(c.Text, "//go:build")
}

func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok || name == "TestMain" {
		return false
	}
	return rest == "" || !strings.ContainsAny(rest[:1], "abcdefghijklmnopqrstuvwxyz")
}

func scenarioTests(repo, sha, pkg string, files []string) ([]requiredTest, error) {
	var required []requiredTest
	for _, file := range files {
		if !strings.HasPrefix(file, scenarioDir) || !strings.HasSuffix(file, scenarioSuffix) {
			continue
		}
		feature, err := git(repo, "show", sha+":"+file)
		if err != nil {
			return nil, err
		}
		required = append(required, scenariosIn(feature, pkg)...)
	}
	return required, nil
}

func scenariosIn(feature, pkg string) []requiredTest {
	var required []requiredTest
	for _, line := range strings.Split(feature, "\n") {
		line = strings.TrimSpace(line)
		if name, ok := cutAnyPrefix(line, exactScenarios); ok {
			required = append(required, requiredTest{pkg: pkg, name: subtestName(name)})
		}
		if name, ok := cutAnyPrefix(line, outlineScenarios); ok {
			before, _, _ := strings.Cut(name, "<")
			required = append(required, requiredTest{pkg: pkg, name: subtestName(before), isNamePrefix: true})
		}
	}
	return required
}

func cutAnyPrefix(line string, prefixes []string) (string, bool) {
	for _, p := range prefixes {
		if rest, ok := strings.CutPrefix(line, p); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func subtestName(scenario string) string {
	return scenarioRunner + "/" + strings.Join(strings.Fields(scenario), "_")
}
