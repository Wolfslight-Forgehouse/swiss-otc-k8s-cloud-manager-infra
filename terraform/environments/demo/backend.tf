terraform {
  backend "s3" {
    bucket = "YOUR-TFSTATE-BUCKET"
    key                         = "demo/terraform.tfstate"
    region                      = "eu-ch2"
    endpoint                    = "https://obs.eu-ch2.otc.t-systems.com"
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_metadata_api_check     = true
    force_path_style            = true
  }
}
