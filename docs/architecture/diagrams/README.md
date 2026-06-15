# Architecture diagrams

Diagrams are defined as code (`*.py`, using the [`diagrams`](https://diagrams.mingrammer.com/)
library) and rendered to committed `*.png` artifacts with official cloud-provider icons. The
`.py` source is the source of truth; regenerate the `.png` whenever the source changes.

## Files

| Source | Output | Shows |
|---|---|---|
| `aws_infra.py` | `aws_infra.png` | AWS component & data-flow architecture (`aws-full` greenfield): VPC primary/secondary CIDR split, per-AZ HA edge tier, EKS data plane with Cilium ENI mode, the Layer-3 satellite, encryption/identity, and the always-on audit floor. |
| `gcp_infra.py` | `gcp_infra.png` | GCP component & data-flow architecture (`gcp-full` greenfield): the project container, VPC data plane with Cloud NAT + default-deny firewall policy, private GKE with Dataplane V2 (Cilium native), the Layer-3 satellite, Cloud KMS / Secret Manager / Workload Identity, and the retention-locked flow-log audit floor. |

## Rendering

Rendering needs Graphviz and the `diagrams` package. To avoid host installs, render inside a
container:

```bash
cd docs/architecture/diagrams
docker run --rm -v "$PWD":/work -w /work python:3.12-slim bash -c '
  apt-get update -qq && apt-get install -y -qq graphviz
  pip install --quiet diagrams
  python aws_infra.py    # or gcp_infra.py
'
```

The rendered `*.png` is written next to the source and committed to the repo so readers need no
tooling to view it.
