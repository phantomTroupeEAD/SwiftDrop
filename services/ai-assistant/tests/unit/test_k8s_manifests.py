"""Unit test suite verifying Kubernetes manifest schema validity, security context, and resource quotas."""

from pathlib import Path
import pytest
import yaml

MANIFEST_DIR = Path(__file__).resolve().parent.parent.parent.parent.parent / "deploy" / "kubernetes" / "ai-assistant"


def load_yaml(filename: str) -> dict:
    filepath = MANIFEST_DIR / filename
    assert filepath.exists(), f"Manifest file missing: {filepath}"
    with open(filepath, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def test_k8s_directory_exists():
    """Verify deploy/kubernetes/ai-assistant directory exists and contains all required manifests."""
    assert MANIFEST_DIR.exists() and MANIFEST_DIR.is_dir()
    expected_files = ["deployment.yaml", "service.yaml", "configmap.yaml", "secret.yaml.example", "hpa.yaml"]
    for fname in expected_files:
        assert (MANIFEST_DIR / fname).exists(), f"Expected file {fname} not found in {MANIFEST_DIR}"


def test_deployment_schema_and_metadata():
    """TC-K8S-001: Verify deployment schema, API version, and standard labels."""
    dep = load_yaml("deployment.yaml")
    assert dep["apiVersion"] == "apps/v1"
    assert dep["kind"] == "Deployment"
    assert dep["metadata"]["name"] == "ai-assistant"
    assert dep["metadata"]["namespace"] == "swiftdrop"
    assert dep["spec"]["replicas"] == 2
    assert dep["spec"]["strategy"]["type"] == "RollingUpdate"
    assert dep["spec"]["strategy"]["rollingUpdate"]["maxUnavailable"] == 0


def test_deployment_stateless_invariant():
    """TC-K8S-002: Verify stateless invariant (zero persistent volume claims or host mounts)."""
    dep = load_yaml("deployment.yaml")
    pod_spec = dep["spec"]["template"]["spec"]
    
    # Pod must only mount emptyDir for /tmp
    volumes = pod_spec.get("volumes", [])
    assert len(volumes) == 1
    assert volumes[0]["name"] == "tmp-volume"
    assert "emptyDir" in volumes[0]
    assert "persistentVolumeClaim" not in volumes[0]
    assert "hostPath" not in volumes[0]


def test_deployment_least_privilege_security_context():
    """TC-K8S-003: Verify pod-level and container-level non-root security context."""
    dep = load_yaml("deployment.yaml")
    pod_spec = dep["spec"]["template"]["spec"]
    
    # Pod-level security context
    pod_sec = pod_spec.get("securityContext", {})
    assert pod_sec.get("runAsNonRoot") is True
    assert pod_sec.get("runAsUser") == 10001
    assert pod_sec.get("runAsGroup") == 10001
    assert pod_sec.get("fsGroup") == 10001
    
    # Container-level security context
    container = pod_spec["containers"][0]
    c_sec = container.get("securityContext", {})
    assert c_sec.get("readOnlyRootFilesystem") is True
    assert c_sec.get("allowPrivilegeEscalation") is False
    assert c_sec.get("capabilities", {}).get("drop") == ["ALL"]


def test_deployment_resource_quotas():
    """TC-K8S-004: Verify CPU and memory requests and limits."""
    dep = load_yaml("deployment.yaml")
    container = dep["spec"]["template"]["spec"]["containers"][0]
    resources = container.get("resources", {})
    
    assert "requests" in resources and "limits" in resources
    assert resources["requests"]["cpu"] == "100m"
    assert resources["requests"]["memory"] == "128Mi"
    assert resources["limits"]["cpu"] == "500m"
    assert resources["limits"]["memory"] == "512Mi"


def test_deployment_probes_and_grace_period():
    """TC-K8S-005 & TC-K8S-006: Verify decoupled liveness/readiness probes and 20s termination grace."""
    dep = load_yaml("deployment.yaml")
    pod_spec = dep["spec"]["template"]["spec"]
    assert pod_spec.get("terminationGracePeriodSeconds") == 20
    
    container = pod_spec["containers"][0]
    liveness = container.get("livenessProbe", {})
    readiness = container.get("readinessProbe", {})
    
    assert liveness.get("httpGet", {}).get("path") == "/health/live"
    assert liveness.get("httpGet", {}).get("port") == 8000
    assert liveness.get("initialDelaySeconds") == 5
    
    assert readiness.get("httpGet", {}).get("path") == "/health/ready"
    assert readiness.get("httpGet", {}).get("port") == 8000
    assert readiness.get("initialDelaySeconds") == 5


def test_service_dual_ports():
    """Verify Service exposes both REST (8000) and FastMCP SSE (8002) ClusterIP ports."""
    svc = load_yaml("service.yaml")
    assert svc["apiVersion"] == "v1"
    assert svc["kind"] == "Service"
    assert svc["metadata"]["name"] == "ai-assistant"
    assert svc["spec"]["type"] == "ClusterIP"
    
    ports = svc["spec"]["ports"]
    assert len(ports) == 2
    
    port_map = {p["name"]: p for p in ports}
    assert "http-api" in port_map
    assert port_map["http-api"]["port"] == 8000
    assert port_map["http-api"]["targetPort"] == 8000
    
    assert "mcp-sse" in port_map
    assert port_map["mcp-sse"]["port"] == 8002
    assert port_map["mcp-sse"]["targetPort"] == 8002


def test_configmap_values():
    """Verify ConfigMap defines mandatory upstream routes and rate limits."""
    cm = load_yaml("configmap.yaml")
    assert cm["apiVersion"] == "v1"
    assert cm["kind"] == "ConfigMap"
    data = cm["data"]
    
    assert data["CORE_API_URL"] == "http://core-api:8080"
    assert data["TRACKING_API_URL"] == "http://tracking-api:8081"
    assert data["PORT"] == "8000"
    assert data["MCP_PORT"] == "8002"
    assert data["RATE_LIMIT_ANONYMOUS_RPM"] == "20"


def test_hpa_configuration():
    """Verify HorizontalPodAutoscaler targets Deployment and scales between 1 and 3 replicas."""
    hpa = load_yaml("hpa.yaml")
    assert hpa["apiVersion"] == "autoscaling/v2"
    assert hpa["kind"] == "HorizontalPodAutoscaler"
    assert hpa["spec"]["scaleTargetRef"]["name"] == "ai-assistant"
    assert hpa["spec"]["minReplicas"] == 1
    assert hpa["spec"]["maxReplicas"] == 3
    
    metrics = hpa["spec"]["metrics"]
    cpu_metric = next(m for m in metrics if m["resource"]["name"] == "cpu")
    assert cpu_metric["resource"]["target"]["averageUtilization"] == 70
