# `cluster-resolver` module (AWS)

The single create-vs-lookup branch point for the EKS cluster. It emits a **uniform interface** —
`{endpoint, ca, auth}` — with identical shape whether the cluster was **provisioned** (the
`cluster` module's outputs fed through) or **brought by the customer** (looked up via
`data.aws_eks_cluster`). The root's `kubernetes`/`helm`/`kubectl` providers are configured from
these three outputs.

- **`endpoint`** is always the full `https://` URL; a bare host (a BYO edge case) is normalized to
  include the scheme inside the resolver.
- **`auth`** is a tagged object, defaulting to the EKS exec-plugin form (`aws eks get-token`). The
  providers invoke it at apply time to fetch a fresh token, avoiding the perpetual-diff / expiring-
  token problem of baking a static `aws_eks_cluster_auth` token into state. Consumers switch on
  `auth.kind` (`exec` today; `token` reserved for static-bearer-token cases).

This is what lets the greenfield root configure its Kubernetes providers identically whether it
provisioned the cluster or is deploying into a customer's existing one.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.60 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.60 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_eks_cluster.byo](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/eks_cluster) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cluster_name"></a> [cluster\_name](#input\_cluster\_name) | EKS cluster name. In provision mode the created cluster's name; in byo mode the existing cluster to look up. Used to fetch a fresh auth token in both modes. | `string` | n/a | yes |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" feeds the cluster module outputs through; "byo" looks up an existing EKS cluster via data sources. | `string` | n/a | yes |
| <a name="input_provisioned_ca"></a> [provisioned\_ca](#input\_provisioned\_ca) | Cluster CA data from the cluster module (provision mode). | `string` | `""` | no |
| <a name="input_provisioned_endpoint"></a> [provisioned\_endpoint](#input\_provisioned\_endpoint) | Cluster endpoint from the cluster module (provision mode). | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_auth"></a> [auth](#output\_auth) | Tagged auth object for the kubernetes/helm/kubectl providers. Shape: { kind = "exec"\|"token", exec = { api\_version, command, args }, token }. The AWS default is the EKS exec-plugin form (aws eks get-token), which avoids data.aws\_eks\_cluster\_auth token churn. Consumers switch on auth.kind. |
| <a name="output_ca"></a> [ca](#output\_ca) | Resolved base64 cluster CA data. |
| <a name="output_endpoint"></a> [endpoint](#output\_endpoint) | Resolved cluster API endpoint — the FULL https:// URL (bare host normalized inside the resolver). Identical shape in both modes. |
<!-- END_TF_DOCS -->
