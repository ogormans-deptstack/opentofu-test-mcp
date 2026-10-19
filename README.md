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

Prebuilt binaries for macOS, Linux, and Windows (amd64/arm64) are distributed
with checksums on [GitHub Releases](https://github.com/ogormans-deptstack/opentofu-test-mcp/releases).
Native binaries still need a separately installed OpenTofu or Terraform CLI.

### Container

The release workflow publishes an image with OpenTofu included:

```sh
docker run --rm -i -v "$PWD:/workspace" \
  ghcr.io/ogormans-deptstack/opentofu-test-mcp:0.1.0
```

Use `-i`, not `-t`: stdio must remain an unmodified JSON-RPC stream. Module paths
passed to tools must be container paths, such as `/workspace`, not host paths.
Only mount modules you trust: native tests can apply real infrastructure and
execute provider code. The image does not inherit host cloud credentials.

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

For Docker, configure `command` as `docker` and `args` as
`["run", "--rm", "-i", "-v", "/absolute/module/path:/workspace",
"ghcr.io/ogormans-deptstack/opentofu-test-mcp:0.1.0"]`.

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

### Automated MCP checks with Inspector

Requires Node.js **22.19+**, Go, and OpenTofu **1.13.1** on `PATH`. Inspector is
pinned in `package-lock.json`; Node is development tooling, not a server runtime
dependency.

```sh
npm ci
go build -o dist/opentofu-test-mcp .
npm run test:mcp
```

This launches the built server through the
[Inspector CLI](https://modelcontextprotocol.io/docs/2026-07-28/tools/inspector/cli)
over real stdio. It checks initialization, the exact five-tool inventory,
portable input schemas (`tools/list --strict`), structured output, all five
tools, a missing-input error, and both passing and deliberately failing native
tests. Fixtures use OpenTofu's built-in `terraform_data` provider, so no cloud
credentials or provider downloads are needed. The failing fixture is intentional.
Tests work in a temporary copy and use an in-memory Inspector secret store.

To check the container with the same suite:

```sh
docker build -t opentofu-test-mcp:local .
MCP_IMAGE=opentofu-test-mcp:local npm run test:mcp
```

To check another build, set `MCP_SERVER=/absolute/path/to/opentofu-test-mcp`.
CI runs both native and container smoke tests in addition to Go race tests.

For individual checks or interactive debugging:

```sh
# Machine-readable tool inventory; non-zero on schema portability errors.
npx --no-install @modelcontextprotocol/inspector --cli ./dist/opentofu-test-mcp \
  --method tools/list --strict --format json --stored-auth-only

# A single tool call, without starting a browser.
npx --no-install @modelcontextprotocol/inspector --cli ./dist/opentofu-test-mcp \
  --method tools/call --tool-name tf_explain_test_schema \
  --tool-args-json '{"topic":"run"}' --format json --stored-auth-only

# Web Inspector for inspecting requests, responses, and schemas.
npm run inspect
```

Before contributing an MCP change, reproduce the behavior using a disposable
fixture, add assertions for the regression and its error path, then run the Go
and Inspector suites. Assert the JSON **result**, not just process exit status:
Inspector exits **5** for `isError: true` and **6** for strict schema failures,
but a valid `tf_validate_test` response with `valid: false` or a native test
summary with `failed > 0` still exits successfully. Inspector checks complement,
not replace, unit tests; they do not prove semantic correctness of every
Terraform expression or compatibility with every MCP client.

The [Inspector documentation](https://modelcontextprotocol.io/docs/2026-07-28/tools/inspector)
also describes the TUI and experimental `mcpdo` persistent connection client.
The CLI is sufficient for this server's stateless requests; use a persistent
client for future session-dependent flows. The pinned CLI defaults to the legacy
protocol era, matching the server SDK; do not force modern-era negotiation
without upgrading and testing the server first. Keep the web UI's default
loopback binding and authentication enabled, and do not put secrets in fixture
files or captured transcripts.

## Releasing

1. Update `server.json`'s top-level version, package version, and OCI identifier
   tag together. Commit the changes and wait for CI on that commit.
2. Tag that tested commit (for example `git tag v0.1.0`) and push the tag.
   `.github/workflows/release.yml` refuses mismatched versions and reruns CI.
3. The workflow builds six native archives with checksums, creates a GitHub
   release, and publishes a Linux amd64/arm64 OCI image. The image embeds the
   release version and the MCP Registry ownership label
   `io.modelcontextprotocol.server.name`.
4. On the first release, GitHub may create the GHCR package as **private**.
   Change its package settings to **public**, then rerun failed workflow jobs.
   The workflow checks anonymous image access before registry publication.
5. The final job validates `server.json` and publishes it using GitHub Actions
   OIDC (`id-token: write`), without a long-lived registry secret.

Registry publishing follows the official
[OCI package requirements](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/package-types.mdx#dockeroci-images)
and [GitHub Actions guide](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/github-actions.mdx).
The registry hosts metadata, not the binary or image, so the package must be
publicly available first. Do not submit a PR to the registry repository.

If only registry publication needs retrying outside Actions:

```sh
mcp-publisher validate server.json
mcp-publisher login github
mcp-publisher publish server.json
curl --fail --get 'https://registry.modelcontextprotocol.io/v0.1/servers' \
  --data-urlencode 'search=io.github.ogormans-deptstack/opentofu-test'
```

Authenticate as `ogormans-deptstack` (or an authorized namespace owner), not an
unrelated GitHub account. To build native archives locally without publishing:
`bash scripts/build-release.sh 0.1.0`.

## License

MIT — see [LICENSE](LICENSE).
