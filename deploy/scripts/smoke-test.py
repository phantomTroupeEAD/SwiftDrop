#!/usr/bin/env python3
"""SwiftDrop gateway smoke test (Member 9).

Goes through the public gateway like a browser would: logs in as each demo role,
checks /auth/me, the {"data": ...} envelope, the HttpOnly refresh cookie and role checks.

Usage:
    python deploy/scripts/smoke-test.py                         # http://localhost (Compose)
    python deploy/scripts/smoke-test.py https://swiftdrop.work  # production

The demo accounts are the ones core-api seeds in local/test mode
(core-api/migrations/20261021000002_seed_mock_users.up.sql). Exit code 0 = no unexpected failure.
Standard library only.
"""
import json
import sys
import urllib.error
import urllib.request

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost").rstrip("/")

USERS = {
    "ADMIN": ("admin@swiftdrop.local", "AdminPassword123!"),
    "DISPATCHER": ("dispatcher@swiftdrop.local", "Dispatcher123!"),
    "COURIER": ("courier@swiftdrop.local", "CourierPassword123!"),
    "MERCHANT": ("merchant@vendor.com", "Merchant123!"),
    "CUSTOMER": ("customer@swiftdrop.local", "CustomerPassword123!"),
}
# path -> roles that should be allowed (from the routes in core-api/cmd/api/wiring.go)
PROTECTED = {
    "/api/v1/admin/dashboard": {"ADMIN"},
    "/api/v1/courier/dashboard": {"COURIER", "ADMIN"},
    "/api/v1/merchant/portal": {"MERCHANT", "ADMIN"},
}
# Reported to the owner; shown as KNOWN instead of FAIL until fixed.
KNOWN = {
    "ADMIN /api/v1/merchant/portal": "admin has no tenant, RequireTenant() returns 403 (reported to M1)",
    "error envelope correlation_id": "error envelope has no correlation_id yet (reported to M1)",
}

results = {"PASS": 0, "FAIL": 0, "KNOWN": 0}


def call(method, path, body=None, token=None):
    req = urllib.request.Request(BASE + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    try:
        with urllib.request.urlopen(req, data=data, timeout=15) as r:
            return r.status, r.headers, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.headers, e.read()


def check(name, ok, detail="", known_key=None):
    if ok:
        status = "PASS"
    elif known_key in KNOWN:
        status, detail = "KNOWN", KNOWN[known_key]
    else:
        status = "FAIL"
    results[status] += 1
    print(f"{status:5}  {name}" + (f"  -> {detail}" if status != "PASS" and detail else ""))


print(f"Smoke test against {BASE}\n")
for role, (email, password) in USERS.items():
    st, hdr, raw = call("POST", "/api/v1/auth/login", {"email": email, "password": password})
    check(f"{role:10} login", st == 200, f"HTTP {st} {raw[:100]!r}")
    if st != 200:
        continue
    body = json.loads(raw)
    data = body.get("data", {})
    token = data.get("access_token") or data.get("accessToken") or data.get("token")
    check(f"{role:10} {{\"data\": ...}} envelope", "data" in body)
    check(f"{role:10} access token in body", bool(token), f"keys={list(data)}")
    cookie = hdr.get("Set-Cookie") or ""
    check(f"{role:10} refresh cookie HttpOnly", "httponly" in cookie.lower(), cookie[:60] or "no Set-Cookie")
    st, _, _ = call("GET", "/api/v1/auth/me", token=token)
    check(f"{role:10} /auth/me", st == 200, f"HTTP {st}")
    for path, allowed in PROTECTED.items():
        st, _, _ = call("GET", path, token=token)
        want = role in allowed
        check(f"{role:10} {path} {'allowed' if want else 'denied'}",
              st == 200 if want else st in (401, 403), f"HTTP {st}", known_key=f"{role} {path}")

st, _, _ = call("GET", "/api/v1/auth/me")
check("no token -> 401", st == 401, f"HTTP {st}")
st, _, raw = call("POST", "/api/v1/auth/login", {"email": "admin@swiftdrop.local", "password": "wrong-password"})
check("wrong password -> 401", st == 401, f"HTTP {st}")
try:
    err = json.loads(raw).get("error", {})
    check("error envelope has code", bool(err.get("code")), str(err)[:100])
    check("error envelope has correlation_id", "correlation_id" in err, str(err)[:100],
          known_key="error envelope correlation_id")
except ValueError as e:
    check("error envelope is JSON", False, str(e))
st, hdr, _ = call("GET", "/")
check("SPA served at /", st == 200 and "text/html" in (hdr.get("Content-Type") or ""), f"HTTP {st}")

print(f"\n{results['PASS']} passed, {results['KNOWN']} known issues, {results['FAIL']} failed")
sys.exit(1 if results["FAIL"] else 0)
