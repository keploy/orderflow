#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# install-istio.sh — Install Istio on Minikube (Mac ARM64 / Apple Silicon)
# ─────────────────────────────────────────────────────────────────────────────
# Run this ONCE before `make deploy-dev`.
# Idempotent — safe to run again.

set -euo pipefail

ISTIO_VERSION="1.29.0"
ARCH="osx-arm64"   # Mac Apple Silicon

echo "=================================================="
echo "  Installing Istio ${ISTIO_VERSION} for Mac ARM64"
echo "=================================================="
echo ""


# ── 2. Download istioctl if not present ─────────────────────────────────────
if ! command -v istioctl &>/dev/null; then
  echo ""
  echo "Downloading istioctl ${ISTIO_VERSION} for ${ARCH}..."
  curl -sSL "https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istio-${ISTIO_VERSION}-${ARCH}.tar.gz" \
    -o /tmp/istio.tar.gz
  tar -xzf /tmp/istio.tar.gz -C /tmp
  sudo mv "/tmp/istio-${ISTIO_VERSION}/bin/istioctl" /usr/local/bin/istioctl
  chmod +x /usr/local/bin/istioctl
  rm -rf /tmp/istio.tar.gz "/tmp/istio-${ISTIO_VERSION}"
  echo "✓ istioctl installed to /usr/local/bin/istioctl"
else
  echo "✓ istioctl already installed: $(istioctl version --remote=false 2>/dev/null || echo 'unknown')"
fi

# ── 3. Pre-flight checks ─────────────────────────────────────────────────────
echo ""
echo "Running Istio pre-flight checks..."
istioctl x precheck
echo "✓ Pre-flight checks passed"

# ── 4. Install Istio with the 'demo' profile ─────────────────────────────────
# 'demo' profile is right for local dev: includes all features, not
# production-tuned. For prod, use 'default' or 'minimal'.
echo ""
echo "Installing Istio (demo profile)..."
istioctl install --set profile=demo -y --force
echo "✓ Istio control plane installed"

# ── 5. Wait for istiod to be ready ──────────────────────────────────────────
echo ""
echo "Waiting for istiod to be ready..."
kubectl rollout status deployment/istiod -n istio-system --timeout=120s
echo "✓ istiod is ready"

# ── 6. Install Kiali (observability dashboard) ───────────────────────────────
echo ""
echo "Installing Kiali + Prometheus (observability addons)..."
ISTIO_ADDONS="https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/addons"
kubectl apply -f "${ISTIO_ADDONS}/prometheus.yaml"
kubectl apply -f "${ISTIO_ADDONS}/kiali.yaml"
echo "✓ Kiali + Prometheus installed"

# ── 7. Apply the orderflow namespace label ───────────────────────────────────
echo ""
echo "Enabling Istio sidecar injection for the 'orderflow' namespace..."
kubectl apply -f base/istio/namespace-patch.yaml
echo "✓ Namespace labelled with istio-injection=enabled"

# ── 8. Summary ────────────────────────────────────────────────────────────────
echo ""
echo "=================================================="
echo "  ✓ Istio installation complete!"
echo "=================================================="
echo ""
echo "Next steps:"
echo ""
echo "  1. Deploy orderflow:"
echo "     make deploy-dev"
echo ""
echo "  2. Verify sidecars are injected (look for 2/2 READY):"
echo "     kubectl get pods -n orderflow"
echo ""
echo "  3. Open Kiali dashboard (service mesh visualiser):"
echo "     make kiali"
echo ""
echo "  4. Test the circuit breaker:"
echo "     make test-circuit-breaker"
echo ""
echo "  To uninstall Istio:"
echo "     istioctl uninstall --purge -y"
echo "     kubectl delete namespace istio-system"
