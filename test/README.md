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

### Runbooks

[`runbooks/`](./runbooks) holds manual, infra-dependent verification procedures (kind, and the
per-cloud greenfield applies added in later phases).
