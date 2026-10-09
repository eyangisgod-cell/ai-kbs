#!/usr/bin/env python3
"""
Langfuse v3 dashboard bootstrap.

Imports `dashboards/ai-kbs-default.json` into the Langfuse instance running at
$LANGFUSE_URL. Runs once on first docker compose up; idempotent (skips if the
dashboard already exists by name).

Auth flow (NextAuth credentials provider):
  1. GET /api/auth/csrf to obtain a CSRF token
  2. POST /api/auth/callback/credentials with the email + password + csrfToken
  3. The response sets `next-auth.session-token` (or `__Secure-...` on https)
  4. Subsequent calls to /api/projects/{id}/dashboards use that cookie

Exit codes:
  0 = success (dashboard created or already present)
  1 = unrecoverable error
  2 = Langfuse never became healthy (timeout)
"""
from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

LANGFUSE_URL = os.environ.get("LANGFUSE_URL", "http://langfuse:3000").rstrip("/")
LANGFUSE_USER_EMAIL = os.environ.get("LANGFUSE_INIT_USER_EMAIL", "dev@ai-kbs.local")
LANGFUSE_USER_PASSWORD = os.environ.get("LANGFUSE_INIT_USER_PASSWORD", "changeme-dev-only")
LANGFUSE_PROJECT_ID = os.environ.get("LANGFUSE_INIT_PROJECT_ID", "")
LANGFUSE_PROJECT_NAME = os.environ.get("LANGFUSE_INIT_PROJECT_NAME", "ai-kbs")
DASHBOARD_PATH = Path(os.environ.get("DASHBOARD_PATH", "/dashboards/ai-kbs-default.json"))
HEALTH_TIMEOUT_S = int(os.environ.get("HEALTH_TIMEOUT_S", "120"))
POLL_INTERVAL_S = 2


def make_session() -> requests.Session:
    s = requests.Session()
    retry = Retry(
        total=5,
        backoff_factor=0.5,
        status_forcelist=(502, 503, 504),
        allowed_methods=("GET", "POST"),
    )
    s.mount("http://", HTTPAdapter(max_retries=retry))
    s.mount("https://", HTTPAdapter(max_retries=retry))
    return s


def wait_for_health(session: requests.Session) -> None:
    """Block until /api/public/health returns 200, or HEALTH_TIMEOUT_S elapses."""
    health_url = f"{LANGFUSE_URL}/api/public/health"
    deadline = time.time() + HEALTH_TIMEOUT_S
    last_err: str | None = None
    while time.time() < deadline:
        try:
            r = session.get(health_url, timeout=3)
            if r.status_code == 200:
                return
            last_err = f"status={r.status_code} body={r.text[:200]}"
        except requests.RequestException as e:
            last_err = f"exception={e!r}"
        time.sleep(POLL_INTERVAL_S)
    raise TimeoutError(
        f"Langfuse at {LANGFUSE_URL} did not become healthy within "
        f"{HEALTH_TIMEOUT_S}s (last: {last_err})"
    )


def login(session: requests.Session) -> None:
    """Authenticate via NextAuth credentials provider. Sets the session cookie."""
    csrf_url = f"{LANGFUSE_URL}/api/auth/csrf"
    csrf = session.get(csrf_url, timeout=10).json()
    csrf_token = csrf.get("csrfToken")
    if not csrf_token:
        raise RuntimeError(f"no csrfToken in {csrf_url} response")

    callback_url = f"{LANGFUSE_URL}/api/auth/callback/credentials"
    r = session.post(
        callback_url,
        data={
            "csrfToken": csrf_token,
            "email": LANGFUSE_USER_EMAIL,
            "password": LANGFUSE_USER_PASSWORD,
            "redirect": "false",
            "json": "true",
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        timeout=10,
        allow_redirects=False,
    )
    if r.status_code not in (200, 302):
        raise RuntimeError(
            f"login failed: status={r.status_code} body={r.text[:200]}"
        )

    # Verify the session cookie landed.
    cookie_names = {c.name for c in session.cookies}
    has_session = any(
        n.startswith("next-auth.session-token") or n.startswith("__Secure-next-auth.session-token")
        for n in cookie_names
    )
    if not has_session:
        raise RuntimeError(
            f"login response set no session cookie (cookies={cookie_names})"
        )


def resolve_project_id(session: requests.Session) -> str:
    """Return LANGFUSE_INIT_PROJECT_ID, or look it up by name."""
    if LANGFUSE_PROJECT_ID:
        return LANGFUSE_PROJECT_ID

    r = session.get(f"{LANGFUSE_URL}/api/projects", timeout=10)
    r.raise_for_status()
    for p in r.json():
        if p.get("name") == LANGFUSE_PROJECT_NAME:
            return p["id"]
    raise RuntimeError(
        f"project {LANGFUSE_PROJECT_NAME!r} not found; "
        f"available: {[p.get('name') for p in r.json()]}"
    )


def list_dashboards(session: requests.Session, project_id: str) -> list[dict]:
    r = session.get(
        f"{LANGFUSE_URL}/api/projects/{project_id}/dashboards",
        timeout=10,
    )
    r.raise_for_status()
    return r.json()


def create_dashboard(
    session: requests.Session, project_id: str, definition: dict
) -> dict:
    r = session.post(
        f"{LANGFUSE_URL}/api/projects/{project_id}/dashboards",
        json=definition,
        headers={"Content-Type": "application/json"},
        timeout=15,
    )
    r.raise_for_status()
    return r.json()


def main() -> int:
    session = make_session()

    print(f"[bootstrap] waiting for Langfuse at {LANGFUSE_URL} ...")
    try:
        wait_for_health(session)
    except TimeoutError as e:
        print(f"[bootstrap] FATAL: {e}", file=sys.stderr)
        return 2
    print(f"[bootstrap] Langfuse is healthy.")

    print(f"[bootstrap] logging in as {LANGFUSE_USER_EMAIL} ...")
    try:
        login(session)
    except Exception as e:
        print(f"[bootstrap] FATAL: login failed: {e}", file=sys.stderr)
        return 1
    print(f"[bootstrap] authenticated.")

    try:
        project_id = resolve_project_id(session)
        print(f"[bootstrap] project_id = {project_id}")
    except Exception as e:
        print(f"[bootstrap] FATAL: cannot resolve project: {e}", file=sys.stderr)
        return 1

    if not DASHBOARD_PATH.exists():
        print(f"[bootstrap] FATAL: dashboard file not found: {DASHBOARD_PATH}", file=sys.stderr)
        return 1
    definition = json.loads(DASHBOARD_PATH.read_text(encoding="utf-8"))
    dashboard_name = definition.get("name", "")

    existing = list_dashboards(session, project_id)
    if any(d.get("name") == dashboard_name for d in existing):
        print(f"[bootstrap] dashboard {dashboard_name!r} already present; skipping.")
        return 0

    print(f"[bootstrap] creating dashboard {dashboard_name!r} with "
          f"{len(definition.get('panels', []))} panels ...")
    try:
        result = create_dashboard(session, project_id, definition)
    except requests.HTTPError as e:
        body = e.response.text[:500] if e.response is not None else ""
        print(f"[bootstrap] FATAL: create failed: status={e.response.status_code} body={body}",
              file=sys.stderr)
        return 1
    print(f"[bootstrap] created: id={result.get('id')} name={result.get('name')}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
