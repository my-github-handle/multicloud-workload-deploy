# --- VPC network + subnet w/ secondary ranges for GKE pods/services ---
# Battle-tested module; do not hand-roll. VPC Flow Logs are enabled on the
# subnet itself (log_config) so the audit floor is at the subnet level.
module "vpc" {
  source  = "terraform-google-modules/network/google"
  version = "~> 9.0"

  project_id   = var.project_id
  network_name = "${var.name}-vpc"
  routing_mode = "REGIONAL"

  subnets = [
    {
      subnet_name               = "${var.name}-subnet"
      subnet_ip                 = var.subnet_cidr
      subnet_region             = var.region
      subnet_private_access     = "true" # Private Google Access: nodes reach Google APIs w/o public IPs.
      subnet_flow_logs          = "true"
      subnet_flow_logs_interval = "INTERVAL_5_SEC"
      subnet_flow_logs_sampling = "0.5"
      subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
    }
  ]

  secondary_ranges = {
    "${var.name}-subnet" = [
      { range_name = "${var.name}-pods", ip_cidr_range = var.pods_cidr },
      { range_name = "${var.name}-services", ip_cidr_range = var.services_cidr },
    ]
  }
}

# --- Private DNS for restricted Private Google Access ---
# Allowing the restricted VIP (199.36.153.4/30) in the firewall is not enough on
# its own: nodes must also RESOLVE Google API domains to that VIP, or they get
# public IPs that default-deny drops — and a private GKE cluster's nodes then
# never reach Artifact Registry / the control plane add-ons and never go Ready.
# These private managed zones map the Google API domains to the restricted VIP
# for the VPC. private.googleapis.com would be 199.36.153.8/30; we use the
# restricted VIP to keep egress on the VPC-SC-eligible surface.
locals {
  restricted_vip_ips = ["199.36.153.4", "199.36.153.5", "199.36.153.6", "199.36.153.7"]
  # Domains that must resolve to the restricted VIP for a private cluster to work:
  # googleapis.com (all Google APIs), gcr.io + pkg.dev (image pull), plus their
  # apex A records.
  google_api_zones = {
    googleapis = { dns_name = "googleapis.com.", apex = "googleapis.com." }
    gcrio      = { dns_name = "gcr.io.", apex = "gcr.io." }
    pkgdev     = { dns_name = "pkg.dev.", apex = "pkg.dev." }
  }
}

resource "google_dns_managed_zone" "google_apis" {
  for_each = local.google_api_zones

  project     = var.project_id
  name        = "${var.name}-${each.key}"
  dns_name    = each.value.dns_name
  description = "Restricted Private Google Access for ${each.value.dns_name} (maps to the restricted VIP)."
  visibility  = "private"

  private_visibility_config {
    networks {
      network_url = module.vpc.network_self_link
    }
  }
}

# Wildcard CNAME -> the zone apex (e.g. *.googleapis.com -> googleapis.com).
resource "google_dns_record_set" "wildcard_cname" {
  for_each = local.google_api_zones

  project      = var.project_id
  managed_zone = google_dns_managed_zone.google_apis[each.key].name
  name         = "*.${each.value.dns_name}"
  type         = "CNAME"
  ttl          = 300
  rrdatas      = [each.value.apex]
}

# Apex A record -> the restricted VIP addresses.
resource "google_dns_record_set" "apex_a" {
  for_each = local.google_api_zones

  project      = var.project_id
  managed_zone = google_dns_managed_zone.google_apis[each.key].name
  name         = each.value.apex
  type         = "A"
  ttl          = 300
  rrdatas      = local.restricted_vip_ips
}

# --- Cloud Router + Cloud NAT for controlled, no-public-IP egress ---
resource "google_compute_router" "this" {
  name    = "${var.name}-router"
  project = var.project_id
  region  = var.region
  network = module.vpc.network_self_link
}

resource "google_compute_router_nat" "this" {
  name    = "${var.name}-nat"
  project = var.project_id
  region  = var.region
  router  = google_compute_router.this.name

  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ALL"
  }
}

# --- Customer-owned, retention-locked GCS bucket for VPC Flow Logs ---
# The always-on audit floor: CNI-independent, survives cluster compromise,
# immutable via a locked retention policy.
#
# Encryption is Google-managed by design, NOT the workload CMK: the audit floor
# must remain readable even if the workload CMK is destroyed or the cluster is
# compromised. Binding flow-log integrity to the workload key would let a CMK
# deletion render the audit trail unreadable — the opposite of an audit floor.
# trivy:ignore:AVD-GCP-0066 audit floor must not depend on the workload CMK
resource "google_storage_bucket" "flow_logs" {
  name                        = "${var.name}-vpc-flow-logs-${var.project_number}"
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  # Retention-locked: objects cannot be deleted/overwritten before retention
  # elapses; retention_policy.is_locked makes the policy itself immutable.
  retention_policy {
    retention_period = var.flow_log_retention_days * 24 * 60 * 60
    is_locked        = true
  }

  versioning {
    enabled = true
  }

  labels = var.labels
}

# Logging sink: route subnet VPC Flow Logs into the retention-locked bucket.
resource "google_logging_project_sink" "flow_logs" {
  name        = "${var.name}-vpc-flow-logs-sink"
  project     = var.project_id
  destination = "storage.googleapis.com/${google_storage_bucket.flow_logs.name}"

  # Only VPC Flow Log entries for this subnet.
  filter = "resource.type=\"gce_subnetwork\" AND logName=\"projects/${var.project_id}/logs/compute.googleapis.com%2Fvpc_flows\""

  unique_writer_identity = true
}

