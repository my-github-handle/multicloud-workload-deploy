# Tests

Two tiers, separated by what infrastructure they need.

## Local — no external infrastructure

Runs on any dev laptop or CI runner with no cluster and no cloud account. This is the default
gate (`mage test`) and what CI runs on every push.

- **Unit tests** — pure logic, e.g. `operator/internal/render` (chart render, Tier A/B parity).
- **envtest** — `operator/internal/controller` spins up a local kube-apiserver + etcd (downloaded
  binaries, no cluster) to exercise the reconcile loop against a real API server. There is no
  operator pod and no scheduler, so pods never actually run — these assert reconciliation, owner
  references, status, and validation, not live serving.

```bash
mage test        # unit + envtest, with the ≥80% coverage gate
```

## Real-world — requires live infrastructure

Lives in [`test/e2e`](./e2e) and is build-tagged `//go:build e2e`, so it never compiles or runs
under `mage test`. It drives the **real operator Deployment** end-to-end against a live cluster
selected by the ambient `KUBECONFIG`: apply a `Workload`, wait for reconciliation, and assert the
live child objects, `Ready=True`, converged replicas, and HPA wiring.

Prerequisite: the operator chart is installed on the target cluster (see
[`runbooks/verify-core-on-kind.md`](./runbooks/verify-core-on-kind.md)).

```bash
mage testE2E     # go test -tags e2e ./test/e2e/... against $KUBECONFIG
```

Environment overrides: `E2E_NAMESPACE`, `E2E_IMAGE`, `E2E_PORT`, `E2E_RUN_AS_ROOT=true`
(relaxes the hardened security context for images that must run as root).

**HPA scale-up test.** `TestHPAScalesUpUnderLoad` deploys a CPU-bound workload, generates HTTP
load, and asserts the HPA scales replicas above `minReplicas`. It is slow and needs metrics-server
on the cluster, so it is opt-in:

```bash
E2E_HPA_SCALE=true KUBECONFIG=... go test -tags e2e ./test/e2e/ -run HPAScalesUp -v -timeout 15m
```

**AWS greenfield test.** `TestAWSFullGreenfield` drives the `live/aws-full` two-phase Terraform
apply against a **real AWS account**, asserts the satellite came up (preflight verdict, install
tier), and tears down. It provisions a private EKS cluster + AWS Network Firewall + NAT (~20-30
min, real cost), so it is build-tagged `e2e_aws` (separate from the `e2e` cluster suite) and gated
on `E2E_AWS=true`. Auth uses an AWS profile (default `c3.test.aws`, which assumes the test role via
`~/.aws/config`); the test clears any ambient static `AWS_*` creds so the profile is used.

These tests drive a composition root under `live/` (consumer-owned scaffolding, not tracked — see
[`../docs/operations/aws/deploy.md`](../docs/operations/aws/deploy.md)). Provide the
`live/aws-full` root and its `terraform.tfvars` before running; the test skips/fails clearly if the
tfvars is absent.

```bash
mage preflightBuild                                                        # build the binary first
# author live/aws-full/terraform.tfvars (region, name, workload_spec_yaml, …)
E2E_AWS=true mage testE2EAWS
# or: E2E_AWS=true AWS_PROFILE=c3.test.aws go test -tags e2e_aws ./test/e2e/ -run TestAWSFullGreenfield -v -timeout 75m
```

See [`../docs/operations/aws/deploy.md`](../docs/operations/aws/deploy.md) for the manual
runbook the test automates.

### Runbooks

[`runbooks/`](./runbooks) holds manual, infra-dependent verification procedures (kind, and the
per-cloud greenfield applies added in later phases).
