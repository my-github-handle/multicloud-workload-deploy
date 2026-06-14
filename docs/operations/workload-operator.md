# Operating the Workload Operator

Component operations for the cloud-agnostic core. For the project-wide guide see
[`README.md`](./README.md); for design see [`../components/workload-operator.md`](../components/workload-operator.md).

---

## Installing & upgrading

```bash
# Install
helm install op charts/workload-operator -n workload-system --create-namespace \
  --set image.repository=<registry>/workload-operator --set image.tag=<version> \
  --set watchNamespace=<app-namespace>

# Upgrade (re-applies the CRD bundled under the chart's crds/ dir is NOT automatic on upgrade —
# apply CRD changes explicitly when the API changes):
kubectl apply -f charts/workload-operator/crds/
helm upgrade op charts/workload-operator -n workload-system --reuse-values --set image.tag=<new>
```

### Key values

| Value | Default | Notes |
|---|---|---|
| `image.repository` / `image.tag` | `ghcr.io/ops-dev/workload-operator` / `0.1.0` | operator image |
| `namespace` | `workload-system` | where the controller runs |
| `watchNamespace` | `""` (all) | the namespace it manages; RBAC is granted here |
| `imagePullSecrets` | `[]` | names of docker-registry secrets (set on the ServiceAccount) |
| `serviceMonitor.enabled` | `false` | requires the Prometheus Operator CRD |

---

## Managing workloads

Create / update / inspect:

```bash
kubectl apply -f docs/operations/examples/workload-basic.yaml
kubectl -n <ns> edit workload <name>
kubectl -n <ns> get workload <name> -o yaml
```

Watch the rollout:

```bash
kubectl -n <ns> get workload <name> -w
kubectl -n <ns> get deploy,hpa -l app.kubernetes.io/instance=<name>
```

---

## Observability

- **Operator metrics** — controller-runtime metrics on `:8080`; a metrics `Service` is shipped and,
  when `serviceMonitor.enabled=true`, scraped by the Prometheus Operator.
- **Workload status** — `.status.conditions` (`Ready`, `RolloutDegraded`) and `.status.readyReplicas`
  are the primary signals.
- **Operator logs** — `kubectl -n workload-system logs deploy/workload-operator`. Start-up errors
  are logged before exit; pass `--dev` for human-friendly logs in development.

---

## Troubleshooting

| Symptom | Likely cause | Action |
|---|---|---|
| No child objects created; operator logs `forbidden` on the watched namespace | RBAC not in the watched namespace | confirm `watchNamespace` matches the workload's namespace; the Role/RoleBinding must exist there |
| Pods `CreateContainerConfigError`: "runAsNonRoot and image will run as root" | image runs as root vs hardened default | set `spec.podSecurityContext.runAsNonRoot: false` (and `spec.securityContext` as needed) |
| Pods `CrashLoopBackOff`, logs show read-only filesystem errors | image needs to write to its root fs | set `spec.securityContext.readOnlyRootFilesystem: false`, or mount writable scratch |
| `Workload` create rejected: "maxReplicas must be ≥ minReplicas" | invalid autoscale bounds | fix `spec.autoscale` |
| `RolloutDegraded=True / CanaryUnsupported` | `rolloutStrategy: Canary` requested | expected — Canary degrades to RollingUpdate until the rollout component lands |
| `ImagePullBackOff` | private image without a reachable pull secret | create the pull secret and set `imagePullSecrets` (operator) / ensure node pull creds (workload) |
| `readyReplicas` stuck at 0 | pods not becoming ready | check pod events/logs; verify probes and resources |

---

## Uninstall

```bash
# A single workload (children cascade-delete via owner references):
kubectl -n <ns> delete workload <name>

# The operator (leaves existing Workloads orphaned until the CRD is removed):
helm uninstall op -n workload-system

# Full removal — deletes ALL Workloads on the cluster:
kubectl delete crd workloads.workload.ops.dev
```
