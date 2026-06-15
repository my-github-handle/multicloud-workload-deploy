data "aws_partition" "current" {}
data "aws_region" "current" {}

# --- Cluster IAM role ---
data "aws_iam_policy_document" "cluster_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  name               = "${var.name}-cluster"
  assume_role_policy = data.aws_iam_policy_document.cluster_assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSClusterPolicy"
}

# Grant the cluster role use of the resolved CMK for secrets envelope encryption.
# The kms module provisions the key with the default key policy (IAM delegation
# allowed), so this attached policy is sufficient; if the key ever carries an
# explicit locked-down policy, that policy must also grant this role.
data "aws_iam_policy_document" "cluster_kms" {
  statement {
    effect = "Allow"
    actions = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:ListGrants",
      "kms:DescribeKey",
      "kms:CreateGrant",
    ]
    resources = [var.kms_key_arn]
  }
}

resource "aws_iam_role_policy" "cluster_kms" {
  name   = "${var.name}-cluster-kms"
  role   = aws_iam_role.cluster.id
  policy = data.aws_iam_policy_document.cluster_kms.json
}

# --- The EKS control plane: private, CMK-encrypted, fully logged ---
# The cluster API endpoint is private by default (no internet exposure). Secrets
# are envelope-encrypted at rest with the resolved CMK. Service ClusterIPs come
# from service_ipv4_cidr, which must not overlap either VPC CIDR.
resource "aws_eks_cluster" "this" {
  name                      = var.name
  version                   = var.k8s_version
  role_arn                  = aws_iam_role.cluster.arn
  enabled_cluster_log_types = var.enabled_cluster_log_types

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = var.endpoint_public_access
    public_access_cidrs     = var.endpoint_public_access ? var.public_access_cidrs : null
  }

  kubernetes_network_config {
    service_ipv4_cidr = var.service_ipv4_cidr
    ip_family         = "ipv4"
  }

  encryption_config {
    provider {
      key_arn = var.kms_key_arn
    }
    resources = ["secrets"]
  }

  tags = var.tags

  depends_on = [
    aws_iam_role_policy_attachment.cluster,
    aws_iam_role_policy.cluster_kms,
  ]
}

# --- IRSA OIDC provider ---
data "tls_certificate" "oidc" {
  url = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "this" {
  client_id_list  = ["sts.${data.aws_partition.current.dns_suffix}"]
  thumbprint_list = [data.tls_certificate.oidc.certificates[0].sha1_fingerprint]
  url             = aws_eks_cluster.this.identity[0].oidc[0].issuer
  tags            = merge(var.tags, { Name = "${var.name}-irsa" })
}

# --- Node IAM role ---
data "aws_iam_policy_document" "node_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "node" {
  name               = "${var.name}-node"
  assume_role_policy = data.aws_iam_policy_document.node_assume.json
  tags               = var.tags
}

# Standard worker node policies. AmazonEKS_CNI_Policy grants the VPC CNI the EC2
# ENI/IP allocation it needs (including the secondary ENIs custom networking
# creates); ECR read lets nodes pull images.
resource "aws_iam_role_policy_attachment" "node" {
  for_each = toset([
    "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])
  role       = aws_iam_role.node.name
  policy_arn = each.value
}

# --- VPC CNI addon with custom networking (installed BEFORE the node group) ---
# The VPC CNI is the default EKS CNI, so nodes reach Ready as soon as it is
# present — there is no bootstrap gap. Custom networking is configured here via
# configuration_values: the addon sets the aws-node env and creates one ENIConfig
# per AZ, so pods draw secondary ENIs from the pod subnets (secondary CIDR) while
# the node primary ENI stays in the node subnet. ENABLE_PREFIX_DELEGATION recovers
# the max-pods lost when the primary ENI no longer serves pods.
locals {
  custom_networking = length(var.pod_subnet_ids) > 0

  # Built unconditionally but only serialized into the addon when custom_networking
  # is true (see configuration_values below). The subnets map is empty when there
  # are no pod subnets, so this is safe to evaluate either way.
  vpc_cni_config = {
    env = {
      AWS_VPC_K8S_CNI_CUSTOM_NETWORK_CFG = "true"
      ENI_CONFIG_LABEL_DEF               = "topology.kubernetes.io/zone"
      ENABLE_PREFIX_DELEGATION           = "true"
      WARM_PREFIX_TARGET                 = "3"
    }
    eniConfig = {
      create = true
      region = data.aws_region.current.name
      subnets = {
        for i, az in var.node_azs : az => {
          id             = var.pod_subnet_ids[i]
          securityGroups = [aws_eks_cluster.this.vpc_config[0].cluster_security_group_id]
        }
      }
    }
  }
}

resource "aws_eks_addon" "vpc_cni" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "vpc-cni"
  addon_version               = var.vpc_cni_addon_version != "" ? var.vpc_cni_addon_version : null
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "OVERWRITE"
  configuration_values        = local.custom_networking ? jsonencode(local.vpc_cni_config) : null
  tags                        = var.tags
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "kube-proxy"
  addon_version               = var.kube_proxy_addon_version != "" ? var.kube_proxy_addon_version : null
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "OVERWRITE"
  tags                        = var.tags
}

# --- Managed node group (private subnets only) ---
# Ordered AFTER the VPC CNI addon so the CNI + per-AZ ENIConfig exist before the
# first node — pods then land on the pod subnets from the start, and nodes are
# Ready immediately (the VPC CNI is present, no bootstrap gap).
resource "aws_eks_node_group" "default" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "default"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.private_subnet_ids
  instance_types  = var.node_instance_types

  scaling_config {
    min_size     = var.node_min_size
    max_size     = var.node_max_size
    desired_size = var.node_desired_size
  }

  tags = var.tags

  # Fail fast on invalid scaling bounds rather than late in the EKS API call.
  lifecycle {
    precondition {
      condition     = var.node_min_size <= var.node_desired_size && var.node_desired_size <= var.node_max_size
      error_message = "Node group scaling bounds must satisfy node_min_size <= node_desired_size <= node_max_size."
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.node,
    aws_eks_addon.vpc_cni,
    aws_eks_addon.kube_proxy,
  ]
}

# coredns schedules on nodes, so it is ordered AFTER the node group.
resource "aws_eks_addon" "coredns" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "coredns"
  addon_version               = var.coredns_addon_version != "" ? var.coredns_addon_version : null
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "OVERWRITE"
  tags                        = var.tags

  depends_on = [aws_eks_node_group.default]
}
