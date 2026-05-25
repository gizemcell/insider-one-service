variable "aws_region" {
  type        = string
  description = "AWS region for all resources"
  default     = "eu-central-1"
}

variable "project_name" {
  type        = string
  description = "Used as prefix for tags and resource names"
  default     = "insider-service"
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type. Minikube needs >= 2 vCPU and >= 4 GB RAM"
  default     = "c7i-flex.large"
}

variable "ssh_public_key_path" {
  type        = string
  description = "Path to SSH public key uploaded to the EC2 keypair"
  default     = "~/.ssh/insider-service.pub"
}

variable "my_ip_cidr" {
  type        = string
  description = "Source IP in CIDR notation (e.g. X.X.X.X/32) allowed for SSH"
  # No default — must be provided via terraform.tfvars
}