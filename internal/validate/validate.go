// Package validate performs static checks on a .tftest.hcl file, catching the
// mistakes agents most commonly make before OpenTofu is ever invoked.
package validate

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Finding is a single diagnostic. Severity is "error" or "warning".
type Finding struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
}

// Report is the outcome of validating a file.
type Report struct {
	Valid    bool      `json:"valid"`
	Runs     int       `json:"runs"`
	Findings []Finding `json:"findings"`
}

var fileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "run", LabelNames: []string{"name"}},
		{Type: "variables"},
		{Type: "provider", LabelNames: []string{"name"}},
		{Type: "mock_provider", LabelNames: []string{"name"}},
		{Type: "override_resource"},
		{Type: "override_data"},
		{Type: "override_module"},
	},
}

var runSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "command"},
		{Name: "expect_failures"},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "assert"},
		{Type: "variables"},
		{Type: "plan_options"},
		{Type: "module"},
	},
}

var assertSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "condition", Required: true},
		{Name: "error_message", Required: true},
	},
}

// File validates the .tftest.hcl at path.
func File(path string) (*Report, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}
	return Source(src, path)
}

// Source validates test file bytes (filename is used only for diagnostics).
func Source(src []byte, filename string) (*Report, error) {
	rep := &Report{}
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		for _, d := range diags {
			rep.Findings = append(rep.Findings, fromDiag(d))
		}
		return rep, nil
	}

	content, _, diags := file.Body.PartialContent(fileSchema)
	addDiags(rep, diags)

	runNames := map[string]bool{}
	for _, block := range content.Blocks {
		if block.Type != "run" {
			continue
		}
		rep.Runs++
		name := block.Labels[0]
		if runNames[name] {
			rep.add("error", fmt.Sprintf("duplicate run name %q; run labels must be unique within a file", name), block.DefRange.Start.Line)
		}
		runNames[name] = true
		validateRun(rep, block)
	}

	if rep.Runs == 0 {
		rep.add("warning", "no run blocks found; a test file needs at least one run block to do anything", 0)
	}

	rep.Valid = !rep.hasErrors()
	sort.SliceStable(rep.Findings, func(i, j int) bool { return rep.Findings[i].Line < rep.Findings[j].Line })
	return rep, nil
}

func validateRun(rep *Report, block *hcl.Block) {
	content, _, diags := block.Body.PartialContent(runSchema)
	addDiags(rep, diags)

	if attr, ok := content.Attributes["command"]; ok {
		// command is a bare keyword (plan/apply), not a string literal.
		cmd := hcl.ExprAsKeyword(attr.Expr)
		if cmd != "" && cmd != "plan" && cmd != "apply" {
			rep.add("error", fmt.Sprintf("run %q: command must be plan or apply, got %q", block.Labels[0], cmd), attr.Range.Start.Line)
		}
	}

	asserts := 0
	for _, inner := range content.Blocks {
		if inner.Type != "assert" {
			continue
		}
		asserts++
		_, _, d := inner.Body.PartialContent(assertSchema)
		for _, diag := range d {
			if strings.Contains(diag.Summary, "Missing required argument") {
				rep.add("error", fmt.Sprintf("run %q: assert block is missing condition or error_message", block.Labels[0]), inner.DefRange.Start.Line)
			}
		}
	}
	if asserts == 0 {
		rep.add("warning", fmt.Sprintf("run %q has no assert blocks; it will only check that plan/apply succeeds", block.Labels[0]), block.DefRange.Start.Line)
	}
}

func (r *Report) add(sev, msg string, line int) {
	r.Findings = append(r.Findings, Finding{Severity: sev, Message: msg, Line: line})
}

func (r *Report) hasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}

func addDiags(rep *Report, diags hcl.Diagnostics) {
	for _, d := range diags {
		rep.Findings = append(rep.Findings, fromDiag(d))
	}
}

func fromDiag(d *hcl.Diagnostic) Finding {
	sev := "warning"
	if d.Severity == hcl.DiagError {
		sev = "error"
	}
	line := 0
	if d.Subject != nil {
		line = d.Subject.Start.Line
	}
	msg := d.Summary
	if d.Detail != "" {
		msg += ": " + d.Detail
	}
	return Finding{Severity: sev, Message: msg, Line: line}
}
