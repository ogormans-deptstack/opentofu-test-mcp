variable "name" {
  type    = string
  default = "inspector"
}

resource "terraform_data" "example" {
  input = var.name
}

output "name" {
  value = var.name
}
