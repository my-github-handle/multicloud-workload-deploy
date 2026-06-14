# Running Preflight

The preflight checker validates deploy prerequisites bottom-up and emits a green/amber/red report.
The Terraform deploy paths run it automatically and gate `apply` on the verdict; you can also run
it standalone to diagnose a cluster before deploying.

> Project guide: [`README.md`](./README.md) · Design:
> [`../components/preflight-checker.md`](../components/preflight-checker.md).

---

## Build

```bash
mage preflightBuild        # → operator/bin/preflight
```

## Run standalone against a cluster

```bash
operator/bin/preflight \
  --kubeconfig "$KUBECONFIG" \
  --namespace <app-namespace> \
  --mode agnostic \
  --exit-on-red          # optional: exit non-zero on a red verdict (CLI convenience)
```

Pretty-print the staged report:

```bash
operator/bin/preflight --kubeconfig "$KUBECONFIG" --namespace <ns> \
  | jq -r '.report_json | fromjson | .stages[] | "\(.id) \(.name): \(.status)"'
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `--kubeconfig` | "" | path to the target cluster's kubeconfig; empty on a real run yields a **blocking red Stage 4** (no cluster to deploy to) |
| `--namespace` | `default` | target workload namespace (Stage 4/5 checks) |
| `--mode` | `agnostic` | `agnostic` (BYOC — every stage blocks) or `full` (greenfield — cloud stages 0–3 are informational) |
| `--cloud` | "" | `aws` \| `gcp` \| `azure`; empty uses the built-in fake (cloud stages report stub green). Real providers are added per cloud. |
| `--exit-on-red` | false | exit non-zero on a red verdict; for standalone CLI use only |

---

## Reading the result

Output is a flat JSON object with two string keys:

```json
{ "verdict": "green|amber|red", "report_json": "<full staged Report as a JSON string>" }
```

- **green** — all prerequisites met; apply proceeds.
- **amber** — documented gaps (e.g. no Cilium, no metrics-server, Tier B install); apply proceeds,
  gaps recorded.
- **red** — a blocking failure; apply must stop. The offending `CheckResult` carries a `message`
  and `remediation`.

The binary **always exits 0** when there is no `--exit-on-red`, even on internal errors (bad
kubeconfig, unimplemented `--cloud`) — those become a red verdict carrying the error, so a caller
(Terraform) gates on the verdict rather than a crash.

---

## How Terraform uses it

The Terraform deploy path invokes the binary through an `external` data source and blocks `apply`
on a red verdict (illustrative — the Terraform module ships with the Layer-3 deploy path):

```hcl
data "external" "preflight" {
  program = [local.preflight_binary, "--mode=agnostic",
             "--kubeconfig", var.kubeconfig, "--namespace", var.namespace]
}

check "preflight_not_red" {
  assert {
    condition     = data.external.preflight.result.verdict != "red"
    error_message = "Preflight failed (red): ${data.external.preflight.result.report_json}"
  }
}
```

Because the binary exits 0 and prints a flat string map, the data source never errors; the gate
keys on `verdict`.

---

## Common results & remediation

| Result ID | Meaning when not green | Action |
|---|---|---|
| `k8s.unreachable` (red) | no kubeconfig / cluster unreachable | pass `--kubeconfig` for the target cluster |
| `k8s.networkpolicy` (red) | NetworkPolicy API not served | install a CNI that supports NetworkPolicy |
| `k8s.installtier` (amber) | Tier B — namespace-only permissions | operator lifecycle unavailable; grant CRD+ClusterRole create for Tier A, or use [Tier B](./helm-only-tier-b.md) |
| `k8s.installtier` (red) | cannot create namespaced Deployments | grant `create` on `apps/deployments` in the namespace |
| `k8s.metricsserver` (amber) | metrics-server absent | install metrics-server so the HPA can scale on CPU |
| `k8s.cilium` (amber) | Cilium not detected | FQDN egress/Hubble fall back to the perimeter firewall + cloud flow logs |
| `k8s.argorollouts*` (amber) | Argo Rollouts absent/incompatible/no traffic primitive | canary degrades to RollingUpdate / replica-weighted |
| `k8s.workloadidentity` (red) | workload SA absent and not creatable | grant `create` on serviceaccounts, or pre-create the annotated SA |
| `egress.controlplane_fqdn` (red) | control-plane FQDN unreachable | allow the connect-agent's outbound FQDN through the egress firewall |
| `workload.namespace` (red) | namespace missing and not creatable | pre-create the namespace, or grant `create` on namespaces |

---

## Modes: agnostic vs full

- **agnostic** (BYOC, the primary path): every stage blocks. A red anywhere stops the apply and
  short-circuits later stages.
- **full** (greenfield `<cloud>-full`): the cloud stages (0–3) are provisioned by the same apply,
  so they're informational — they still run and report their true severity, but a red there does
  not block and does not skip the Kubernetes stages. A red in a Kubernetes stage still blocks.
