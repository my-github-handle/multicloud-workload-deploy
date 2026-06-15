# Verify _agnostic-deploy on kind (end-to-end, Tier A + red-gate)

Prereqs: the operator image is built/loadable (`mage dockerBuild` or pull
`ghcr.io/<owner>/workload-operator:<tag>`), the preflight binary is built
(`mage preflightBuild` → `operator/bin/preflight`), and `kind`, `kubectl`, `helm`,
`terraform >= 1.7` are installed.

## 1. Create a kind cluster with metrics-server (HPA needs it)

```bash
kind create cluster --name agnostic
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

## 2. Install the ServiceMonitor CRD (so the ServiceMonitor applies) OR disable observability

```bash
kubectl apply --server-side -f \
  https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.74.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml
# If you skip this, set observability_enabled = false in step 5.
```

## 3. Load the operator image into kind

```bash
docker build --platform linux/amd64 -f operator/Dockerfile -t ghcr.io/ops-dev/workload-operator:dev .
kind load docker-image ghcr.io/ops-dev/workload-operator:dev --name agnostic
```

## 4. Export the kind kubeconfig to a file the providers + preflight read

```bash
kind get kubeconfig --name agnostic > /tmp/agnostic.kubeconfig
```

## 5. Write terraform.tfvars

The workload spec is a single YAML document. nginx runs as root, writes to its filesystem, and
needs a few capabilities, so this example relaxes the namespace PSA to `baseline` and supplies a
security context in the spec. A non-root image keeps the default `restricted` PSA and needs none of
that.

In your BYOC composition root (a small root that wires the Layer-3 modules — copy the reference
composition into your IaC repo):

```bash
cat > terraform.tfvars <<EOF
kubeconfig_path  = "/tmp/agnostic.kubeconfig"
preflight_binary = "$(git rev-parse --show-toplevel)/operator/bin/preflight"
namespace        = "workload-system"
operator_image_tag = "dev"
observability_enabled = true
# nginx is root + writes its filesystem → baseline PSA (restricted forbids runAsNonRoot=false).
psa_enforce_level = "baseline"
# On a rate-limited cluster (kind) disable the in-apply readiness wait; confirm readiness below.
workload_wait_for_ready = false
workload_name = "demo"
workload_spec_yaml = <<-YAML
  image: nginx:1.27
  port: 80
  autoscale:
    minReplicas: 2
    maxReplicas: 5
    targetCPUUtilization: 70
  podSecurityContext:
    runAsNonRoot: false
    seccompProfile:
      type: RuntimeDefault
  securityContext:
    readOnlyRootFilesystem: false
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]
      # nginx's entrypoint chowns its cache/pid dirs and binds :80.
      add: ["CHOWN", "SETUID", "SETGID", "DAC_OVERRIDE", "NET_BIND_SERVICE"]
YAML
EOF
```

## 6. One terraform apply (the BYOC primary success criterion)

```bash
terraform init
terraform apply -auto-approve
```

Expected: apply succeeds. `terraform output preflight_verdict` is `green` or `amber`
(kind grants the test identity cluster-admin, so install-tier resolves to Tier A → green).
`terraform output install_tier` is `A`.

**CRD-Established gate.** The Workload CR apply must not race the CRD registration.
`helm_release wait=true` on the operator chart only waits for the chart's own resources,
NOT for the api-server to report the CRD `Established`. The workload module's
`kubectl_manifest` is ordered after the operator via the root `depends_on`. If the single
apply ever fails with `no matches for kind "Workload"`, insert this gate between the
operator install and the CR apply, then re-apply:

```bash
kubectl wait --for=condition=established crd/workloads.workload.ops.dev --timeout=120s
```

## 7. Assert the Workload + child objects came up (Tier A)

```bash
kubectl wait --for=condition=Ready workload/demo -n workload-system --timeout=180s
kubectl get workload demo -n workload-system \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'   # -> True
kubectl get deploy,svc,hpa,pdb demo -n workload-system            # all four present
kubectl get networkpolicy -n workload-system                      # default-deny + egress-allow
kubectl get servicemonitor workload-operator -n workload-system   # present
```

Confirm the operator pod scheduled under enforce=restricted and the metrics Service is present:

```bash
kubectl get ns workload-system \
  -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}'  # -> restricted
kubectl get pods -n workload-system -l app.kubernetes.io/name=workload-operator \
  -o jsonpath='{.items[0].status.phase}'                                  # -> Running
kubectl get svc -n workload-system -l app.kubernetes.io/name=workload-operator \
  -o jsonpath='{.items[0].spec.ports[?(@.name=="metrics")].name}'         # -> metrics
```

## 8. Prove a RED preflight verdict blocks apply

```bash
# Standalone confirm the binary can report red for an undeployable target, using a kubeconfig
# whose identity lacks deployments/namespaces create:
"$(git rev-parse --show-toplevel)/operator/bin/preflight" \
  --mode=agnostic --kubeconfig <restricted.kubeconfig> --namespace demo | jq .verdict   # -> "red"
# With that restricted kubeconfig in terraform.tfvars:
terraform plan
```

Expected: `terraform plan` (or apply) FAILS at the preflight module with
`Preflight verdict is RED — deployment blocked. Report: {...}` and NO downstream resources
are planned/created.

## 9. Cleanup

```bash
terraform destroy -auto-approve
kind delete cluster --name agnostic
```
