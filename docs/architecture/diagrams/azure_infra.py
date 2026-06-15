"""Generate the Azure component & data-flow architecture diagram.

A component/data-flow view (not a deployment/apply flow): it shows the Azure
building-block components, the satellite running on them, and how a workload's
runtime traffic and data move between them. Renders official Azure service icons
via the `diagrams` library (Graphviz backend). Run inside the project's diagram
container (see README) — no host installs.

Output: azure_infra.png in this directory.
"""

from diagrams import Diagram, Cluster, Edge
from diagrams.azure.network import VirtualNetworks, Firewall, RouteTables
from diagrams.azure.network import LoadBalancers as NATGateway
from diagrams.azure.security import KeyVaults
from diagrams.azure.identity import ManagedIdentities
from diagrams.azure.compute import AKS
from diagrams.azure.storage import BlobStorage
from diagrams.azure.analytics import LogAnalyticsWorkspaces
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
RED = "#CC0000"      # default-deny egress path (UDR)
GREEN = "#1A8A1A"    # allowed / pass
GREY = "#6A6A6A"     # plumbing / telemetry
PURPLE = "#7B42BC"   # identity (Workload ID) + secret material
RED_K = "#DD3522"    # encryption (Key Vault key)
ORANGE = "#D86613"   # in-cluster reconcile / serving

with Diagram(
    "Multi-Cloud Workload Deploy  —  Azure Component & Data-Flow Architecture",
    filename="azure_infra",
    outformat="png",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    with Cluster("Azure Subscription / Region"):

        with Cluster("VNet  10.0.0.0/16  (nodes only — pods use the CNI overlay, not VNet IPs)"):

            # ---- Edge tier: firewall + NAT ----
            with Cluster("Edge — AzureFirewallSubnet + NAT"):
                fw = Firewall("Azure Firewall\nFQDN/CIDR allowlist\ndefault-deny")
                nat = NATGateway("NAT Gateway")
                udr = RouteTables("UDR\n0.0.0.0/0 → firewall")

            # ---- Data plane: AKS + CNI Overlay (Cilium dataplane) + satellite ----
            with Cluster("Data plane — node subnet + Cilium overlay"):
                aks = AKS("AKS\nprivate · OIDC / Workload ID\nCMK disk enc · svc 172.20.0.0/16")
                cni = NetworkPolicy("Azure CNI Overlay\nCilium dataplane\npods on 100.64.0.0/16 (overlay)")

                with Cluster("Layer-3 satellite (Tier A)"):
                    operator = Deploy("workload-operator")
                    netpol = NetworkPolicy("default-deny\n+ metadata-IP block")
                    pods = Pod("Workload pods\n(overlay IPs)")
                    svc = SVC("Service")
                    operator >> Edge(color=ORANGE, label="reconciles") >> pods
                    pods - Edge(color=GREY, style="dashed") - svc
                    netpol - Edge(color=GREY, style="dotted") - pods

                aks >> Edge(color=PURPLE) >> cni >> Edge(color=PURPLE) >> pods

            # ---- Default-deny egress data path: pod -> UDR -> firewall -> NAT ----
            pods >> Edge(label="0.0.0.0/0", color=RED) >> udr
            udr >> Edge(color=RED) >> fw
            fw >> Edge(label="allowlist pass", color=GREEN) >> nat

        # ---- Encryption & identity ----
        with Cluster("Encryption & Identity"):
            kv = KeyVaults("Key Vault\nkey rotation · purge protect\n+ CMK-encrypted secrets")
            uami = ManagedIdentities("UAMI + federated cred\nwildcard-free\ndeploy + runtime role")
            kv >> Edge(style="dashed", color=RED_K, label="disk enc set") >> aks

        # ---- Always-on, immutable audit floor ----
        with Cluster("Audit floor — always-on, immutable"):
            flowlogs = BlobStorage("VNet Flow Logs\nBlob · time-based WORM")
            law = LogAnalyticsWorkspaces("AKS control-plane\ndiagnostics (kube-audit)")

    # ---- Cross-cutting data flows ----
    fw >> Edge(label="ALL traffic", style="dashed", color=GREY) >> flowlogs
    aks >> Edge(style="dashed", color=GREY) >> law
    pods >> Edge(label="Workload ID: scoped Key Vault", style="dotted", color=PURPLE) >> uami
    pods >> Edge(label="CSI mount", style="dotted", color=PURPLE) >> kv
