"""Generate the AWS component & data-flow architecture diagram.

A component/data-flow view (not a deployment/apply flow): it shows the AWS
building-block components, the satellite running on them, and how a workload's
runtime traffic and data move between them. Renders official AWS service icons
via the `diagrams` library (Graphviz backend). Run inside the project's diagram
container (see render.sh) — no host installs.

Output: aws_infra.png in this directory.
"""

from diagrams import Diagram, Cluster, Edge
from diagrams.aws.network import NATGateway, InternetGateway, NetworkFirewall
from diagrams.aws.security import KMS, SecretsManager, IAMRole
from diagrams.aws.compute import EKS
from diagrams.aws.storage import SimpleStorageServiceS3Bucket as S3
from diagrams.aws.management import Cloudwatch
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
PURPLE = "#7B42BC"   # identity (IRSA) + secret material
RED_K = "#DD3522"    # encryption (KMS)
ORANGE = "#D86613"   # in-cluster reconcile / serving

with Diagram(
    "Multi-Cloud Workload Deploy  —  AWS Component & Data-Flow Architecture",
    filename="aws_infra",
    outformat="png",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    with Cluster("AWS Account / Region"):

        with Cluster("VPC   primary 10.0.0.0/24  (edge)   +   secondary 100.64.0.0/16  (data plane)"):

            # ---- Per-AZ HA swimlanes: edge tier ----
            with Cluster("Edge tier — primary CIDR (one per AZ)"):
                igw = InternetGateway("Internet Gateway")
                with Cluster("AZ-a"):
                    nat_a = NATGateway("NAT")
                    fw_a = NetworkFirewall("Firewall ENI")
                with Cluster("AZ-b"):
                    nat_b = NATGateway("NAT")
                    fw_b = NetworkFirewall("Firewall ENI")
                with Cluster("AZ-c"):
                    nat_c = NATGateway("NAT")
                    fw_c = NetworkFirewall("Firewall ENI")
                nats = [nat_a, nat_b, nat_c]

            # ---- Data plane: EKS + VPC CNI custom networking + the Layer-3 satellite ----
            with Cluster("Data plane — secondary CIDR (nodes + pods, per AZ)"):
                eks = EKS("EKS\nprivate · OIDC/IRSA\nCMK envelope enc\nsvc CIDR 172.20.0.0/16")
                cilium = NetworkPolicy("VPC CNI custom networking\npods on secondary CIDR\n+ optional Cilium chaining")

                with Cluster("Layer-3 satellite (Tier A)"):
                    operator = Deploy("workload-operator")
                    netpol = NetworkPolicy("default-deny\n+ metadata-IP block")
                    pods = Pod("Workload pods\n(real secondary-CIDR IPs)")
                    svc = SVC("Service")
                    operator >> Edge(color=ORANGE, label="reconciles") >> pods
                    pods - Edge(color=GREY, style="dashed") - svc
                    netpol - Edge(color=GREY, style="dotted") - pods

                eks >> Edge(color=PURPLE) >> cilium >> Edge(color=PURPLE) >> pods

            # ---- Default-deny egress data path: pod -> same-AZ firewall -> NAT -> IGW ----
            pods >> Edge(label="0.0.0.0/0 (same-AZ)", color=RED) >> fw_a
            fw_a >> Edge(label="allowlist pass", color=GREEN) >> nat_a
            fw_b >> Edge(color=GREEN) >> nat_b
            fw_c >> Edge(color=GREEN) >> nat_c
            for nat in nats:
                nat >> Edge(color=GREY) >> igw

        # ---- Encryption & identity ----
        with Cluster("Encryption & Identity"):
            kms = KMS("KMS CMK\nrotation enabled")
            iam = IAMRole("IRSA\nwildcard-free\ndeploy + runtime")
            sm = SecretsManager("Secrets Manager\nCMK envelope enc")
            kms >> Edge(style="dashed", color=RED_K) >> sm
            kms >> Edge(style="dashed", color=RED_K) >> eks

        # ---- Always-on, immutable audit floor ----
        with Cluster("Audit floor — always-on, immutable"):
            flowlogs = S3("VPC Flow Logs\nS3 · COMPLIANCE\nObject-Lock")
            cw = Cloudwatch("EKS control-plane\nlogging")

    # ---- Cross-cutting data flows ----
    fw_a >> Edge(label="ALL traffic", style="dashed", color=GREY) >> flowlogs
    eks >> Edge(style="dashed", color=GREY) >> cw
    pods >> Edge(label="IRSA: scoped KMS + Secrets", style="dotted", color=PURPLE) >> iam
    pods >> Edge(style="dotted", color=PURPLE) >> sm
