"""Unit tests for app.auth.oidc.verify_id_token - generates a throwaway
RSA keypair and a real signed id_token to verify against, the same shape
Apple/Google's actual JWKS + id_tokens take, without hitting either
provider's servers.
"""

from __future__ import annotations

import time
from unittest.mock import patch

import pytest
from authlib.jose import JsonWebKey, jwt as authlib_jwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.auth import oidc


def _make_test_jwks_and_signer():
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    jwk = JsonWebKey.import_key(private_pem, {"kty": "RSA", "kid": "test-kid"})
    jwks = {"keys": [jwk.as_dict(is_private=False)]}

    def sign(claims: dict) -> str:
        header = {"alg": "RS256", "kid": "test-kid"}
        return authlib_jwt.encode(header, claims, private_pem).decode()

    return jwks, sign


@pytest.fixture(autouse=True)
def _clear_jwks_cache():
    oidc._jwks_cache.clear()
    yield
    oidc._jwks_cache.clear()


def test_verify_id_token_accepts_a_validly_signed_token():
    jwks, sign = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign({"iss": "https://issuer.example", "aud": "client-1", "sub": "user-1", "email": "a@b.com", "iat": now, "exp": now + 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks
        mock_get.return_value.raise_for_status.return_value = None

        claims = oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")

    assert claims["sub"] == "user-1"
    assert claims["email"] == "a@b.com"
    mock_get.assert_called_once()


def test_verify_id_token_caches_the_jwks_across_calls():
    jwks, sign = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign({"iss": "https://issuer.example", "aud": "client-1", "sub": "user-1", "iat": now, "exp": now + 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks
        mock_get.return_value.raise_for_status.return_value = None

        oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")
        oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")

    mock_get.assert_called_once()  # second call served from cache


def test_verify_id_token_rejects_wrong_audience():
    jwks, sign = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign({"iss": "https://issuer.example", "aud": "someone-else", "sub": "user-1", "iat": now, "exp": now + 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks
        mock_get.return_value.raise_for_status.return_value = None

        with pytest.raises(Exception):
            oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")


def test_verify_id_token_rejects_wrong_issuer():
    jwks, sign = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign({"iss": "https://not-the-issuer.example", "aud": "client-1", "sub": "user-1", "iat": now, "exp": now + 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks
        mock_get.return_value.raise_for_status.return_value = None

        with pytest.raises(Exception):
            oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")


def test_verify_id_token_rejects_expired_token():
    jwks, sign = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign({"iss": "https://issuer.example", "aud": "client-1", "sub": "user-1", "iat": now - 600, "exp": now - 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks
        mock_get.return_value.raise_for_status.return_value = None

        with pytest.raises(Exception):
            oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")


def test_verify_id_token_rejects_a_token_signed_by_a_different_key():
    jwks, _sign = _make_test_jwks_and_signer()
    _other_jwks, sign_with_other_key = _make_test_jwks_and_signer()
    now = int(time.time())
    token = sign_with_other_key({"iss": "https://issuer.example", "aud": "client-1", "sub": "user-1", "iat": now, "exp": now + 300})

    with patch("app.auth.oidc.requests.get") as mock_get:
        mock_get.return_value.json.return_value = jwks  # the *first* keypair's public JWKS
        mock_get.return_value.raise_for_status.return_value = None

        with pytest.raises(Exception):
            oidc.verify_id_token(token, "https://issuer.example/jwks", "https://issuer.example", "client-1")
