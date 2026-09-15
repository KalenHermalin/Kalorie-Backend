"""Unit test for the Apple client-secret JWT generation - the one bit of
AppleProvider that's pure logic and testable without hitting Apple's
servers or needing real Apple Developer credentials. Generates a
throwaway EC keypair to sign/verify against, the same shape as a real
Sign in with Apple `.p8` key.
"""

from __future__ import annotations

import time

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

from app.auth.providers.apple_provider import AppleProvider


def _generate_test_keypair() -> tuple[str, str]:
    private_key = ec.generate_private_key(ec.SECP256R1())
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode()
    public_pem = private_key.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    ).decode()
    return private_pem, public_pem


def test_generate_client_secret_is_a_valid_signed_jwt_with_expected_claims():
    private_pem, public_pem = _generate_test_keypair()

    provider = AppleProvider(
        client_id="com.example.kalorie",
        team_id="TEAMID1234",
        key_id="KEYID5678",
        private_key=private_pem,
        callback="kalorie://",
    )

    token = provider._generate_client_secret()

    # Verify signature + claims exactly the way Apple's token endpoint would.
    claims = pyjwt.decode(token, public_pem, algorithms=["ES256"], audience="https://appleid.apple.com")

    assert claims["iss"] == "TEAMID1234"
    assert claims["sub"] == "com.example.kalorie"
    assert claims["aud"] == "https://appleid.apple.com"
    assert 0 < claims["exp"] - claims["iat"] <= 60 * 60 * 24 * 180  # under Apple's 6-month max

    header = pyjwt.get_unverified_header(token)
    assert header["kid"] == "KEYID5678"
    assert header["alg"] == "ES256"


def test_generate_client_secret_unescapes_literal_backslash_n_in_env_style_keys():
    private_pem, public_pem = _generate_test_keypair()
    escaped_key = private_pem.replace("\n", "\\n")  # as it'd look pasted into a .env file

    provider = AppleProvider(
        client_id="com.example.kalorie",
        team_id="TEAMID1234",
        key_id="KEYID5678",
        private_key=escaped_key,
        callback="kalorie://",
    )

    token = provider._generate_client_secret()
    claims = pyjwt.decode(token, public_pem, algorithms=["ES256"], audience="https://appleid.apple.com")
    assert claims["sub"] == "com.example.kalorie"


def test_get_provider_name_and_platform():
    provider = AppleProvider("id", "team", "key", "pk", "kalorie://")
    assert provider.get_provider_name() == "apple"
    assert provider.get_platform() == ""
