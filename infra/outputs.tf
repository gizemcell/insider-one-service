output "public_ip" {
  value       = aws_eip.main.public_ip
  description = "Elastic IP — Public URL bunun üzerinden"
}

output "ssh_command" {
  value = "ssh -i ~/.ssh/insider-service ubuntu@${aws_eip.main.public_ip}"
}