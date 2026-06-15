"""Generate the GCP component & data-flow architecture diagram.

A component/data-flow view (not a deployment/apply flow): it shows the GCP
building-block components, the satellite running on them, and how a workload's
runtime traffic and data move between them. Renders official GCP service icons
via the `diagrams` library (Graphviz backend). Run inside the project's diagram
container (see README) — no host installs.

Output: gcp_infra.png in this directory.
"""

from diagrams import Diagram, Cluster, Edge
from diagrams.gcp.network import Router, NAT, FirewallRules
from diagrams.gcp.security import KMS, Iam, SecretManager
from diagrams.gcp.compute import GKE
from diagrams.gcp.storage import GCS
from diagrams.gcp.operations import Logging
from diagrams.k8s.compute import Pod, Deploy
from diagrams.k8s.network import NetworkPolicy, SVC

graph_attr = {
    "fontsize": "18",
    "labelloc": "t",
    "fontname": "Helvetica",
    "pad": "0.7",
    "nodesep": "0.5",
    "ranksep": "1.1",
    "bgcolor": "white",
}

node_attr = {"fontname": "Helvetica", "fontsize": "11"}
edge_attr = {"fontname": "Helvetica", "fontsize": "10"}

# Edge colors by data-flow kind.
RED = "#CC0000"      # default-deny egress path
GREEN = "#1A8A1A"    # allowed / pass
GREY = "#6A6A6A"     # plumbing / telemetry
PURPLE = "#7B42BC"   # identity (Workload Identity) + secret material
RED_K = "#DD3522"    # encryption (Cloud KMS / CMEK)
ORANGE = "#D86613"   # in-cluster reconcile / serving

with Diagram(
    "Multi-Cloud Workload Deploy  —  GCP Component & Data-Flow Architecture",
    filename="gcp_infra",
    outformat="png",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    with Cluster("GCP Project   (dedicated, or BYO)   ·   required service APIs enabled"):

        with Cluster("VPC   data plane 100.64.0.0/16   (nodes + pod/service alias ranges)"):

            # ---- Controlled egress edge: Cloud NAT + VPC firewall policy ----
            with Cluster("Controlled egress edge"):
                fw = FirewallRules("VPC firewall policy\ndefault-deny egress\n+ FQDN/CIDR allowlist")
                router = Router("Cloud Router")
                nat = NAT("Cloud NAT\n(no public node IPs)")

            # ---- Data plane: GKE + Dataplane V2 + the Layer-3 satellite ----
            with Cluster("Data plane — private GKE"):
                gke = GKE("GKE\nprivate · Workload Identity\nshielded · CMEK db enc\nmetadata concealment")
                dpv2 = NetworkPolicy("Dataplane V2 (= Cilium)\nnative NetworkPolicy\n+ Hubble observability")

                with Cluster("Layer-3 satellite (Tier A)"):
                    operator = Deploy("workload-operator")
                    netpol = NetworkPolicy("default-deny\n+ metadata-IP block")
                    pods = Pod("Workload pods\n(pod alias IPs)")
                    svc = SVC("Service")
                    operator >> Edge(color=ORANGE, label="reconciles") >> pods
                    pods - Edge(color=GREY, style="dashed") - svc
                    netpol - Edge(color=GREY, style="dotted") - pods

                gke >> Edge(color=PURPLE) >> dpv2 >> Edge(color=PURPLE) >> pods

            # ---- Default-deny egress data path: pod -> firewall -> Cloud NAT ----
            pods >> Edge(label="0.0.0.0/0", color=RED) >> fw
            fw >> Edge(label="allowlist pass", color=GREEN) >> router
            router >> Edge(color=GREY) >> nat

        # ---- Encryption & identity ----
        with Cluster("Encryption & Identity"):
            kms = KMS("Cloud KMS CryptoKey\nrotation enabled")
            iam = Iam("Workload Identity\nwildcard-free\ndeploy + runtime roles")
            sm = SecretManager("Secret Manager\nCMEK envelope enc")
            kms >> Edge(style="dashed", color=RED_K) >> sm
            kms >> Edge(style="dashed", color=RED_K) >> gke

        # ---- Always-on, immutable audit floor ----
        with Cluster("Audit floor — always-on, immutable"):
            flowlogs = GCS("VPC Flow Logs\nGCS · locked\nretention policy")
            logging = Logging("GKE control-plane\nlogging/monitoring")

    # ---- Cross-cutting data flows ----
    fw >> Edge(label="ALL traffic", style="dashed", color=GREY) >> flowlogs
    gke >> Edge(style="dashed", color=GREY) >> logging
    pods >> Edge(label="WI: scoped KMS + Secrets", style="dotted", color=PURPLE) >> iam
    pods >> Edge(style="dotted", color=PURPLE) >> sm
