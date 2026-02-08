variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "namespace" {
  type    = string
  default = "argocd"
}

variable "release_name" {
  type    = string
  default = "argocd"
}

variable "chart_repository" {
  type    = string
  default = "https://argoproj.github.io/argo-helm"
}

variable "chart_version" {
  type    = string
  default = ""
}

variable "repo_url" {
  type = string

  validation {
    condition     = length(var.repo_url) > 0
    error_message = "repo_url must be configured for the GitOps repository."
  }
}

variable "repo_secret_name" {
  type    = string
  default = "argocd-repo-creds"
}

variable "server_service_type" {
  type    = string
  default = "ClusterIP"
}

variable "ingress_class_name" {
  type    = string
  default = "alb"
}

variable "ingress_hosts" {
  type = list(object({
    host  = string
    paths = optional(list(object({
      path     = string
      pathType = string
    })))
  }))
  default = []
}

variable "ingress_annotations" {
  type    = map(string)
  default = {}
}

variable "ingress_tls_secret_name" {
  type    = string
  default = ""
}

variable "ingress_tls_secret_hosts" {
  type    = list(string)
  default = []
}

variable "extra_values" {
  type    = map(any)
  default = {}
}
