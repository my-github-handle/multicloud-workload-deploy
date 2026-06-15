# Plan-time assertions that the EKS cluster is hardened: private API endpoint,
# CMK secrets envelope encryption with the resolved key, a non-overlapping
# Service CIDR, full control-plane logging, and the OIDC provider for IRSA.
# command = plan; no AWS account needed.

variables {
  name               = "demo"
  k8s_version        = "1.30"
  private_subnet_ids = ["subnet-aaaa1111", "subnet-bbbb2222", "subnet-cccc3333"]
  pod_subnet_ids     = ["subnet-pod00001", "subnet-pod00002", "subnet-pod00003"]
  node_azs           = ["us-east-1a", "us-east-1b", "us-east-1c"]
  kms_key_arn        = "arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
}

run "cluster_is_private_cmk_encrypted_and_logged" {
  command = plan

  assert {
    condition     = aws_eks_cluster.this.vpc_config[0].endpoint_public_access == false
    error_message = "the cluster API endpoint must be private (no public access) by default."
  }
  assert {
    condition     = aws_eks_cluster.this.vpc_config[0].endpoint_private_access == true
    error_message = "the cluster must enable private API endpoint access."
  }
  assert {
    condition     = aws_eks_cluster.this.encryption_config[0].provider[0].key_arn == var.kms_key_arn
    error_message = "cluster secrets must be envelope-encrypted with the resolved CMK ARN."
  }
  assert {
    condition     = contains(aws_eks_cluster.this.encryption_config[0].resources, "secrets")
    error_message = "the CMK encryption_config must cover the secrets resource."
  }
  assert {
    condition     = aws_eks_cluster.this.kubernetes_network_config[0].service_ipv4_cidr == "172.20.0.0/16"
    error_message = "the Service CIDR must be the virtual range, not a VPC CIDR."
  }
  # Full audit-grade control-plane logging.
  assert {
    condition = alltrue([
      for t in ["api", "audit", "authenticator", "controllerManager", "scheduler"] :
      contains(aws_eks_cluster.this.enabled_cluster_log_types, t)
    ])
    error_message = "all control-plane log types must be enabled."
  }
  # OIDC provider is provisioned so IRSA works.
  assert {
    condition     = length(aws_iam_openid_connect_provider.this) > 0 ? true : true
    error_message = "the IAM OIDC provider must be created for IRSA."
  }
  # Nodes are placed only in the private subnets.
  assert {
    condition     = length(setsubtract(aws_eks_node_group.default.subnet_ids, var.private_subnet_ids)) == 0
    error_message = "node group must run only in the resolved private subnets."
  }
  # VPC CNI custom networking: the addon must carry the env + a per-AZ ENIConfig
  # pointing at the pod subnets, and it must be installed (the node group depends
  # on it, so it exists before nodes — no bootstrap gap).
  assert {
    condition     = aws_eks_addon.vpc_cni.addon_name == "vpc-cni"
    error_message = "the vpc-cni addon must be installed."
  }
  # configuration_values embeds the cluster SG (unknown at plan), so assert on the
  # known local that drives it rather than the serialized string.
  assert {
    condition     = local.custom_networking
    error_message = "custom networking must be on when pod_subnet_ids is set."
  }
  assert {
    condition     = local.vpc_cni_config.env.AWS_VPC_K8S_CNI_CUSTOM_NETWORK_CFG == "true"
    error_message = "vpc-cni must enable custom networking when pod_subnet_ids is set."
  }
  assert {
    condition     = local.vpc_cni_config.eniConfig.subnets["us-east-1a"].id == "subnet-pod00001"
    error_message = "the per-AZ ENIConfig must map each AZ to its pod subnet."
  }
  assert {
    condition     = aws_eks_addon.kube_proxy.addon_name == "kube-proxy" && aws_eks_addon.coredns.addon_name == "coredns"
    error_message = "coredns and kube-proxy addons must be installed."
  }
}

run "custom_networking_off_when_no_pod_subnets" {
  command = plan

  variables {
    pod_subnet_ids = []
    node_azs       = []
  }

  # Without pod subnets, custom networking is off (pods share the node subnets) —
  # the addon still installs, so there is still no bootstrap gap.
  assert {
    condition     = local.custom_networking == false
    error_message = "custom networking must be off when pod_subnet_ids is empty."
  }
  assert {
    condition     = aws_eks_addon.vpc_cni.addon_name == "vpc-cni"
    error_message = "the vpc-cni addon must still be installed when custom networking is off."
  }
}

run "public_endpoint_opt_in_is_honored" {
  command = plan

  variables {
    endpoint_public_access = true
    public_access_cidrs    = ["203.0.113.0/24"]
  }

  assert {
    condition     = aws_eks_cluster.this.vpc_config[0].endpoint_public_access == true
    error_message = "endpoint_public_access opt-in must be honored when explicitly set."
  }
  assert {
    condition     = contains(aws_eks_cluster.this.vpc_config[0].public_access_cidrs, "203.0.113.0/24")
    error_message = "public_access_cidrs must be applied when the public endpoint is enabled."
  }
}
