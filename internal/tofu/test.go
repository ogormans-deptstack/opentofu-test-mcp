package tofu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
)

// TestResult is a condensed summary of a `test -json` run, shaped for an agent
// to reason about rather than the raw log stream.
type TestResult struct {
	Passed   int       `json:"passed"`
	Failed   int       `json:"failed"`
	Errored  int       `json:"errored"`
	Skipped  int       `json:"skipped"`
	Runs     []TestRun `json:"runs"`
	Problems []string  `json:"problems,omitempty"`
}

// TestRun is a single run block's outcome.
type TestRun struct {
	File   string `json:"file,omitempty"`
	Run    string `json:"run,omitempty"`
	Status string `json:"status,omitempty"`
}

// testLine is the subset of the JSON log schema we care about.
type testLine struct {
	Level   string `json:"@level"`
	Message string `json:"@message"`
	Type    string `json:"type"`
	TestRun *struct {
		Path   string `json:"path"`
		Run    string `json:"run"`
		Status string `json:"status"`
	} `json:"test_run"`
	TestSummary *struct {
		Status  string `json:"status"`
		Passed  int    `json:"passed"`
		Failed  int    `json:"failed"`
		Errored int    `json:"errored"`
		Skipped int    `json:"skipped"`
	} `json:"test_summary"`
}

// RunTests executes `test -json` (optionally filtered to a single file) and
// condenses the stream. A non-nil error is only returned for failures to launch
// the CLI or parse; test failures themselves are reported in the result so the
// agent can inspect them.
func RunTests(ctx context.Context, bin, dir, filter string) (*TestResult, error) {
	args := []string{"test", "-json", "-no-color"}
	if filter != "" {
		args = append(args, "-filter="+filter)
	}
	// Test failures make the CLI exit non-zero; that is expected, so the exit
	// error is tolerated as long as we got parseable output.
	out, runErr := Run(ctx, bin, dir, args...)

	res, parsedAny := parseTestStream(out)
	if !parsedAny && runErr != nil {
		// Nothing parseable came back: surface the launch/CLI error.
		return nil, runErr
	}
	return res, nil
}

// parseTestStream condenses the `test -json` log stream. The bool reports
// whether any well-formed JSON line was seen.
func parseTestStream(out []byte) (*TestResult, bool) {
	res := &TestResult{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	parsedAny := false
	for scanner.Scan() {
		var line testLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		parsedAny = true
		switch {
		case line.TestRun != nil:
			res.Runs = append(res.Runs, TestRun{
				File:   line.TestRun.Path,
				Run:    line.TestRun.Run,
				Status: line.TestRun.Status,
			})
		case line.TestSummary != nil:
			res.Passed = line.TestSummary.Passed
			res.Failed = line.TestSummary.Failed
			res.Errored = line.TestSummary.Errored
			res.Skipped = line.TestSummary.Skipped
		case line.Level == "error" && line.Message != "":
			res.Problems = append(res.Problems, line.Message)
		}
	}
	return res, parsedAny
}
