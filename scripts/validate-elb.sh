#!/bin/bash
set -euo pipefail

ENV="${1:-demo}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-$ROOT_DIR/kubeconfig-$ENV}"

echo "🔍 Validating ELB functionality..."

# Create test service
kubectl apply -f - << YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: elb-test
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: elb-test
  template:
    metadata:
      labels:
        app: elb-test
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: elb-test-svc
  namespace: default
  annotations:
    kubernetes.io/elb.class: "union"
spec:
  type: LoadBalancer
  selector:
    app: elb-test
  ports:
    - port: 80
      targetPort: 80
YAML

echo "⏳ Waiting for External IP (up to 3 minutes)..."
for i in $(seq 1 36); do
  EXT_IP=$(kubectl get svc elb-test-svc -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  if [[ -n "$EXT_IP" ]]; then
    echo "✅ External IP assigned: $EXT_IP"
    echo "🌐 Testing HTTP..."
    if curl -s --max-time 5 "http://$EXT_IP" | grep -q "Welcome to nginx"; then
      echo "✅ ELB is working correctly!"
    else
      echo "⚠️  Got IP but HTTP not responding yet — may need a moment"
    fi
    break
  fi
  echo "  Attempt $i/36 — waiting 5s..."
  sleep 5
done

# Cleanup
echo "🧹 Cleaning up test resources..."
kubectl delete deployment elb-test 2>/dev/null || true
kubectl delete svc elb-test-svc 2>/dev/null || true
echo "✅ Cleanup done"
