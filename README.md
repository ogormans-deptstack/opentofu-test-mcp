# opentofu-test-mcp

An [MCP](https://modelcontextprotocol.io) server for the **OpenTofu / Terraform
native test framework** (`.tftest.hcl` / `.tfmock.hcl`).

LLM agents are good at writing Terraform but routinely produce broken `run`,
`assert`, and `mock_provider` blocks because the test-framework grammar is small
but unforgiving. This server gives an agent the exact block reference, scaffolds
tests from a module's real variables and outputs, generates mock providers from
the installed provider schema, validates test files statically, and runs the
suite — so the agent can author and check tests without guessing.

It drives the `tofu` CLI when present and falls back to `terraform`.

## Tools

| Tool | What it does |
|------|--------------|
| `tf_explain_test_schema` | Reference + example for a test-framework block (`run`, `assert`, `mock_provider`, `expect_failures`, …). Empty topic returns an overview. No CLI needed. |
| `tf_scaffold_test` | Reads a module's variables and outputs and generates a `.tftest.hcl` starter with a run block, assertions per output, and placeholders for required variables. |
| `tf_gen_mock_provider` | Runs `providers schema -json` on an initialized module and emits a `mock_provider` block with `mock_resource` defaults pre-filled for computed attributes. |
| `tf_validate_test` | Statically checks a `.tftest.hcl` (by path or raw content): invalid `command`, asserts missing `condition`/`error_message`, duplicate run names, no run blocks. Does not run OpenTofu. |
| `tf_run_tests` | Runs `tofu test -json` and returns a condensed pass/fail summary with per-run status. |

## Install

```sh
go install github.com/ogormans-deptstack/opentofu-test-mcp@latest
```

Requires `tofu` (preferred) or `terraform` on `PATH` for the schema- and
run-backed tools. `tf_explain_test_schema` and `tf_validate_test` work without
either.

## Configure (client)

```json
{
  "mcpServers": {
    "opentofu-test": {
      "command": "opentofu-test-mcp"
    }
  }
}
```

## Example

Ask the agent to "scaffold a test for this module and run it":

1. `tf_scaffold_test { "dir": "." }` → a `.tftest.hcl` skeleton.
2. `tf_gen_mock_provider { "dir": "." }` → `mock_provider` blocks (module must be
   initialized).
3. `tf_validate_test { "path": "main.tftest.hcl" }` → structural problems, if any.
4. `tf_run_tests { "dir": "." }` → `passed=1 failed=0 errored=0 skipped=0`.

## Development

```sh
go test ./...
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE).
