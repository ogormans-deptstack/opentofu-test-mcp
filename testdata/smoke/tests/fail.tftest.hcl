run "fails" {
  command = plan

  assert {
    condition     = output.name == "deliberately-wrong"
    error_message = "Intentional failure for the MCP smoke test."
  }
}
