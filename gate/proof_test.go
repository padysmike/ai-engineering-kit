package main

import (
	"reflect"
	"strings"
	"testing"
)

const (
	healthyCode     = "package lib\n\nfunc Healthy() bool { return true }\n"
	healthyTest     = "package lib\n\nimport \"testing\"\n\nfunc TestHealthy(t *testing.T) {\n\tif !Healthy() {\n\t\tt.Fatal(\"unhealthy\")\n\t}\n}\n"
	taggedTest      = "//go:build slow\n\npackage lib\n\nimport \"testing\"\n\nfunc TestSlow(t *testing.T) {}\n"
	featuresRunner  = "package lib\n\nimport \"testing\"\n\nfunc TestFeatures(t *testing.T) {}\n\nfunc helperNotATest() {}\n\nfunc Testable(t *testing.T) {}\n"
	exitEarlyInInit = "package lib\n\nimport \"os\"\n\nfunc init() {\n\tos.Exit(0)\n}\n"
)

func moduleRepo(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	write(t, dir, "go.mod", "module demo\n\ngo 1.25\n")
	write(t, dir, "lib/health.go", healthyCode)
	write(t, dir, "lib/health_test.go", healthyTest)
	write(t, dir, "lib/slow_test.go", taggedTest)
	commitAll(t, dir, "module")
	return dir
}

func proofFor(t *testing.T, dir, branch string) []result {
	t.Helper()
	base := strings.TrimSpace(runGit(t, dir, "rev-parse", "main"))
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", branch))
	worktree, cleanup, err := trustedWorktree(dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	o := options{repo: dir, targets: []string{"test"}}
	return proofOfPass(o, checkRun{dir: worktree, baseSHA: base}, head)
}

func TestProofOfPass_ProductionInitExitsZero_Blocked(t *testing.T) {
	dir := moduleRepo(t)
	branchWith(t, dir, "ticket-1", func() {
		write(t, dir, "lib/health.go", strings.Replace(healthyCode, "true", "false", 1))
		write(t, dir, "lib/boot.go", exitEarlyInInit)
	})

	got := proofFor(t, dir, "ticket-1")

	if len(got) != 1 || got[0].ok || !strings.Contains(strings.Join(got[0].detail, "\n"), "demo/lib TestHealthy") {
		t.Fatalf("results %+v", got)
	}
}

func TestProofOfPass_HonestBranch_Passes(t *testing.T) {
	dir := moduleRepo(t)
	branchWith(t, dir, "ticket-2", func() { write(t, dir, "lib/extra.go", "package lib\n\nfunc Extra() int { return 1 }\n") })

	got := proofFor(t, dir, "ticket-2")

	if len(got) != 1 || !got[0].ok || got[0].name != "approved tests reported passing (1 of 1)" {
		t.Fatalf("results %+v", got)
	}
}

func TestProofOfPass_TargetsWithoutTests_NothingToProve(t *testing.T) {
	got := proofOfPass(options{targets: []string{"lint"}}, checkRun{}, "")

	if got != nil {
		t.Fatalf("results %+v", got)
	}
}

func TestRequiredTests_Base_ListsTestsAndScenariosButNotTaggedFiles(t *testing.T) {
	dir := moduleRepo(t)
	write(t, dir, "lib/features_test.go", featuresRunner)
	write(t, dir, "scenarios/login.feature", "Feature: login\n  Scenario: Health is  ok\n  Scenario Outline: Login as <role>\n")
	commitAll(t, dir, "scenarios")

	got, err := requiredTests(dir, "main")

	want := []requiredTest{
		{pkg: "demo/lib", name: "TestFeatures"},
		{pkg: "demo/lib", name: "TestHealthy"},
		{pkg: "demo/lib", name: "TestFeatures/Health_is_ok"},
		{pkg: "demo/lib", name: "TestFeatures/Login_as", isNamePrefix: true},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestApprovedTestList_ApprovedDeletion_NoLongerRequired(t *testing.T) {
	dir := moduleRepo(t)
	branchWith(t, dir, "ticket-3", func() { runGit(t, dir, "rm", "-q", "lib/health_test.go") })
	base := strings.TrimSpace(runGit(t, dir, "rev-parse", "main"))
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", "ticket-3"))

	strict, strictErr := approvedTestList(options{repo: dir}, base, head)
	approved, approvedErr := approvedTestList(options{repo: dir, allowTestChanges: true}, base, head)

	if strictErr != nil || approvedErr != nil || len(strict) != 1 || len(approved) != 0 {
		t.Fatalf("strict %+v (%v), approved %+v (%v)", strict, strictErr, approved, approvedErr)
	}
}

func TestMissingPasses_JSONOutput_OnlyPassEventsCount(t *testing.T) {
	output := strings.Join([]string{
		`{"Action":"pass","Package":"demo/lib","Test":"TestHealthy"}`,
		`{"Action":"fail","Package":"demo/lib","Test":"TestBroken"}`,
		`{"Action":"output","Package":"demo/lib","Test":"TestForged","Output":"{\"Action\":\"pass\",\"Package\":\"demo/lib\",\"Test\":\"TestForged\"}\n"}`,
		`{"Action":"pass","Package":"demo/other","Test":"TestElsewhere"}`,
		`{"Action":"pass","Package":"demo/lib","Test":"TestFeatures/Login_as_admin"}`,
		`{"Action":"pass","Package":"demo/lib","Test":"TestFeatures/Twice#01"}`,
		`not json at all`,
	}, "\n")
	required := []requiredTest{
		{pkg: "demo/lib", name: "TestHealthy"},
		{pkg: "demo/lib", name: "TestBroken"},
		{pkg: "demo/lib", name: "TestForged"},
		{pkg: "demo/lib", name: "TestElsewhere"},
		{pkg: "demo/lib", name: "TestFeatures/Login_as", isNamePrefix: true},
		{pkg: "demo/lib", name: "TestFeatures/Twice"},
	}

	got := missingPasses(required, passEvents(output))

	want := []requiredTest{required[1], required[2], required[3]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestIsTestName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "WHEN the name is Test THEN it is a test", input: "Test", want: true},
		{name: "WHEN an upper case letter follows Test THEN it is a test", input: "TestHealthy", want: true},
		{name: "WHEN an underscore follows Test THEN it is a test", input: "Test_health", want: true},
		{name: "WHEN a lower case letter follows Test THEN it is not a test", input: "Testable", want: false},
		{name: "WHEN the name is TestMain THEN it is not a test", input: "TestMain", want: false},
		{name: "WHEN the name lacks the prefix THEN it is not a test", input: "helper", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTestName(tc.input)

			if got != tc.want {
				t.Fatalf("isTestName(%q) = %v", tc.input, got)
			}
		})
	}
}
