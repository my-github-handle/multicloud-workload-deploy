variable "mode" {
  description = "\"provision\" creates a new project and enables the required service APIs; \"byo\" resolves an existing customer project and ensures the same APIs are enabled (the BYOC path — customers commonly pre-create the project)."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "project_id" {
  description = "Project ID. In provision mode this is the ID for the project to create; in byo mode the existing project to resolve."
  type        = string
}

variable "project_name" {
  description = "Human-readable project display name (provision mode). Defaults to project_id when empty."
  type        = string
  default     = ""
}

variable "billing_account" {
  description = "Billing account id to link the created project to (provision mode), e.g. \"0X0X0X-0X0X0X-0X0X0X\". Required in provision mode; ignored in byo mode."
  type        = string
  default     = ""
}

variable "org_id" {
  description = "Organization id to create the project under (provision mode). Mutually exclusive with folder_id; leave both empty for a project with no parent."
  type        = string
  default     = ""
}

variable "folder_id" {
  description = "Folder id to create the project under (provision mode). Mutually exclusive with org_id."
  type        = string
  default     = ""

  # Fail fast at the input boundary rather than on a provider error: a project
  # cannot have both an org and a folder parent.
  validation {
    condition     = var.org_id == "" || var.folder_id == ""
    error_message = "org_id and folder_id are mutually exclusive; set at most one."
  }
}

variable "activate_apis" {
  description = "Service APIs to enable on the project. These are the APIs every building block in this product needs (GKE, Compute, Cloud KMS, Secret Manager, IAM + credentials, Logging, Monitoring, Artifact Registry, Resource Manager, Service Usage). Enabled in BOTH modes so a BYOC project is brought up to the required baseline."
  type        = list(string)
  default = [
    "compute.googleapis.com",
    "container.googleapis.com",
    "cloudkms.googleapis.com",
    "secretmanager.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "serviceusage.googleapis.com",
    "dns.googleapis.com",
  ]
}

variable "disable_services_on_destroy" {
  description = "Whether `terraform destroy` disables the enabled APIs. Default false: on a BYO project we must NOT disable APIs the customer may rely on elsewhere; on a provisioned project the whole project is destroyed anyway."
  type        = bool
  default     = false
}

variable "labels" {
  description = "Labels applied to the provisioned project."
  type        = map(string)
  default     = {}
}
