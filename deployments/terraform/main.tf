provider "digitalocean" {
  token = var.token
}

resource "digitalocean_project" "air_alert" {
  name        = "airalert.app"
  description = "Project for website airalert.app"
  purpose     = "Web Application"
  environment = "Production"
  resources   = [
    digitalocean_domain.air_alert.urn,
    data.digitalocean_loadbalancer.air_alert_load_balancer.urn
  ]
}

// Domains
resource "digitalocean_domain" "air_alert" {
  name = "airalert.app"
}

resource "digitalocean_record" "main" {
  domain  = digitalocean_domain.air_alert.name
  type    = "A"
  name    = "@"
  value   = data.digitalocean_loadbalancer.air_alert_load_balancer.ip
  ttl     = 35
}

resource "digitalocean_record" "mx" {
  domain    = digitalocean_domain.air_alert.name
  type      = "MX"
  name      = "@"
  priority  = 10
  value     = "mx.hover.com.cust.hostedemail.com."
  ttl       = var.default_ttl
}

resource "digitalocean_record" "mx_cname" {
  domain  = digitalocean_domain.air_alert.name
  type    = "CNAME"
  name    = "mail"
  value   = "mail.hover.com.cust.hostedemail.com."
  ttl     = var.default_ttl
}

// Kubernetes
resource "digitalocean_kubernetes_cluster" "air_alert_prod" {
  name    = "air-alert"
  region  = "sfo2"
  version = "1.18.8-do.1"
  
  node_pool {
    name        = "main"
    size        = "s-2vcpu-2gb"
    node_count  = 2
  }
}

provider "kubernetes" {
  load_config_file = false
  host  = digitalocean_kubernetes_cluster.air_alert_prod.endpoint
  token = digitalocean_kubernetes_cluster.air_alert_prod.kube_config[0].token
  cluster_ca_certificate = base64decode(
    digitalocean_kubernetes_cluster.air_alert_prod.kube_config[0].cluster_ca_certificate
  )
}

// Configuration maps
resource "kubernetes_config_map" "postgres_initdb_scripts" {
  metadata {
    name = "postgres-initdb-scripts"
  }

  data = {
    "init_airalert.sh" = file("${path.module}/../../scripts/init-airalert.sh")
  }
}

resource "kubernetes_config_map" "air_alert_config" {
  metadata {
    name = "airalert-config"
  }

  data = {
    "config.toml" = file("${path.module}/config/app.toml")
  }
}

// Persistent volumes
resource "kubernetes_persistent_volume_claim" "redis_pvc" {
  metadata {
    name = "redis-volume"
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "1Gi"
      }
    }
    storage_class_name = "do-block-storage"
  }
}

resource "kubernetes_persistent_volume_claim" "postgres_pvc" {
  metadata {
    name = "postgres-volume"
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        "storage" = "1Gi"
      }
    }
    storage_class_name = "do-block-storage"
  }
}

// Redis service
resource "kubernetes_stateful_set" "redis_datastore" {
  metadata {
    name = "redis-datastore"
    labels = {
      "app" = "redis"
    }
  }

  spec {
    replicas = 1
    service_name = "redis"
    selector {
      match_labels = {
        "app" = "redis"
      }
    }
    template {
      metadata {
        labels = {
          "app" = "redis"
        }
      }
      spec {
        container {
          name = "redis"
          image = "redis:6-alpine"
          args = ["--appendonly", "yes", "--save", "60", "10"]
          port {
            container_port = 6379
          }
          volume_mount {
            mount_path = "/data"
            name = "datastore"
          }
          resources {
            limits {
              memory = "128Mi"
              cpu = "250m"
            }
          }
        }
        volume {
          name = "datastore"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.redis_pvc.metadata[0].name
          }
        }
      }
    }

    update_strategy {
      type = "RollingUpdate"
    }
  }
}

resource "kubernetes_service" "redis_service" {
  metadata {
    name = "redis-service"
  }

  spec {
    selector = {
      "app" = kubernetes_stateful_set.redis_datastore.metadata[0].labels.app
    }
    port {
      protocol = "TCP"
      port = 6379
      target_port = "6379"
    }
  }
}

resource "kubernetes_config_map" "redis_service_config" {
  metadata {
    name = "redis-service-config"
  }

  data = {
    "redis-url" = "${kubernetes_service.redis_service.metadata[0].name}:6379"
  }
}

