import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { after, before, test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const inspector = join(root, "node_modules/.bin/mcp-inspector");
const image = process.env.MCP_IMAGE;
const binary = resolve(process.env.MCP_SERVER ?? join(root, "dist/opentofu-test-mcp"));
const fixture = mkdtempSync(join(tmpdir(), "opentofu-test-mcp-"));
const moduleDir = image ? "/workspace" : fixture;
const dockerArgs = ["run", "--rm", "-i", "--volume", `${fixture}:/workspace`];
const server = image ? ["docker", ...dockerArgs, image] : [binary];
const env = {
  ...process.env,
  MCP_INSPECTOR_SECRET_STORE: "memory",
  MCP_STORAGE_DIR: join(fixture, ".inspector"),
  MCP_AUTO_OPEN_ENABLED: "false",
};

function execute(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: root,
    env,
    encoding: "utf8",
    timeout: 60_000,
    maxBuffer: 4 * 1024 * 1024,
    ...options,
  });
  assert.ifError(result.error);
  assert.equal(result.signal, null, `${command} terminated: ${result.signal}`);
  return result;
}

function inspect(method, args = [], expectedStatus = 0) {
  const result = execute(process.execPath, [
    inspector, "--cli", ...server, "--",
    "--stored-auth-only", "--format", "json", "--method", method, ...args,
  ]);
  assert.equal(result.status, expectedStatus, result.stderr || result.stdout);
  const envelope = JSON.parse(result.stdout);
  assert.ok(envelope.result, `Missing Inspector result: ${result.stdout}`);
  return envelope.result;
}

function call(name, args, expectedStatus = 0) {
  return inspect("tools/call", [
    "--tool-name", name, "--tool-args-json", JSON.stringify(args),
  ], expectedStatus);
}

function output(name, args) {
  const result = call(name, args);
  assert.notEqual(result.isError, true, JSON.stringify(result));
  assert.ok(result.structuredContent, `Missing structured output from ${name}`);
  assert.ok(result.content.some((item) => item.type === "text" && item.text.length > 0));
  return result.structuredContent;
}

before(() => {
  cpSync(join(root, "testdata/smoke"), fixture, { recursive: true });
  const initArgs = ["init", "-backend=false", "-input=false", "-no-color"];
  const result = image
    ? execute("docker", [...dockerArgs, "--entrypoint", "tofu", image, ...initArgs])
    : execute("tofu", initArgs, { cwd: fixture });
  assert.equal(result.status, 0, result.stderr || result.stdout);
});

after(() => rmSync(fixture, { recursive: true, force: true }));

test("initializes and exposes exactly five portable tool schemas", () => {
  const initialized = inspect("initialize");
  assert.equal(initialized.serverInfo.name, "opentofu-test");
  if (process.env.MCP_EXPECTED_VERSION) {
    assert.equal(initialized.serverInfo.version, process.env.MCP_EXPECTED_VERSION);
  }
  const { tools } = inspect("tools/list", ["--strict"]);
  assert.deepEqual(tools.map((tool) => tool.name).sort(), [
    "tf_explain_test_schema",
    "tf_gen_mock_provider",
    "tf_run_tests",
    "tf_scaffold_test",
    "tf_validate_test",
  ]);
  for (const tool of tools) {
    assert.equal(tool.inputSchema.type, "object");
    assert.equal(tool.outputSchema.type, "object");
  }
});

test("explains a test block over stdio", () => {
  const explained = output("tf_explain_test_schema", { topic: "run" });
  assert.match(explained.reference, /command/);
  assert.ok(explained.topics.includes("run"));
});

test("scaffolds a module and validates the generated test", () => {
  const scaffold = output("tf_scaffold_test", { dir: moduleDir });
  assert.match(scaffold.content, /output\.name/);
  const report = output("tf_validate_test", { content: scaffold.content });
  assert.equal(report.valid, true);
  assert.equal(report.runs, 1);
});

test("validates a file and reports invalid content without a protocol error", () => {
  const report = output("tf_validate_test", { path: `${moduleDir}/tests/pass.tftest.hcl` });
  assert.equal(report.valid, true);
  const invalid = output("tf_validate_test", { content: 'run "bad" {\n command = destroy\n}\n' });
  assert.equal(invalid.valid, false);
  assert.ok(invalid.findings.some((finding) => finding.severity === "error"));
});

test("generates a mock from a real built-in provider schema without downloads", () => {
  const mock = output("tf_gen_mock_provider", {
    dir: moduleDir, provider: "terraform", resource: "terraform_data",
  });
  assert.match(mock.content, /mock_provider "terraform"/);
  assert.match(mock.content, /mock_resource "terraform_data"/);
  assert.match(mock.content, /id\s*=\s*"mock-id"/);
});

test("reports a passing native test run", () => {
  const summary = output("tf_run_tests", { dir: moduleDir, filter: "tests/pass.tftest.hcl" });
  assert.deepEqual(
    [summary.passed, summary.failed, summary.errored, summary.skipped], [1, 0, 0, 0],
  );
  assert.ok(summary.runs.some((run) => run.run === "passes" && run.status === "pass"));
});

test("reports a failing native test run as data, not transport success alone", () => {
  const summary = output("tf_run_tests", { dir: moduleDir, filter: "tests/fail.tftest.hcl" });
  assert.deepEqual(
    [summary.passed, summary.failed, summary.errored, summary.skipped], [0, 1, 0, 0],
  );
  assert.ok(summary.runs.some((run) => run.run === "fails" && run.status === "fail"));
});

test("missing required input returns a tool error and Inspector exit code 5", () => {
  const result = call("tf_scaffold_test", {}, 5);
  assert.equal(result.isError, true);
});

test("registry version and OCI tag agree", () => {
  const metadata = JSON.parse(readFileSync(join(root, "server.json"), "utf8"));
  assert.ok(metadata.description.length <= 100);
  assert.equal(metadata.packages[0].registryType, "oci");
  for (const field of ["registryBaseUrl", "version", "fileSha256"]) {
    assert.equal(Object.hasOwn(metadata.packages[0], field), false, `OCI forbids ${field}`);
  }
  assert.equal(
    metadata.packages[0].identifier,
    `ghcr.io/ogormans-deptstack/opentofu-test-mcp:${metadata.version}`,
  );
});
