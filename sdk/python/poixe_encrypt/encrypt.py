"""Core encryption implementation for Poixe API key envelopes."""

from __future__ import annotations

import base64
import hashlib
import json
import os
import threading
import urllib.error
import urllib.request
from typing import Optional
from urllib.parse import urlparse

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

DEFAULT_PUBKEY_URL = "https://poixe.com/security.json"

_DEFAULT_AWS_REGION = "eu-central-1"
_VALID_AWS_REGIONS = [
    "eu-central-1", "eu-west-1", "eu-west-2",
    "us-east-1", "us-east-2", "us-west-2",
]

_CHANNEL_TYPES: dict[str, dict] = {
    "openai_chatgpt":        {"type_int": 2},
    "azure_chatgpt":         {"type_int": 3,  "needs_endpoint": True},
    "deepseek_deepseek":     {"type_int": 4},
    "anthropic_claude":      {"type_int": 5},
    "google_aistudio_gemini":{"type_int": 6},
    "xai_grok":              {"type_int": 7},
    "volcengine_deepseek":   {"type_int": 8},
    "volcengine_doubao":     {"type_int": 9},
    "aliyun_deepseek":       {"type_int": 10},
    "aliyun_qwen":           {"type_int": 11},
    "siliconflow_deepseek":  {"type_int": 12},
    "siliconflow_qwen":      {"type_int": 13},
    "chutes_deepseek":       {"type_int": 14},
    "chutes_qwen":           {"type_int": 15},
    "moonshot_kimi":         {"type_int": 16},
    "aliyun_kimi":           {"type_int": 17},
    "siliconflow_kimi":      {"type_int": 18},
    "chutes_kimi":           {"type_int": 19},
    "chutes_chatgpt":        {"type_int": 20},
    "cerebras_chatgpt":      {"type_int": 21},
    "groq_chatgpt":          {"type_int": 22},
    "fireworks_chatgpt":     {"type_int": 23},
    "novita_chatgpt":        {"type_int": 24},
    "together_chatgpt":      {"type_int": 25},
    "nebius_chatgpt":        {"type_int": 26},
    "aws_bedrock_claude":    {"type_int": 29, "needs_region": True},
    "aws_platform_claude":   {"type_int": 30, "needs_region": True},
    "azure_claude":          {"type_int": 31, "needs_endpoint": True},
    "google_vertex_gemini":  {"type_int": 32},
    "openrouter_chatgpt":    {"type_int": 33},
    "openrouter_claude":     {"type_int": 34},
    "openrouter_gemini":     {"type_int": 35},
}

# Package-level state (protected by _lock)
_lock = threading.Lock()
_pubkey_url: str = DEFAULT_PUBKEY_URL
_override_pem: Optional[str] = None
_override_kid: Optional[str] = None
_cached_key: Optional[dict] = None  # {"kid": str, "key": RSAPublicKey, "fingerprint": str}


def set_pubkey_url(url: str) -> None:
    """Override the URL used to fetch the public key."""
    global _pubkey_url, _cached_key
    with _lock:
        _pubkey_url = url
        _cached_key = None


def set_pubkey_pem(pem_str: str, kid: str) -> None:
    """Use a local PEM key instead of fetching from the network."""
    global _override_pem, _override_kid, _cached_key
    with _lock:
        _override_pem = pem_str
        _override_kid = kid
        _cached_key = None


def encrypt(
    channel_type: str,
    api_key: str,
    *,
    endpoint: Optional[str] = None,
    region: Optional[str] = None,
    fingerprint: Optional[str] = None,
) -> str:
    """Encrypt api_key for channel_type. Returns an enc:v2:... ciphertext string.

    Args:
        channel_type: One of the ChannelType constants (e.g. ChannelType.OPENAI_CHATGPT).
        api_key: The API key to encrypt. Must not be empty.
        endpoint: Azure endpoint URL (required for azure_chatgpt / azure_claude).
        region: AWS region (for aws_bedrock_claude / aws_platform_claude). Defaults to eu-central-1.
        fingerprint: Expected fingerprint prefix (hex, >= 16 chars). Raises on mismatch.

    Returns:
        enc:v2:<kid>:<wrapped_key>:<nonce>:<ciphertext> (base64url, no padding)
    """
    if channel_type not in _CHANNEL_TYPES:
        raise ValueError(f"Unknown channel type: {channel_type!r}")
    if not api_key:
        raise ValueError("api_key must not be empty")

    spec = _CHANNEL_TYPES[channel_type]
    params: dict = {}

    if spec.get("needs_endpoint"):
        if not endpoint:
            raise ValueError(f"channel type {channel_type!r} requires endpoint=")
        params["endpoint"] = _normalize_azure_endpoint(endpoint)

    if spec.get("needs_region"):
        r = region or _DEFAULT_AWS_REGION
        if r not in _VALID_AWS_REGIONS:
            raise ValueError(f"Invalid AWS region {r!r}; valid: {', '.join(_VALID_AWS_REGIONS)}")
        params["region"] = r

    loaded = _get_key()

    if fingerprint:
        _check_fingerprint(loaded, fingerprint)

    return _seal(loaded["key"], loaded["kid"], spec["type_int"], api_key, params)


