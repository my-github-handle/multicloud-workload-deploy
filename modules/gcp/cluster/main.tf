# GKE database encryption requires the GKE service agent
# (service-<project-number>@container-engine-robot.iam.gserviceaccount.com) to
# hold roles/cloudkms.cryptoKeyEncrypterDecrypter on the CMEK key BEFORE the
# cluster is created, or `database_encryption` fails the cluster create. Grant it
# here and order the cluster AFTER the grant via depends_on.
resource "google_kms_crypto_key_iam_member" "gke_robot" {
  crypto_key_id = var.kms_key_id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:service-${var.project_number}@container-engine-robot.iam.gserviceaccount.com"
}

# Hardened private GKE via the maintained module. Private nodes (no public IPs) +
# private endpoint, Workload Identity, shielded nodes, database secrets encryption
# with the CMK, release channel, full control-plane logging/monitoring, and
# Dataplane V2 (= Cilium → native NetworkPolicy). Pinned to ~> 33.0, which exposes
# the monitoring_enable_observability_metrics / monitoring_enable_observability_relay
# inputs used below.
#
# AVD-GCP-0057 is a false positive here: node_metadata is GKE_METADATA_SERVER and
# Workload Identity is enabled, so metadata concealment is on; trivy does not
# resolve the value through the module call.
# trivy:ignore:AVD-GCP-0057 metadata concealment is enabled (GKE_METADATA + Workload Identity)
module "gke" {
  source  = "terraform-google-modules/kubernetes-engine/google//modules/beta-private-cluster"
  version = "~> 33.0"

  project_id = var.project_id
  name       = var.name
  region     = var.region
  regional   = true

  # The module wants the network/subnetwork NAMES, not self-links; the resolver
  # emits self-links, so take the last path segment.
  network           = element(split("/", var.network_self_link), length(split("/", var.network_self_link)) - 1)
  subnetwork        = element(split("/", var.subnet_self_link), length(split("/", var.subnet_self_link)) - 1)
  ip_range_pods     = var.pods_range_name
  ip_range_services = var.services_range_name

  # Private cluster: nodes have no public IPs; the control-plane endpoint is private.
  # Nodes are always private (no public IPs). The control-plane endpoint is
  # private by default; enable_private_endpoint can be flipped to false for
  # testing, in which case master_authorized_networks restricts the public
  # endpoint to a CIDR allowlist.
  enable_private_nodes       = true
  enable_private_endpoint    = var.enable_private_endpoint
  master_ipv4_cidr_block     = var.master_ipv4_cidr_block
  master_authorized_networks = var.master_authorized_networks
  # The endpoint output follows the reachable endpoint: private in-VPC by default,
  # public when the endpoint is flipped for testing.
  deploy_using_private_endpoint = var.enable_private_endpoint

  # Dataplane V2 = Cilium, so Cilium + Kubernetes NetworkPolicy are native; no
  # separate Cilium install is needed. network_policy MUST be false with
  # ADVANCED_DATAPATH: setting it true enables the Calico network-policy addon,
  # which is mutually exclusive with Dataplane V2. DPv2 enforces NetworkPolicy
  # itself.
  datapath_provider = "ADVANCED_DATAPATH"
  network_policy    = false

  # Workload Identity enabled (the iam module binds the KSA → GSA through it).
  # Set to PROJECT.svc.id.goog to enable Workload Identity.
  identity_namespace = "${var.project_id}.svc.id.goog"

  # Metadata concealment: pods reach the GKE metadata server, never the raw VM
  # metadata endpoint, so node credentials cannot be read by pods.
  node_metadata = "GKE_METADATA_SERVER"

  # Shielded nodes (secure boot + integrity monitoring).
  enable_shielded_nodes = true

  # Database/application-layer secrets encryption at rest with the resolved CMK.
  # Requires the robot-SA grant above.
  database_encryption = [{
    state    = "ENCRYPTED"
    key_name = var.kms_key_id
  }]

  release_channel    = var.release_channel
  kubernetes_version = var.k8s_version

  # Full control-plane logging + monitoring (audit-grade).
  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"

  remove_default_node_pool = true

  node_pools = [{
    name         = "default"
    machine_type = var.node_machine_type
    min_count    = var.node_min_count
    max_count    = var.node_max_count
    auto_repair  = true
    auto_upgrade = true
    # Shielded node options on the pool.
    enable_secure_boot          = true
    enable_integrity_monitoring = true
  }]

  # Dataplane V2 (Hubble-grade) flow/identity/L7 observability:
  # monitoring_enable_observability_metrics enables advanced-datapath metrics,
  # monitoring_enable_observability_relay enables the Hubble relay, and
  # enable_cilium_clusterwide_network_policy turns on the
  # CiliumClusterwideNetworkPolicy CRD.
  enable_cilium_clusterwide_network_policy = true
  monitoring_enable_observability_metrics  = true
  monitoring_enable_observability_relay    = true

  # GKE-managed Secret Manager CSI driver add-on. Without the Secrets-Store-CSI
  # driver + GCP provider present on the cluster, the secrets module's
  # SecretProviderClass apply fails on a clean greenfield cluster. This managed
  # add-on installs them.
  enable_secret_manager_addon = var.enable_secret_manager_csi_addon

  # Always carry at least a managed-by label so the cluster is identifiable in
  # asset inventory; merged with any caller-supplied labels.
  cluster_resource_labels = merge({ "managed-by" = "workload-operator" }, var.labels)

  # Cluster create must wait for the CMEK robot-SA grant.
  depends_on = [google_kms_crypto_key_iam_member.gke_robot]
}

# Fail fast on invalid node-pool autoscaling bounds rather than late in the GKE
# API call.
resource "terraform_data" "node_bounds" {
  input = "${var.node_min_count}:${var.node_max_count}"
  lifecycle {
    precondition {
      condition     = var.node_min_count >= 0 && var.node_min_count <= var.node_max_count
      error_message = "Node pool bounds must satisfy 0 <= node_min_count <= node_max_count."
    }
  }
}
