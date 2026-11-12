# Public Key Verification

All tools fetch the Poixe public key from `security.json` at runtime. This document explains how to independently verify the key and how to pin it in automated workflows.

## security.json

The authoritative key document is served at:

```
https://poixe.com/security.json
```

and mirrored in this repository at [`security.json`](../security.json).

```json
{
  "version": 1,
  "updated_at": "2026-11-11T19:21:16Z",
  "pubkey": {
    "kid": "k1",
    "algorithm": "RSA-OAEP-SHA256+AES-256-GCM",
    "pem": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
    "fingerprint": "19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"
  },
  "egress_ips": ["178.105.29.176", "91.99.1.101"]
}
```

The `fingerprint` is the SHA-256 hash of the DER-encoded SubjectPublicKeyInfo, hex-encoded.

## DNS TXT verification

The fingerprint is independently published as a DNS TXT record at `_encryption.poixe.com`:

```sh
dig TXT _encryption.poixe.com +short
# "v=v2; kid=k1; fp=19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"
```

This allows you to cross-check the key served over HTTPS against a record distributed through the DNS infrastructure, without trusting a single endpoint.

```sh
# Windows / systems without dig
nslookup -type=TXT _encryption.poixe.com
```

## Computing the fingerprint locally

Extract the PEM from `security.json`, save it as `pubkey.pem`, then:

```sh
# OpenSSL
openssl pkey -pubin -in pubkey.pem -outform DER | sha256sum

# Python
python3 - <<'PY'
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.serialization import load_pem_public_key
import hashlib
pem = open("pubkey.pem", "rb").read()
key = load_pem_public_key(pem)
der = key.public_bytes(serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo)
print(hashlib.sha256(der).hexdigest())
PY
```

The output must match the `fingerprint` field in `security.json` and the `fp` value in the DNS TXT record.

## Pinning the fingerprint

Pass the fingerprint to any tool to abort encryption if the fetched key does not match — useful in CI/CD pipelines or automated workflows where key substitution attacks must be detected:

```sh
# CLI
echo "sk-..." | poixe encrypt -t openai_chatgpt \
  --fingerprint 19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761
```

```go
// Go SDK
pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, key,
    pxencrypt.WithFingerprint("19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"))
```

```python
# Python SDK
encrypt(ChannelType.OPENAI_CHATGPT, key,
    fingerprint="19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761")
```

A prefix of at least 16 hex characters is accepted; the full fingerprint is recommended for production use.

## Egress IPs

The `egress_ips` field in `security.json` lists the fixed IP addresses from which Poixe dispatches upstream model API calls. Fetch the current list:

```sh
curl -s https://poixe.com/security.json | jq '.egress_ips'
```

**We recommend adding these IPs to the allowlist of your upstream provider accounts** to restrict your API keys to Poixe egress traffic only. Configuration paths:

- **OpenAI**: Organization → Settings → API → Allowed IP addresses
- **AWS**: IAM policy condition `aws:SourceIp`
- **Azure**: API key firewall rules or network restrictions on the resource
- **Anthropic / others**: contact provider support for IP restrictions

The `egress_ips` list in `security.json` is the authoritative source.
