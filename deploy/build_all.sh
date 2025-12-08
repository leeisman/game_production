#!/bin/bash
set -e # Exit on error

echo "🚀 Starting Full Build & Deploy Pipeline..."
CLUSTER_NAME="game-cluster"

# 1. Build Docker Images
# We use the generic Dockerfile defined in root
echo "🔨 Building Docker Images..."

# Helper function to build and load
build_and_load() {
    SERVICE_NAME=$1
    APP_PATH=$2
    IMAGE_NAME="game_product/$SERVICE_NAME:latest"
    
    echo "  - Building $SERVICE_NAME ($APP_PATH)..."
    docker build -t $IMAGE_NAME -f Dockerfile --build-arg APP_PATH=$APP_PATH .
    
    echo "  - Loading $IMAGE_NAME into Kind..."
    kind load docker-image $IMAGE_NAME --name $CLUSTER_NAME
}

# Build all microservices
build_and_load "user" "cmd/color_game/microservices/user"
build_and_load "gms" "cmd/color_game/microservices/gms"
build_and_load "gs" "cmd/color_game/microservices/gs"
build_and_load "gateway" "cmd/color_game/microservices/gateway"
build_and_load "ops" "cmd/ops"

# 2. Deploy to K8s
echo "📦 Deploying Applications to Kubernetes..."

# Apply App Deployments (This includes Ingress)
kubectl apply -f deploy/k8s/app/

# Restart deployments to pick up new images (since we use 'latest' tag)
echo "🔄 Rolling Restarting Deployments..."
kubectl rollout restart deployment user-service
kubectl rollout restart deployment gms-service
kubectl rollout restart deployment gs-service
kubectl rollout restart deployment gateway-service
kubectl rollout restart deployment ops-service

echo "✅ All Done! Use 'kubectl get pods' or k9s to check status."
echo "🌍 Access User API: http://localhost/api/user"
echo "🌍 Access Gateway WS: ws://localhost/ws"
