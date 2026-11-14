#!/usr/bin/env python3
"""SwiftDrop gateway smoke test (Member 9).

Goes through the public gateway like a browser would: logs in as each demo role,
checks /auth/me, the {"data": ...} envelope, the HttpOnly refresh cookie and role checks.

Usage:
    python deploy/scripts/smoke-test.py                         # http://localhost (Compose)
    python deploy/scripts/smoke-test.py https://swiftdrop.work  # production

Passwords for the demo accounts:
  - env SMOKE_PASSWORD_<ROLE> (ADMIN, DISPATCHER, COURIER, MERCHANT, CUSTOMER) if set;
  - otherwise, for localhost only, the published development defaults
    (core-api/migrations/20261021000002_seed_mock_users.up.sql).
In production those defaults are rotated or suspended by core-api, so roles without a
SMOKE_PASSWORD_<ROLE> are skipped there (public checks still run).
Exit code 0 = no failure. Standard library only. Never prints passwords or tokens.
"""
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost").rstrip("/")
IS_LOCAL = urllib.parse.urlparse(BASE).hostname in ("localhost", "127.0.0.1")

DEV_DEFAULTS = {
    "ADMIN": ("admin@swiftdrop.local", "AdminPassword123!"),
    "DISPATCHER": ("dispatcher@swiftdrop.local", "Dispatcher123!"),
    "COURIER": ("courier@swiftdrop.local", "CourierPassword123!"),
    "MERCHANT": ("merchant@vendor.com", "Merchant123!"),
    "CUSTOMER": ("customer@swiftdrop.local", "CustomerPassword123!"),
}
USERS = {}
for _role, (_email, _default) in DEV_DEFAULTS.items():
    _pw = os.environ.get(f"SMOKE_PASSWORD_{_role}") or (_default if IS_LOCAL else None)
    if _pw:
        USERS[_role] = (_email, _pw)

# path -> roles that should be allowed (from the routes in core-api/cmd/api/wiring.go).
# merchant/portal needs a tenant in the token, so Admin (no tenant) gets 403 by design.
PROTECTED = {
    "/api/v1/admin/dashboard": {"ADMIN"},
    "/api/v1/courier/dashboard": {"COURIER", "ADMIN"},
    "/api/v1/merchant/portal": {"MERCHANT"},
}
# Reported to an owner and not fixed yet: shown as KNOWN instead of FAIL. (Empty = none.)
KNOWN = {}

results = {"PASS": 0, "FAIL": 0, "KNOWN": 0}


LOGIN_CALLS = 0
# Member 7's gateway allows 5 logins/min per IP with a burst of 3 (login-ratelimit). Through the
# gateway, pace the logins after the first two so the test stays inside that limit (~1 min total).
LOGIN_PACE_SECONDS = 0 if IS_LOCAL else 13


def call(method, path, body=None, token=None):
    global LOGIN_CALLS
    if path == "/api/v1/auth/login":
        if LOGIN_CALLS >= 2 and LOGIN_PACE_SECONDS:
            time.sleep(LOGIN_PACE_SECONDS)
        LOGIN_CALLS += 1
    req = urllib.request.Request(BASE + path, method=method)
    req.add_header("Content-Type", "application/json")
    # Cloudflare's Browser Integrity Check blocks the default "Python-urllib" agent (403, error 1010).
    req.add_header("User-Agent", "SwiftDrop-SmokeTest/1.0 (+deploy)")
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
_skipped = [r for r in DEV_DEFAULTS if r not in USERS]
if _skipped:
    print(f"SKIP   role logins for {', '.join(_skipped)} (set SMOKE_PASSWORD_<ROLE> to test them)\n")
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
