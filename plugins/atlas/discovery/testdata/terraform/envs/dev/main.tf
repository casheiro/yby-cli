# Ambiente de desenvolvimento: chama o módulo de rede compartilhado.
module "net" {
  source = "../../modules/net"
  cidr   = "10.0.0.0/16"
}

resource "aws_s3_bucket" "logs" {
  bucket = "logs-dev"
}

resource "aws_s3_bucket_policy" "logs" {
  bucket = aws_s3_bucket.logs.id
  # vpc = module.net.vpc_id (comentário não é referência)
  description = "texto que cita aws_instance.app.id não é referência"
}

/* resource "aws_s3_bucket" "antigo" {
  bucket = aws_s3_bucket.logs.id
} */

resource "aws_instance" "app" {
  subnet_id = module.net.subnet_id
  user_data = <<-EOT
    #!/bin/bash
    # aws_s3_bucket_policy.logs.id é texto do script
    echo "${aws_s3_bucket.logs.bucket}
  EOT
  tags = {
    Name = "app-{dev"
  }
}
