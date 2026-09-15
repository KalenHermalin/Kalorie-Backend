"""Shared OIDC id_token verification, built on Authlib's own JOSE
implementation rather than hand-rolled JWKS-fetching + signature
checking. Used by the Google and Apple providers, which are both OIDC
providers - GitHub isn't (no id_token), so it's unaffected.

This intentionally doesn't use Authlib's higher-level framework clients
(authlib.integrations.starlette_client.OAuth) - those are built around a
server-initiated "redirect the browser, then handle the callback"
flow with session-bound nonce checking, which doesn't fit how this app
works: the mobile app already performs the native sign-in and hands our
backend a `code` (and PKCE `verifier`) directly. We still get Authlib's
real signature/claims verification, just called directly instead of
through the redirect-flow wrapper.
"""

from __future__ import annotations

import time
from typing import Any, Dict, Tuple

import requests
from authlib.jose import JsonWebKey, jwt as authlib_jwt

_JWKS_CACHE_TTL_SECONDS = 60 * 60  # 1 hour - these keys rotate infrequently
_jwks_cache: Dict[str, Tuple[float, Any]] = {}


def _get_key_set(jwks_uri: str):
    cached = _jwks_cache.get(jwks_uri)
    now = time.time()
    if cached is not None and now - cached[0] < _JWKS_CACHE_TTL_SECONDS:
        return cached[1]

    resp = requests.get(jwks_uri, timeout=10)
    resp.raise_for_status()
    key_set = JsonWebKey.import_key_set(resp.json())
    _jwks_cache[jwks_uri] = (now, key_set)
    return key_set


def verify_id_token(id_token: str, jwks_uri: str, issuer: str, audience: str) -> Dict[str, Any]:
    """Verify an OIDC id_token's signature against the provider's published
    JWKS, and validate its issuer/audience/expiry. Returns the decoded
    claims on success; raises on any failure (bad signature, wrong
    issuer/audience, expired, malformed, etc.)."""
    key_set = _get_key_set(jwks_uri)
    claims = authlib_jwt.decode(
        id_token,
        key_set,
        claims_options={
            "iss": {"essential": True, "value": issuer},
            "aud": {"essential": True, "value": audience},
        },
    )
    claims.validate()
    return dict(claims)
