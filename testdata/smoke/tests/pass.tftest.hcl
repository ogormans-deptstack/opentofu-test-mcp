run "passes" {
  command = plan

  assert {
    condition     = output.name == "inspector"
    error_message = "The fixture should use its default name."
  }
}
