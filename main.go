// Command opentofu-test-mcp is an MCP server for the OpenTofu/Terraform native
// test framework. It helps an LLM agent explain, scaffold, mock, validate, and
// run .tftest.hcl tests.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/scaffold"
	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/testschema"
	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/tofu"
	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/validate"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "opentofu-test",
		Version: version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tf_explain_test_schema",
		Description: "Explain the OpenTofu/Terraform native test framework grammar (.tftest.hcl / .tfmock.hcl). Call with an empty topic for an overview, or a topic like 'run', 'assert', 'mock_provider', 'expect_failures' for its block reference and an example.",
	}, explainTestSchema)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tf_scaffold_test",
		Description: "Generate a .tftest.hcl starter test for a module directory by reading its input variables and outputs. Produces a run block with assertions for each output and placeholders for required variables.",
	}, scaffoldTest)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tf_gen_mock_provider",
		Description: "Generate a mock_provider block for an initialized module directory by reading 'providers schema -json'. Pre-fills mock_resource defaults for computed attributes. The directory must have been initialized first.",
	}, genMockProvider)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tf_validate_test",
		Description: "Statically validate a .tftest.hcl file (by path or raw content) and report structural problems such as an invalid command, assert blocks missing condition/error_message, duplicate run names, or no run blocks. Does not run OpenTofu.",
	}, validateTest)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tf_run_tests",
		Description: "Run 'tofu test -json' in a module directory and return a condensed pass/fail summary with per-run status. Optionally filter to a single test file. Requires the directory to be initialized.",
	}, runTests)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// ---- tf_explain_test_schema ----

type ExplainInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"a test-framework topic such as run, assert, mock_provider, mock_resource, expect_failures, tftest, tfmock; empty returns an overview"`
}

type ExplainOutput struct {
	Reference string   `json:"reference"`
	Topics    []string `json:"topics"`
}

func explainTestSchema(_ context.Context, _ *mcp.CallToolRequest, in ExplainInput) (*mcp.CallToolResult, ExplainOutput, error) {
	ref, err := testschema.Explain(in.Topic)
	if err != nil {
		return nil, ExplainOutput{}, err
	}
	out := ExplainOutput{Reference: ref, Topics: testschema.Topics()}
	return textResult(ref), out, nil
}

// ---- tf_scaffold_test ----

type ScaffoldInput struct {
	Dir     string `json:"dir" jsonschema:"path to the module directory to scaffold a test for"`
	Command string `json:"command,omitempty" jsonschema:"the run command to use: plan (default) or apply"`
	Mock    bool   `json:"mock,omitempty" jsonschema:"also emit empty mock_provider blocks for the module's providers"`
}

type ScaffoldOutput struct {
	Content string           `json:"content"`
	Module  *scaffold.Module `json:"module"`
}

func scaffoldTest(_ context.Context, _ *mcp.CallToolRequest, in ScaffoldInput) (*mcp.CallToolResult, ScaffoldOutput, error) {
	if strings.TrimSpace(in.Dir) == "" {
		return nil, ScaffoldOutput{}, fmt.Errorf("dir is required")
	}
	mod, err := scaffold.Inspect(in.Dir)
	if err != nil {
		return nil, ScaffoldOutput{}, err
	}
	content := scaffold.TestFile(mod, in.Command, in.Mock)
	return textResult(content), ScaffoldOutput{Content: content, Module: mod}, nil
}

// ---- tf_gen_mock_provider ----

type MockInput struct {
	Dir      string `json:"dir" jsonschema:"path to an initialized module directory"`
	Provider string `json:"provider,omitempty" jsonschema:"restrict output to this provider source or local name; empty emits all providers"`
	Resource string `json:"resource,omitempty" jsonschema:"restrict output to this resource type"`
	Binary   string `json:"binary,omitempty" jsonschema:"override the CLI binary (tofu or terraform, or an absolute path)"`
}

type MockOutput struct {
	Content string `json:"content"`
}

func genMockProvider(ctx context.Context, _ *mcp.CallToolRequest, in MockInput) (*mcp.CallToolResult, MockOutput, error) {
	if strings.TrimSpace(in.Dir) == "" {
		return nil, MockOutput{}, fmt.Errorf("dir is required")
	}
	bin, err := tofu.Binary(in.Binary)
	if err != nil {
		return nil, MockOutput{}, err
	}
	schemas, err := tofu.ProvidersSchema(ctx, bin, in.Dir)
	if err != nil {
		return nil, MockOutput{}, err
	}
	var sb strings.Builder
	for source, entry := range schemas.ProviderSchemas {
		if in.Provider != "" && source != in.Provider && !strings.HasSuffix(source, "/"+in.Provider) {
			continue
		}
		sb.WriteString(scaffold.MockProvider(source, entry, in.Resource))
		sb.WriteString("\n")
	}
	content := strings.TrimSpace(sb.String())
	if content == "" {
		return nil, MockOutput{}, fmt.Errorf("no matching provider/resource schema found (is the directory initialized?)")
	}
	content += "\n"
	return textResult(content), MockOutput{Content: content}, nil
}

// ---- tf_validate_test ----

type ValidateInput struct {
	Path    string `json:"path,omitempty" jsonschema:"path to a .tftest.hcl file to validate"`
	Content string `json:"content,omitempty" jsonschema:"raw .tftest.hcl content to validate instead of a path"`
}

func validateTest(_ context.Context, _ *mcp.CallToolRequest, in ValidateInput) (*mcp.CallToolResult, *validate.Report, error) {
	var (
		rep *validate.Report
		err error
	)
	switch {
	case in.Content != "":
		rep, err = validate.Source([]byte(in.Content), "inline.tftest.hcl")
	case in.Path != "":
		rep, err = validate.File(in.Path)
	default:
		return nil, nil, fmt.Errorf("provide either path or content")
	}
	if err != nil {
		return nil, nil, err
	}
	return textResult(summarizeReport(rep)), rep, nil
}

// ---- tf_run_tests ----

type RunInput struct {
	Dir    string `json:"dir" jsonschema:"path to an initialized module directory containing tests"`
	Filter string `json:"filter,omitempty" jsonschema:"restrict the run to a single test file, e.g. tests/main.tftest.hcl"`
	Binary string `json:"binary,omitempty" jsonschema:"override the CLI binary (tofu or terraform, or an absolute path)"`
}

func runTests(ctx context.Context, _ *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, *tofu.TestResult, error) {
	if strings.TrimSpace(in.Dir) == "" {
		return nil, nil, fmt.Errorf("dir is required")
	}
	bin, err := tofu.Binary(in.Binary)
	if err != nil {
		return nil, nil, err
	}
	res, err := tofu.RunTests(ctx, bin, in.Dir, in.Filter)
	if err != nil {
		return nil, nil, err
	}
	summary := fmt.Sprintf("passed=%d failed=%d errored=%d skipped=%d", res.Passed, res.Failed, res.Errored, res.Skipped)
	return textResult(summary), res, nil
}

// textResult wraps a string as the human-readable content of a tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func summarizeReport(rep *validate.Report) string {
	var sb strings.Builder
	if rep.Valid {
		fmt.Fprintf(&sb, "valid: %d run block(s)\n", rep.Runs)
	} else {
		sb.WriteString("invalid\n")
	}
	for _, f := range rep.Findings {
		if f.Line > 0 {
			fmt.Fprintf(&sb, "  [%s] line %d: %s\n", f.Severity, f.Line, f.Message)
		} else {
			fmt.Fprintf(&sb, "  [%s] %s\n", f.Severity, f.Message)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
