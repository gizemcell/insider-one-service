#!/usr/bin/env bash
# EC2'ye SSH ile bağlandıktan sonra çalıştırılır.
# Docker + kubectl + minikube kurar, minikube'u başlatır.
set -euo pipefail

echo "==> Installing Docker and conntrack..."
sudo apt update
sudo apt install -y docker.io conntrack

echo "==> Adding ubuntu user to docker group..."
sudo usermod -aG docker ubuntu

echo "==> Installing minikube..."
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube
rm minikube-linux-amd64

echo "==> Installing kubectl..."
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install kubectl /usr/local/bin/
rm kubectl

echo "==> Starting minikube..."
# newgrp docker yerine sg docker -c kullan, script içinde çalışsın
sg docker -c "minikube start --driver=docker --memory=3072mb"

echo "==> Enabling ingress addon..."
sg docker -c "minikube addons enable ingress"

echo "==> Done. Verify with: kubectl get nodes"