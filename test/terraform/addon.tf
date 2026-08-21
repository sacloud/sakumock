# One resource per add-on family. Each family decodes the API's free-form
# `data` differently (SKU names, Front Door endpoints, search properties), so
# they are all exercised here rather than a single representative resource.

resource "sakura_addon_ai" "test" {
  location = "japaneast"
  sku      = 1
}

resource "sakura_addon_cdn" "test" {
  location      = "japaneast"
  pricing_level = 1
  patterns      = ["/*"]
  origin = {
    hostname    = "origin.example.com"
    host_header = "cdn.example.com"
  }
}

resource "sakura_addon_ddos" "test" {
  location      = "japaneast"
  pricing_level = 2
  patterns      = ["/*"]
  origin = {
    hostname    = "origin.example.com"
    host_header = "ddos.example.com"
  }
}

resource "sakura_addon_waf" "test" {
  location      = "japaneast"
  pricing_level = 1
  patterns      = ["/*", "/api/*"]
  origin = {
    hostname    = "origin.example.com"
    host_header = "waf.example.com"
  }
}

resource "sakura_addon_datalake" "test" {
  location    = "japaneast"
  performance = 1
  redundancy  = 4
}

resource "sakura_addon_dwh" "test" {
  location = "japaneast"
}

resource "sakura_addon_etl" "test" {
  location = "japaneast"
}

resource "sakura_addon_query" "test" {
  location = "japaneast"
}

resource "sakura_addon_search" "test" {
  location        = "japaneast"
  sku             = 3
  replica_count   = 2
  partition_count = 2
}

resource "sakura_addon_streaming" "test" {
  location   = "japaneast"
  unit_count = "30"
}
