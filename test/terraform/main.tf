terraform {
  required_providers {
    sakura = {
      source  = "sacloud/sakura"
      version = "~> 3"
    }
  }
}

# Endpoints and dummy credentials come from the environment (`sakumock env`,
# e.g. `eval "$(sakumock env --export)"`). tk1v is SAKURA Cloud's sandbox zone
# (used for testing); the mock ignores the zone regardless.
provider "sakura" {
  zone = "tk1v"
}
