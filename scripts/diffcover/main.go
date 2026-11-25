package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	exitPassed  = 0
	exitBlocked = 1
	exitError   = 2
)

var (
	hunkHeader   = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
	profileEntry = regexp.MustCompile(`^(.+):(\d+)\.\d+,(\d+)\.\d+ (\d+) (\d+)$`)
	modulePrefix = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

type block struct {
	start, end, statements, count int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("diffcover", flag.ContinueOnError)
	fs.SetOutput(out)
	profile := fs.String("profile", "", "coverage profile written by go test -coverprofile (required)")
	base := fs.String("base", defaultBase(), "git ref to compare against")
	minimum := fs.Float64("min", 80, "minimum percentage of changed statements that must be covered")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	changed, blocks, err := load(*profile, *base)
	if err != nil {
		say(out, "diffcover: %v\n", err)
		return exitError
	}
	covered, total, missed := measure(changed, blocks)
	return report(out, covered, total, missed, *minimum)
}

func defaultBase() string {
	if sha := os.Getenv("GATE_BASE_SHA"); sha != "" {
		return sha
	}
	return "main"
}

func load(profilePath, base string) (map[string]map[int]bool, map[string][]block, error) {
	if profilePath == "" {
		return nil, nil, fmt.Errorf("--profile is required")
	}
	diff, err := gitDiff(base)
	if err != nil {
		return nil, nil, err
	}
	module, err := moduleName()
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(profilePath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	blocks, err := parseProfile(f, module)
	return parseChangedLines(diff), blocks, err
}

func gitDiff(base string) (string, error) {
	cmd := exec.Command("git", "diff", "-U0", "--no-color", base+"...HEAD", "--", "*.go", ":!*_test.go")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff %s: %w: %s", base, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func moduleName() (string, error) {
	raw, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	m := modulePrefix.FindSubmatch(raw)
	if m == nil {
		return "", fmt.Errorf("go.mod has no module line")
	}
	return string(m[1]), nil
}

func parseChangedLines(diff string) map[string]map[int]bool {
	changed := map[string]map[int]bool{}
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if path, ok := strings.CutPrefix(line, "+++ b/"); ok {
			file = path
			continue
		}
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		first, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for l := first; l < first+count; l++ {
			if changed[file] == nil {
				changed[file] = map[int]bool{}
			}
			changed[file][l] = true
		}
	}
	return changed
}

func parseProfile(r io.Reader, module string) (map[string][]block, error) {
	blocks := map[string][]block{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		m := profileEntry.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		file := strings.TrimPrefix(m[1], module+"/")
		blocks[file] = append(blocks[file], block{
			start:      atoi(m[2]),
			end:        atoi(m[3]),
			statements: atoi(m[4]),
			count:      atoi(m[5]),
		})
	}
	return blocks, sc.Err()
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func measure(changed map[string]map[int]bool, blocks map[string][]block) (covered, total int, missed map[string][]int) {
	missed = map[string][]int{}
	for file, lines := range changed {
		for line := range lines {
			state := lineState(blocks[file], line)
			if state == notStatement {
				continue
			}
			total++
			if state == coveredLine {
				covered++
				continue
			}
			missed[file] = append(missed[file], line)
		}
		sort.Ints(missed[file])
	}
	return covered, total, missed
}

type state int

const (
	notStatement state = iota
	coveredLine
	missedLine
)

func lineState(blocks []block, line int) state {
	result := notStatement
	for _, b := range blocks {
		if b.statements == 0 || line < b.start || line > b.end {
			continue
		}
		if b.count > 0 {
			return coveredLine
		}
		result = missedLine
	}
	return result
}

func report(out io.Writer, covered, total int, missed map[string][]int, minimum float64) int {
	if total == 0 {
		say(out, "%s\n", "no changed statements to cover")
		return exitPassed
	}
	percent := float64(covered) * 100 / float64(total)
	say(out, "changed statements covered: %d of %d (%.1f%%, minimum %.1f%%)\n", covered, total, percent, minimum)
	if percent >= minimum {
		return exitPassed
	}
	files := make([]string, 0, len(missed))
	for file := range missed {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		say(out, "  uncovered %s: %v\n", file, missed[file])
	}
	return exitBlocked
}

func say(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }
