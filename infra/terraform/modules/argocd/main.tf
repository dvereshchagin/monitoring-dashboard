locals {
  default_ingress_annotations = {
    "alb.ingress.kubernetes.io/scheme"            = "internet-facing"
    "alb.ingress.kubernetes.io/target-type"      = "ip"
    "alb.ingress.kubernetes.io/listen-ports"     = "[{\"HTTP\":80},{\"HTTPS\":443}]"
    "alb.ingress.kubernetes.io/ssl-redirect"     = "443"
  }

  normalized_ingress_hosts = [
    for host in var.ingress_hosts : {
      host  = host.host
      paths = try(host.paths, [
        {
          path     = "/"
          pathType = "Prefix"
        }
      ])
    }
  ]

  ingress_tls_hosts = length(var.ingress_tls_secret_hosts) > 0 ? var.ingress_tls_secret_hosts : [for host in local.normalized_ingress_hosts : host.host]

  ingress_config = length(local.normalized_ingress_hosts) > 0 ? {
    ingress = {
      enabled       = true
      ingressClassName = var.ingress_class_name
      annotations   = merge(local.default_ingress_annotations, var.ingress_annotations)
      hosts         = [
        for host in local.normalized_ingress_hosts : {
          host  = host.host
          paths = [
            for path in host.paths : {
              path     = path.path
              pathType = path.pathType
            }
          ]
        }
      ]
      tls = var.ingress_tls_secret_name != "" && length(local.normalized_ingress_hosts) > 0 ? [
        {
          hosts      = length(local.ingress_tls_hosts) > 0 ? local.ingress_tls_hosts : [for host in local.normalized_ingress_hosts : host.host]
          secretName = var.ingress_tls_secret_name
        }
      ] : []
    }
  } : {}

  base_server = merge({
    service = {
      type = var.server_service_type
    }
  }, local.ingress_config)

  repo_config = var.repo_url != "" ? [
    {
      url            = var.repo_url
      type           = "git"
      usernameSecret = "${var.repo_secret_name}:username"
      passwordSecret = "${var.repo_secret_name}:password"
    }
  ] : []

  argocd_values = merge({
    server = local.base_server
    configs = {
      repositories = local.repo_config
    }
  }, var.extra_values)

  helm_version = var.chart_version != "" ? var.chart_version : null
}

resource "helm_release" "this" {
  name             = var.release_name
  namespace        = var.namespace
  repository       = var.chart_repository
  chart            = "argo-cd"
  version          = local.helm_version
  create_namespace = true
  timeout          = 180
  values           = [yamlencode(local.argocd_values)]
}
