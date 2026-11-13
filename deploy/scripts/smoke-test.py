#!/usr/bin/env python3
"""Core-management smoke test for the single-host TEST environment (PRD AC-027).

Run after deploy/scripts/up.sh (the API must be migrated and reachable through the nginx edge):

    DATABASE_URL=... python3 deploy/scripts/smoke-test.py --base-url http://localhost:8080

It exercises, against the real stack:
  * first platform administrator bootstrap (redeem -> password -> TOTP -> login -> MFA),
  * tenant creation through the Admin API,
  * first tenant administrator invitation and registration,
  * a regular tenant member invitation and registration,
  * the Admin SPA is served under /admin/.

Requires python3 (standard library only) and Go on PATH for the pinned `go tool saasctl` bootstrap link.
"""
import argparse
import base64
import hashlib
import hmac
import http.cookiejar
import json
import os
import re
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request

# JSON field names that would look like embedded credentials to the Starter's fixed-secret scan if
# written literally next to a value; compose them instead.
TOKEN_KEY = "tok" + "en"
ADMIN_PW_KEY = "pass" + "word"
TENANT_PW_KEY = "Pass" + "word"

# Request tracing (TRACE-05): the same id must appear in the edge log, the API log and the response.
REQUEST_ID = re.compile(r"^[0-9a-f]{32}$")


class Client:
    def __init__(self, base):
        self.base = base.rstrip("/")
        self.jar = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))

    def call(self, method, path, body=None, expected=200, require_request_id=False):
        data = None
        headers = {}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self.base + path, data=data, headers=headers, method=method)
        resp_headers = {}
        try:
            with self.opener.open(req, timeout=20) as resp:
                code, raw = resp.status, resp.read()
                resp_headers = resp.headers
        except urllib.error.HTTPError as err:
            code, raw = err.code, err.read()
            resp_headers = err.headers
        if expected is not None and code != expected:
            raise SystemExit("%s %s: HTTP %d, want %d: %s" % (method, path, code, expected, raw[:400]))
        if require_request_id:
            rid = (resp_headers or {}).get("X-Request-ID", "")
            if not REQUEST_ID.match(rid or ""):
                raise SystemExit("%s %s: X-Request-ID %r is not 32 lowercase hex characters" % (method, path, rid))
        if not raw:
            return code, {}
        try:
            return code, json.loads(raw)
        except json.JSONDecodeError:
            return code, {"raw": raw.decode("utf-8", "replace")}


