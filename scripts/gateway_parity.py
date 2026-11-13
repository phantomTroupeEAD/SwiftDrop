#!/usr/bin/env python3
"""Fail if the Docker gateway file and the Kubernetes gateway files disagree (Doc 3 DD-12).
Compares every middleware and servers transport by name and value. Allowed origins are
skipped on purpose: they differ by environment (localhost vs the real domain).
Usage: python3 scripts/gateway_parity.py"""
import sys, yaml, pathlib

root = pathlib.Path(__file__).resolve().parent.parent / "deploy/gateway/traefik"
file_cfg = yaml.safe_load((root / "dynamic-routes.yml").read_text())["http"]
crds = [d for d in yaml.safe_load_all((root / "kubernetes/middlewares.yaml").read_text()) if d]

def norm(v):
    if isinstance(v, dict):
        return {k: norm(x) for k, x in v.items() if k != "accessControlAllowOriginList"}
    return v

problems = []
for kind, file_key in (("Middleware", "middlewares"), ("ServersTransport", "serversTransports")):
    in_file = {k: norm(v) for k, v in file_cfg.get(file_key, {}).items()}
    in_k8s = {d["metadata"]["name"]: norm(d["spec"]) for d in crds if d["kind"] == kind}
    for name in sorted(set(in_file) | set(in_k8s)):
        if name not in in_k8s:
            problems.append(f"{kind} {name}: only in dynamic-routes.yml")
        elif name not in in_file:
            problems.append(f"{kind} {name}: only in kubernetes/middlewares.yaml")
        elif in_file[name] != in_k8s[name]:
            problems.append(f"{kind} {name}: values differ\n   file: {in_file[name]}\n   k8s:  {in_k8s[name]}")
    print(f"{kind}: {len(in_file)} in file, {len(in_k8s)} in Kubernetes")
if problems:
    print("PARITY FAILED"); print("\n".join(problems)); sys.exit(1)
print("PARITY OK")