// Postgres service
resource "kubernetes_stateful_set" "postgres_userstore" {
  metadata {
    name = "postgres-userstore"
    labels = {
      "app" = "postgres"
    }
  }

  spec {
    replicas = 1
    service_name = "postgres"
    selector {
      match_labels = {
        "app" = "postgres"
      }
    }
    template {
      metadata {
        labels = {
          "app" = "postgres"
        }
      }
      spec {
        container {
          name = "postgres"
          image = "postgres:12-alpine"
          port {
            container_port = 5432
          }
          env {
            name = "AIR_ALERT_PGPASS"
            value_from {
              secret_key_ref {
                name = data.kubernetes_secret.airalert_secrets.metadata[0].name
                key = "pgpass"
              }
            }
          }
          env {
            name = "POSTGRES_PASSWORD"
            value_from {
              secret_key_ref {
                name = data.kubernetes_secret.airalert_secrets.metadata[0].name
                key = "pgrootpass"
              }
            }
          }
          env {
            name = "PGDATA"
            value = "/var/lib/postgresql/data/pgdata"
          }
          volume_mount {
            mount_path = "/docker-entrypoint-initdb.d"
            name = "init-scripts"
          }
          volume_mount {
            mount_path = "/var/lib/postgresql/data"
            name = "data"
          }
          resources {
            limits {
              memory = "256Mi"
              cpu = "250m"
            }
          }
        }
        volume {
          name = "init-scripts"
          config_map {
            name = kubernetes_config_map.postgres_initdb_scripts.metadata[0].name
            default_mode = "0744"
          }
        }
        volume {
          name = "data"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.postgres_pvc.metadata[0].name
          }
        }
      }
    }

    update_strategy {
      type = "RollingUpdate"
    }
  }
}

resource "kubernetes_service" "postgres_service" {
  metadata {
    name = "postgres-service"
  }

  spec {
    selector = {
      "app" = kubernetes_stateful_set.postgres_userstore.metadata[0].labels.app
    }
    port {
      protocol = "TCP"
      port = 5432
      target_port = "5432"
    }
  }
}

resource "kubernetes_config_map" "postgres_service_config" {
  metadata {
    name = "postgres-service-config"
  }

  data = {
    "postgres-url" = kubernetes_service.postgres_service.metadata[0].name
  }
}

// Air Alert service
resource "kubernetes_deployment" "air_alert_app" {
  metadata {
    name = "air-alert-app"
    labels = {
      "app" = "air-alert"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app" = "air-alert"
      }
    }
    template {
      metadata {
        labels = {
          "app" = "air-alert"
        }
      }
      spec {
        container {
          name = "air-alert"
          image = "mrflynn/air-alert:latest"
          args = ["-c", "/config/config.toml"]
          port {
            container_port = 3000
          }
          env {
            name = "AIR_ALERT_DATABASE_POSTGRES_HOST"
            value_from {
              config_map_key_ref {
                name = kubernetes_config_map.postgres_service_config.metadata[0].name
                key = "postgres-url"
              }
            }
          }
          env {
            name = "AIR_ALERT_DATABASE_POSTGRES_PASSWORD"
            value_from {
              secret_key_ref {
                name = data.kubernetes_secret.airalert_secrets.metadata[0].name
                key = "pgpass"
              }
            }
          }
          env {
            name = "AIR_ALERT_DATABASE_REDIS_ADDR"
            value_from {
              config_map_key_ref {
                name = kubernetes_config_map.redis_service_config.metadata[0].name
                key = "redis-url"
              }
            }
          }
          env {
            name = "AIR_ALERT_WEB_NOTIFICATIONS_PRIVATE_KEY"
            value_from {
              secret_key_ref {
                name = data.kubernetes_secret.airalert_secrets.metadata[0].name
                key = "vapid-private-key"
              }
            }
          }
          env {
            name = "AIR_ALERT_WEB_NOTIFICATIONS_PUBLIC_KEY"
            value_from {
              secret_key_ref {
                name = data.kubernetes_secret.airalert_secrets.metadata[0].name
                key = "vapid-public-key"
              }
            }
          }
          volume_mount {
            mount_path = "/config"
            name = "config-volume"
          }
          resources {
            limits {
              memory = "128Mi"
              cpu = "490m"
            }
          }
        }
        volume {
          name = "config-volume"
          config_map {
            name = kubernetes_config_map.air_alert_config.metadata[0].name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "air_alert_service" {
  metadata {
    name = "airalert-service"
  }

  spec {
    selector = {
      "app" = kubernetes_deployment.air_alert_app.metadata[0].labels.app
    }
    port {
      protocol = "TCP"
      port = 3000
      target_port = "3000"
    }
  }
}

// Ingress controller
resource "kubernetes_ingress" "air_alert_ingress" {
  metadata {
    name = "air-alert-ingress"
    annotations = {
      "kubernetes.io/ingress.class" = "nginx"
      "cert-manager.io/issuer" = "letsencrypt-prod"
    }
  }

  spec {
    tls {
      hosts = ["airalert.app"]
      secret_name = "air-alert-tls"
    }
    rule {
      host = "airalert.app"
      http {
        path {
          path = "/"
          backend {
            service_name = kubernetes_service.air_alert_service.metadata[0].name
            service_port = kubernetes_service.air_alert_service.spec[0].port[0].port
          }
        }
      }
    }
  }
}