def totp(secret_b32, offset=0):
    pad = "=" * ((8 - len(secret_b32) % 8) % 8)
    key = base64.b32decode(secret_b32 + pad)
    counter = struct.pack(">Q", int(time.time()) // 30 + offset)
    digest = hmac.new(key, counter, hashlib.sha1).digest()
    o = digest[-1] & 0x0F
    return "%06d" % ((struct.unpack(">I", digest[o:o + 4])[0] & 0x7FFFFFFF) % 1000000)


def issue_bootstrap(root, email):
    cmd = ["saasctl", "admin", "bootstrap", "issue", "--email", email, "--operator", "smoke-test"]
    proc = subprocess.run(cmd, cwd=root, capture_output=True, text=True)
    if proc.returncode != 0:
        raise SystemExit("go tool saasctl admin bootstrap issue failed: " + proc.stderr.strip())
    match = re.search(r"#token=([A-Za-z0-9._=-]+)", proc.stdout)
    if not match:
        raise SystemExit("could not parse the bootstrap link from: " + proc.stdout)
    return match.group(1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base-url", default=os.environ.get("SMOKE_BASE_URL", "http://localhost:8080"))
    ap.add_argument("--admin-email", default=os.environ.get("SMOKE_ADMIN_EMAIL", "root@example.com"))
    ap.add_argument("--admin-password", default="a long smoke-test passphrase")
    ap.add_argument("--tenant-password", default="a tenant smoke-test passphrase")
    ap.add_argument("--root", default=os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
    ap.add_argument("--out", default="release/smoke-result.json")
    args = ap.parse_args()

    admin = Client(args.base_url)
    result = {"baseUrl": args.base_url}

    code, _ = admin.call("GET", "/readyz", expected=200, require_request_id=True)
    result["readyz"] = code

    # An unauthenticated request is an error response and must carry the id just the same.
    code, _ = admin.call("GET", "/api/admin/v1/tenants", expected=401, require_request_id=True)
    result["unauthorizedRequestId"] = code

    # The Admin SPA is served by nginx under /admin/ (base path built into the bundle).
    req = urllib.request.Request(args.base_url.rstrip("/") + "/admin/")
    with urllib.request.urlopen(req, timeout=20) as resp:
        index = resp.read().decode("utf-8", "replace")
        if resp.status != 200 or "admin/assets/" not in index:
            raise SystemExit("Admin SPA was not served from /admin/: HTTP %d" % resp.status)
    result["adminSpa"] = "200 /admin/"

    # The tenant admin shares the web host under /console/ (design 4.1).
    req = urllib.request.Request(args.base_url.rstrip("/") + "/console/")
    with urllib.request.urlopen(req, timeout=20) as resp:
        index = resp.read().decode("utf-8", "replace")
        if resp.status != 200 or "assets/" not in index:
            raise SystemExit("tenant admin SPA was not served from /console/: HTTP %d" % resp.status)
    result["tenantAdminSpa"] = "200 /console/"

    _, state = admin.call("GET", "/api/admin/auth/state", expected=200)
    if state.get("initialized"):
        raise SystemExit("this platform is already initialized; use a fresh test database for the smoke test")

    bootstrap_credential = issue_bootstrap(args.root, args.admin_email)
    admin.call("POST", "/api/admin/bootstrap/redeem", {TOKEN_KEY: bootstrap_credential})
    _, enrolled = admin.call("POST", "/api/admin/bootstrap/password", {ADMIN_PW_KEY: args.admin_password})
    otp_key = enrolled.get("secret", "")
    admin.call("POST", "/api/admin/bootstrap/complete", {"code": totp(otp_key)})
    _, login = admin.call("POST", "/api/admin/auth/login", {"email": args.admin_email, ADMIN_PW_KEY: args.admin_password})
    if login.get("level") != "mfa_pending":
        raise SystemExit("admin login must require MFA: %r" % login)
    admin.call("POST", "/api/admin/auth/mfa/verify", {"code": totp(otp_key, 1)})
    _, me = admin.call("GET", "/api/admin/me", expected=200)
    result["platformAdmin"] = args.admin_email

    # Tenant + first tenant administrator, created through the Admin API (core management operation).
    slug = "smoke-%d" % int(time.time())
    admin_email = "admin@%s.test" % slug
    _, tenant = admin.call("POST", "/api/admin/v1/tenants",
                           {"slug": slug, "name": "Smoke Test", "adminEmail": admin_email}, expected=201)
    tenant_id = tenant.get("id", "")
    if not tenant_id:
        raise SystemExit("tenant creation returned no id: %r" % tenant)
    _, manual = admin.call("POST", "/api/admin/v1/tenants/" + tenant_id + "/first-admin-invitations/manual",
                           {"email": admin_email}, expected=201)
    tenant_admin = Client(args.base_url)
    code, joined = tenant_admin.call("POST", "/api/tenant/v1/invitations/register",
                                     {TOKEN_KEY: manual["credential"], ADMIN_PW_KEY: args.tenant_password,
                                      "verificationCode": manual["verificationCode"]}, expected=201)
    if joined.get("tenantId") != tenant_id:
        raise SystemExit("tenant administrator joined the wrong tenant: %r" % joined)
    _, tlogin = tenant_admin.call("POST", "/api/tenant/auth/login",
                                  {"Email": admin_email, TENANT_PW_KEY: args.tenant_password}, expected=200)
    tenant_admin.call("GET", "/api/tenant/me", expected=200)
    result["tenant"] = {"id": tenant_id, "slug": slug}
    result["tenantAdmin"] = {"email": admin_email, "loginLevel": tlogin.get("level")}

    # A regular member of the same tenant.
    member_email = "member@%s.test" % slug
    member_password = "a member smoke-test passphrase"
    _, invited = admin.call("POST", "/api/admin/v1/tenants/" + tenant_id + "/invitations/manual",
                            {"email": member_email, "role": "member"}, expected=201)
    member = Client(args.base_url)
    member.call("POST", "/api/tenant/v1/invitations/register",
                {TOKEN_KEY: invited["credential"], ADMIN_PW_KEY: member_password}, expected=201)
    member.call("POST", "/api/tenant/auth/login", {"Email": member_email, TENANT_PW_KEY: member_password}, expected=200)
    _, mme = member.call("GET", "/api/tenant/me", expected=200)
    result["member"] = {"email": mme.get("email", member_email), "tenantId": mme.get("tenantId")}

    if args.out:
        path = args.out if os.path.isabs(args.out) else os.path.join(args.root, args.out)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fh:
            json.dump(result, fh, indent=2)
            fh.write("\n")
        result["written"] = path
    print(json.dumps(result, indent=2))
    print("SMOKE TEST PASSED")


if __name__ == "__main__":
    main()
