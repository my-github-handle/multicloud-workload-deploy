output "verdict" {
  description = "Top-level preflight verdict: green | amber | red."
  value       = local.verdict
}

output "report" {
  description = "The full decoded preflight Report (stages + results)."
  value       = local.report
  # Non-sensitive: the Report carries check ids, statuses, human-readable messages, and remediation
  # hints only — no credentials, tokens, or kubeconfig contents. Left unmarked so an SE/CI can read
  # and persist it as a plain artifact.
}

output "install_tier" {
  description = "Derived install tier: \"A\" (operator) or \"B\" (namespaced manifests). Override via install_tier_override. (\"RED\" is surfaced only when fail_on_red is disabled and the identity cannot deploy at all — it deliberately trips downstream validation.)"
  value       = local.install_tier
}
