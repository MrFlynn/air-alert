variable "token" {}

variable "default_ttl" {
  type = number
  default = 60
}

data "kubernetes_secret" "airalert_secrets" {
  metadata {
    name = "airalert-secrets"
  }
}

data "digitalocean_loadbalancer" "air_alert_load_balancer" {
  name = "ae15b928cd801480198311bdf3426246"
}