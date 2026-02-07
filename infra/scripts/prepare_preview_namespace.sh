#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <namespace>" >&2
  exit 1
fi

NAMESPACE="$1"

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

cat <<YAML | kubectl apply -n "$NAMESPACE" -f -
apiVersion: v1
kind: ResourceQuota
metadata:
  name: preview-quota
spec:
  hard:
    requests.cpu: "1"
    requests.memory: 1Gi
    limits.cpu: "2"
    limits.memory: 2Gi
    pods: "10"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: preview-limits
spec:
  limits:
    - type: Container
      default:
        cpu: 300m
        memory: 512Mi
      defaultRequest:
        cpu: 100m
        memory: 128Mi
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-ingress
spec:
  podSelector: {}
  policyTypes:
    - Ingress
YAML

echo "Prepared preview namespace: $NAMESPACE"
