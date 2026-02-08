output "namespace" {
  value = var.namespace
}

output "server_url" {
  value       = length(var.ingress_hosts) > 0 ? "https://${var.ingress_hosts[0].host}" : ""
  description = "URL to reach the Argo CD server (ingress host)."
}
