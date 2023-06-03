
terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.64.0"
    }
  }
}

provider "google" {
  project = var.project
  region  = var.region
  zone    = var.zone
}

locals {
  enable_services = toset([
    "cloudbuild.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "compute.googleapis.com",
    "run.googleapis.com",
    "spanner.googleapis.com",
    "secretmanager.googleapis.com",
    "vpcaccess.googleapis.com",
    "redis.googleapis.com",
    "certificatemanager.googleapis.com",
    "artifactregistry.googleapis.com",
  ])
}

resource "google_project_service" "compute_service" {
  service = "compute.googleapis.com"
}

resource "google_project_service" "service" {
  for_each = local.enable_services
  project  = var.project
  service  = each.value
  timeouts {
    create = "60m"
    update = "120m"
  }
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_spanner_instance" "test_instance" {
  display_name     = "test-instance"
  name             = "test-instance"
  config           = "regional-${var.region}"
  processing_units = 100
  labels = {
    "environment" = "development"
  }
  depends_on = [
    google_project_service.service
  ]
}

resource "google_spanner_database" "database" {
  instance = google_spanner_instance.test_instance.name
  name     = "game"
  version_retention_period = "3d"
  ddl = [
  ]
  deletion_protection = false
  depends_on = [
    google_spanner_instance.test_instance
  ]
}

resource "google_cloud_run_service" "game_api" {
  name     = "game-api"
  provider = google-beta
  location = var.region

  template {
    spec {
      containers {
        image = "us-docker.pkg.dev/cloudrun/container/hello"
        resources {
          limits = {
            cpu    = "1000m"
            memory = "1028M"
          }
        }
      }
      service_account_name = google_service_account.run_sa.email
    }
  }
  autogenerate_revision_name = true
  depends_on                 = [google_project_service.service]
}

resource "google_compute_network" "game_vpc" {
  name                    = "my-network"
  auto_create_subnetworks = true
  # mtu = 1460
  depends_on = [
    google_project_service.service
  ]
}

resource "google_vpc_access_connector" "game-api-vpc-access" {
  name           = "game-api-vpc-access"
  provider       = google
  region         = var.region
  ip_cidr_range  = "10.8.0.0/28"
  max_throughput = 300
  network        = google_compute_network.game_vpc.name
  depends_on     = [google_project_service.service]
}

resource "google_redis_instance" "test_redis" {
  authorized_network      = google_compute_network.game_vpc.id
  connect_mode            = "DIRECT_PEERING"
  location_id             = var.zone
  memory_size_gb          = 1
  name                    = "test-redis"
  project                 = var.project
  read_replicas_mode      = "READ_REPLICAS_DISABLED"
  redis_version           = "REDIS_6_X"
  region                  = var.region
  tier                    = "BASIC"
  transit_encryption_mode = "DISABLED"
}

resource "google_cloud_run_service_iam_binding" "run_iam_binding" {
  location = google_cloud_run_service.game_api.location
  project  = google_cloud_run_service.game_api.project
  service  = google_cloud_run_service.game_api.name
  role     = "roles/run.invoker"
  members = [
    "allUsers",
  ]
}

resource "google_service_account" "run_sa" {
  account_id = "game-api"
}

resource "google_project_iam_member" "binding_run_sa" {
  role    = "roles/spanner.databaseUser"
  member  = "serviceAccount:${google_service_account.run_sa.email}"
  project = var.project
}

resource "google_compute_region_network_endpoint_group" "run_neg" {
  name                  = "run-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  cloud_run {
    service = google_cloud_run_service.game_api.name
  }
  depends_on = [
    google_project_service.compute_service,
    google_cloud_run_service.game_api
  ]
}

resource "google_compute_global_address" "reserved_ip" {
  name = "reserverd-ip"
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_compute_backend_service" "run_backend" {
  name = "run-backend"

  protocol    = "HTTP"
  port_name   = "http"
  timeout_sec = 30

  backend {
    group = google_compute_region_network_endpoint_group.run_neg.id
  }
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_compute_url_map" "run_url_map" {
  name = "run-url-map"

  default_service = google_compute_backend_service.run_backend.id
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_compute_target_https_proxy" "run_https_proxy" {
  name = "run-https-proxy"

  url_map = google_compute_url_map.run_url_map.id
  certificate_map = "//certificatemanager.googleapis.com/projects/${var.project}/locations/global/certificateMaps/cert-map"
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_compute_global_forwarding_rule" "run_lb" {
  name = "rub-lb"

  target     = google_compute_target_https_proxy.run_https_proxy.id
  port_range = "443"
  ip_address = google_compute_global_address.reserved_ip.address
  depends_on = [
    google_project_service.compute_service
  ]
}

resource "google_bigquery_dataset" "my_dataset" {
  dataset_id                  = "my_dataset"
  friendly_name               = "my_dataset"
  location                    = "US"
}

resource "google_logging_project_sink" "logging_to_bq" {
  name = "logging-to-bq"

  destination = "bigquery.googleapis.com/projects/${var.project}/datasets/${google_bigquery_dataset.my_dataset.dataset_id}"

  filter = "resource.type=\"cloud_run_revision\" AND resource.labels.configuration_name=\"game-api\" AND jsonPayload.message!=\"\""

  unique_writer_identity = true
}

resource "google_project_iam_binding" "log_writer" {
  project = var.project
  role    = "roles/bigquery.dataEditor"

  members = [
    google_logging_project_sink.logging_to_bq.writer_identity,
  ]
}

resource "google_certificate_manager_certificate" "test" {
  name        = "dns-cert"
  description = "test cert"
  managed {
    domains = [
      google_certificate_manager_dns_authorization.test.domain,
      ]
    dns_authorizations = [
      google_certificate_manager_dns_authorization.test.id,
      ]
  }
  depends_on = [
    google_project_service.service,
  ]
}

resource "google_certificate_manager_dns_authorization" "test" {
  name        = "dns-auth"
  description = "The default dnss"
  domain      = var.domain
  depends_on = [
    google_project_service.service,
  ]
}

resource "google_certificate_manager_certificate_map" "test" {
  name        = "cert-map"
  description = "My acceptance test certificate map"
  labels      = {
    "terraform" : true,
  }
  depends_on = [
    google_project_service.service,
  ]
}

resource "google_certificate_manager_certificate_map_entry" "test" {
  name        = "cert-map-entry"
  description = "My acceptance test certificate map entry"
  map = google_certificate_manager_certificate_map.test.name
  labels      = {
    "terraform" : true,
  }
  certificates = [google_certificate_manager_certificate.test.id]
  matcher = "PRIMARY"
  depends_on = [
    google_certificate_manager_certificate_map.test,
  ]
}

output "dns_auth" {
    value = google_certificate_manager_dns_authorization.test.dns_resource_record
}

output "external_ip_attached_to_gclb" {
  value = google_compute_global_address.reserved_ip.address
}

output "url_cloud_run_embeded" {
  value = google_cloud_run_service.game_api.status[0].url
}

output "redis_private_ip_in_vpc" {
  value = "${google_redis_instance.test_redis.host}:${google_redis_instance.test_redis.port}"
}

output "url_for_service" {
  value = "https://${var.domain}"
}