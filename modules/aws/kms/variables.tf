variable "mode" {
  description = "\"provision\" creates a new CMK with rotation; \"byo\" resolves a customer-supplied key ARN."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "alias" {
  description = "KMS alias name (without the alias/ prefix) for the provisioned key."
  type        = string
  default     = "workload-cmk"
}

variable "enable_rotation" {
  description = "Enable automatic annual key rotation (provision mode)."
  type        = bool
  default     = true
}

variable "deletion_window_days" {
  description = "Waiting period before a scheduled key deletion completes (provision mode)."
  type        = number
  default     = 30
}

variable "provided_key_arn" {
  description = "Existing CMK ARN to resolve (byo mode). Ignored in provision mode."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to the provisioned key."
  type        = map(string)
  default     = {}
}