# ---- Internal ----

def _seal(pub_key, kid: str, type_int: int, api_key: str, params: dict) -> str:
    payload = {"v": 2, "t": type_int, "k": api_key}
    if params:
        payload["p"] = params
    plain = json.dumps(payload, separators=(",", ":")).encode()

    aes_key = os.urandom(32)
    nonce = os.urandom(12)
    aad = f"px-egress-v2|{kid}".encode()

    aesgcm = AESGCM(aes_key)
    ct = aesgcm.encrypt(nonce, plain, aad)

    wrapped = pub_key.encrypt(
        aes_key,
        padding.OAEP(
            mgf=padding.MGF1(algorithm=hashes.SHA256()),
            algorithm=hashes.SHA256(),
            label=None,
        ),
    )

    b64 = lambda b: base64.urlsafe_b64encode(b).rstrip(b"=").decode()
    return f"enc:v2:{kid}:{b64(wrapped)}:{b64(nonce)}:{b64(ct)}"


def _get_key() -> dict:
    global _cached_key
    with _lock:
        if _cached_key is not None:
            return _cached_key
        if _override_pem:
            k = _parse_pem(_override_kid or "k1", _override_pem)
        else:
            k = _fetch_key(_pubkey_url)
        _cached_key = k
        return k


def _fetch_key(url: str) -> dict:
    if not url.startswith("https://"):
        raise ValueError("pubkey URL must be https")
    req = urllib.request.Request(url, headers={"User-Agent": "pxencrypt-python/0.1"})
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            doc = json.loads(resp.read(128 * 1024))
    except urllib.error.URLError as e:
        raise RuntimeError(f"Failed to fetch pubkey from {url}: {e}") from e
    pk = doc.get("pubkey", {})
    return _parse_pem(pk.get("kid", ""), pk.get("pem", ""))


def _parse_pem(kid: str, pem_str: str) -> dict:
    if "REPLACE_WITH_ACTUAL" in pem_str:
        raise ValueError("Placeholder public key detected — use set_pubkey_url() or set_pubkey_pem() with the real key")
    from cryptography.hazmat.primitives.serialization import load_pem_public_key
    pub_key = load_pem_public_key(pem_str.encode())
    from cryptography.hazmat.primitives.asymmetric.rsa import RSAPublicKey
    if not isinstance(pub_key, RSAPublicKey):
        raise ValueError("Public key is not RSA")
    if pub_key.key_size < 2048:
        raise ValueError(f"Public key too small ({pub_key.key_size} bits)")
    # Compute fingerprint from DER bytes.
    der = pub_key.public_bytes(
        encoding=serialization.Encoding.DER,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )
    fp = hashlib.sha256(der).hexdigest()
    return {"kid": kid, "key": pub_key, "fingerprint": fp}


def _check_fingerprint(loaded: dict, expected: str) -> None:
    expected = expected.lower().replace(":", "").strip()
    if len(expected) < 16:
        raise ValueError("Fingerprint must be at least 16 hex characters")
    if not loaded["fingerprint"].startswith(expected):
        raise ValueError(
            f"Pubkey fingerprint mismatch: expected prefix {expected}, got {loaded['fingerprint']}"
        )


def _normalize_azure_endpoint(raw: str) -> str:
    raw = raw.strip()
    parsed = urlparse(raw)
    if parsed.scheme != "https" or not parsed.netloc:
        raise ValueError(f"Azure endpoint must be https://xxx.azure.com (got {raw!r})")
    host = parsed.netloc.lower()
    host_only = host.rsplit(":", 1)[0] if ":" in host else host
    if not host_only.endswith(".azure.com"):
        raise ValueError(f"Azure endpoint must end with .azure.com (got {host!r})")
    return f"https://{host}"