# Grant the sink's writer identity objectCreator on the bucket so it can write.
resource "google_storage_bucket_iam_member" "flow_logs_writer" {
  bucket = google_storage_bucket.flow_logs.name
  role   = "roles/storage.objectCreator"
  member = google_logging_project_sink.flow_logs.writer_identity
}

# --- VPC firewall policy: FQDN + CIDR egress allowlist, default-deny egress ---
# A network firewall policy associated with the VPC. Priority order (lowest
# number = highest priority): explicit ALLOW rules for the GKE control-plane
# (890), intra-VPC ranges (895), Google APIs via the restricted VIP (900), and
# DNS via the metadata server (910) — all REQUIRED for a private cluster to
# function at all under default-deny — then the permitted FQDNs (1000) and CIDRs
# (1100), then a lowest-priority DENY-all-egress catch at 65000 (default-deny).
# This is the perimeter FQDN backstop that holds regardless of the cluster CNI.
#
# IMPORTANT — `dest_fqdns` capability gating: FQDN-match rules in a *global*
# network firewall policy require Cloud NGFW and may require the Enterprise tier
# (with a firewall endpoint / security profile) in some configurations. If a
# bare network firewall policy rejects `dest_fqdns` at apply, the deployment
# still has a working perimeter via the CIDR allow rules below + the in-cluster
# NetworkPolicy/Cilium toFQDNs layer — but verify `dest_fqdns` is accepted on
# this policy type under the pinned provider/org before relying on it as the
# sole FQDN backstop.
resource "google_compute_network_firewall_policy" "egress" {
  name    = "${var.name}-egress-policy"
  project = var.project_id
}

# GKE private control-plane: nodes MUST reach the master endpoint at
# master_ipv4_cidr_block, or kubelet/node registration fails and the cluster
# bricks under default-deny. Highest priority.
resource "google_compute_network_firewall_policy_rule" "egress_control_plane" {
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 890
  enable_logging  = true

  match {
    dest_ip_ranges = [var.master_ipv4_cidr_block]
    layer4_configs {
      ip_protocol = "tcp"
      ports       = ["443", "10250"]
    }
  }
}

# Intra-VPC: pod-to-pod, pod-to-node, and pod-to-service traffic (subnet + pod +
# service CIDRs) MUST be allowed under default-deny or in-cluster networking
# breaks. All protocols within the VPC ranges.
resource "google_compute_network_firewall_policy_rule" "egress_intra_vpc" {
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 895
  enable_logging  = true

  match {
    dest_ip_ranges = var.intra_vpc_cidrs
    layer4_configs {
      ip_protocol = "all"
    }
  }
}

# Google APIs via Private Google Access through the RESTRICTED VIP
# (199.36.153.4/30 — restricted.googleapis.com): nodes have no public IPs and
# reach Artifact Registry, KMS, Secret Manager, Logging, and the Workload
# Identity token endpoint through it. Under default-deny these MUST be explicitly
# allowed or the cluster cannot pull images, decrypt, fetch secrets, or even
# register its nodes — a silent brick. The speculative anycast 34.126.0.0/18 is
# intentionally NOT in the default set.
resource "google_compute_network_firewall_policy_rule" "egress_google_apis" {
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 900
  enable_logging  = true

  match {
    dest_ip_ranges = var.google_api_cidrs
    layer4_configs {
      ip_protocol = "tcp"
      ports       = ["443"]
    }
  }
}

# DNS egress (UDP+TCP 53) routed via the GCE METADATA SERVER (169.254.169.254),
# which forwards to Cloud DNS — NOT public resolvers. FQDN-match rules can only
# resolve names if DNS itself is permitted. Public 8.8.8.8/8.8.4.4 are
# deliberately NOT allowed (DNS via metadata server / Cloud DNS, keeping egress
# on-Google and the FQDN backstop coherent).
resource "google_compute_network_firewall_policy_rule" "egress_dns" {
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 910
  enable_logging  = true

  match {
    dest_ip_ranges = ["169.254.169.254/32"]
    layer4_configs {
      ip_protocol = "udp"
      ports       = ["53"]
    }
    layer4_configs {
      ip_protocol = "tcp"
      ports       = ["53"]
    }
  }
}

resource "google_compute_network_firewall_policy_rule" "egress_fqdn_allow" {
  count           = length(var.egress_allowed_fqdns) > 0 ? 1 : 0
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 1000
  enable_logging  = true

  match {
    dest_fqdns = var.egress_allowed_fqdns
    layer4_configs {
      ip_protocol = "tcp"
      ports       = ["443"]
    }
  }
}

resource "google_compute_network_firewall_policy_rule" "egress_cidr_allow" {
  count           = length(var.egress_allowed_cidrs) > 0 ? 1 : 0
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "allow"
  priority        = 1100
  enable_logging  = true

  match {
    dest_ip_ranges = var.egress_allowed_cidrs
    layer4_configs {
      ip_protocol = "tcp"
    }
  }
}

# Default-deny: lowest priority, matches all remaining egress and drops it.
resource "google_compute_network_firewall_policy_rule" "egress_default_deny" {
  project         = var.project_id
  firewall_policy = google_compute_network_firewall_policy.egress.name
  direction       = "EGRESS"
  action          = "deny"
  priority        = 65000
  enable_logging  = true

  match {
    dest_ip_ranges = ["0.0.0.0/0"]
    layer4_configs {
      ip_protocol = "all"
    }
  }
}

resource "google_compute_network_firewall_policy_association" "egress" {
  name              = "${var.name}-egress-assoc"
  project           = var.project_id
  firewall_policy   = google_compute_network_firewall_policy.egress.name
  attachment_target = module.vpc.network_id
}

locals {
  subnet_self_link = module.vpc.subnets_self_links[0]
  subnet_id        = module.vpc.subnets_ids[0]
}
