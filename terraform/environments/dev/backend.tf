terraform {
  backend "s3" {
    bucket   = "rke2-sotc-tfstate"
    key      = "rke2/terraform.tfstate"
    region   = "eu-ch2"
    endpoint = "https://obs.eu-ch2.sc.otc.t-systems.com"

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
    force_path_style            = true
  }
}
