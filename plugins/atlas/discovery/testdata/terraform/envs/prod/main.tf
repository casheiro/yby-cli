module "net" {
  source = "../../modules/net"
  cidr   = "10.1.0.0/16"
}

resource "aws_s3_bucket" "logs" {
  bucket = "logs-prod"
}
