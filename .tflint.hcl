# tflint configuration for the Layer-3 Terraform modules and the live root.
# The terraform ruleset enforces standard module conventions (documented variables/outputs,
# required_providers, no deprecated syntax, naming).
config {
  call_module_type = "local"
}

plugin "terraform" {
  enabled = true
  preset  = "recommended"
}
