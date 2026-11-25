package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseChangedLines_HunksWithAndWithoutCounts_ExpandedPerLine(t *testing.T) {
	diff := "+++ b/a/x.go\n@@ -1,0 +5,2 @@\n+a\n+b\n@@ -9 +20 @@\n+c\n@@ -30,2 +40,0 @@\n-d\n-e\n+++ b/y.go\n@@ -1 +1 @@\n+z\n"

	got := parseChangedLines(diff)

	want := map[string]map[int]bool{"a/x.go": {5: true, 6: true, 20: true}, "y.go": {1: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseProfile_ModulePrefixedPaths_KeyedByRelativeFile(t *testing.T) {
	profile := "mode: set\nmod/app/s.go:10.2,12.3 2 1\nmod/app/s.go:14.2,15.3 1 0\nmod/other.go:1.1,2.2 1 1\n"

	got, err := parseProfile(strings.NewReader(profile), "mod")

	if err != nil {
		t.Fatal(err)
	}
	if len(got["app/s.go"]) != 2 || got["app/s.go"][1].count != 0 || len(got["other.go"]) != 1 {
		t.Fatalf("unexpected blocks: %+v", got)
	}
}

func TestMeasure_ChangedLines_CountsOnlyStatements(t *testing.T) {
	changed := map[string]map[int]bool{"s.go": {10: true, 11: true, 14: true, 99: true}}
	blocks := map[string][]block{"s.go": {{start: 10, end: 12, statements: 2, count: 1}, {start: 14, end: 15, statements: 1, count: 0}}}

	covered, total, missed := measure(changed, blocks)

	if covered != 2 || total != 3 || !reflect.DeepEqual(missed["s.go"], []int{14}) {
		t.Fatalf("covered %d, total %d, missed %v", covered, total, missed)
	}
}

func TestReport_Percentages_ExitCodesFollowMinimum(t *testing.T) {
	tests := []struct {
		name           string
		covered, total int
		minimum        float64
		wantCode       int
		wantOutputPart string
	}{
		{"WHEN nothing changed THEN it passes", 0, 0, 80, exitPassed, "no changed statements"},
		{"WHEN coverage meets the minimum THEN it passes", 4, 5, 80, exitPassed, "4 of 5"},
		{"WHEN coverage is below the minimum THEN it is blocked and lists lines", 1, 5, 80, exitBlocked, "uncovered f.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder

			code := report(&out, tc.covered, tc.total, map[string][]int{"f.go": {3}}, tc.minimum)

			if code != tc.wantCode || !strings.Contains(out.String(), tc.wantOutputPart) {
				t.Fatalf("code %d, output %q", code, out.String())
			}
		})
	}
}

func TestRun_MissingProfile_ExitsWithError(t *testing.T) {
	var out strings.Builder

	code := run(nil, &out)

	if code != exitError || !strings.Contains(out.String(), "--profile is required") {
		t.Fatalf("code %d, output %q", code, out.String())
	}
}

func TestGitDiff_DiffFileSet_ReadsFileInsteadOfGit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changes.diff")
	if err := os.WriteFile(path, []byte("diff content"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GATE_DIFF_FILE", path)

	got, err := gitDiff("main")

	if err != nil || got != "diff content" {
		t.Fatalf("got %q, err %v", got, err)
	}
}
