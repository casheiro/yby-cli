variable "cidr" {
  type = string
}

resource "aws_vpc" "this" {
  cidr_block = var.cidr
}

resource "aws_subnet" "a" {
  vpc_id            = aws_vpc.this.id
  availability_zone = data.aws_availability_zones.disponiveis.names[0]
  tags = {
    Name = "sub-${var.cidr}" // o nome vem do ambiente
  }
}

data "aws_availability_zones" "disponiveis" {}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "subnet_id" {
  value = aws_subnet.a.id
}
