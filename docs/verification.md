# Public Key Verification

Before encrypting, the tools fetch the Poixe public key from `security.json`. This document explains how to verify the key independently.

## security.json

The authoritative key document is served at:

```
https://poixe.com/security.json
```

```json
{
  "schema_version": 1,
  "updated_at": "2026-11-12T00:00:00Z",
  "pubkey": {
    "kid": "k1",
    "algorithm": "RSA-OAEP-SHA256+AES-256-GCM",
    "pem": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n",
    "fingerprint": "<sha256-hex>"
  },
  "egress_ips": ["..."]
}
```

The `fingerprint` field is the SHA-256 hash of the DER-encoded public key (SubjectPublicKeyInfo), hex-encoded.

## DNS TXT Verification

The first 16 characters of the fingerprint are also published as a DNS TXT record:

```
_encryption.poixe.com  TXT  "v=px1 kid=k1 fp=<first16hexchars>"
```

Verify with:

```sh
dig TXT _encryption.poixe.com +short
# or
nslookup -type=TXT _encryption.poixe.com
```

## Computing the Fingerprint Locally

```sh
# Download the PEM from security.json and save as pubkey.pem, then:
openssl pkey -pubin -in pubkey.pem -outform DER | sha256sum

# Or with Python:
python3 -c "
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.serialization import load_pem_public_key
import hashlib, sys
pem = open('pubkey.pem','rb').read()
key = load_pem_public_key(pem)
der = key.public_bytes(serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo)
print(hashlib.sha256(der).hexdigest())
"
```

## Pinning the Fingerprint

The CLI and both SDKs support passing a fingerprint prefix. If the fetched key doesn't match, the operation fails:

```sh
poixe encrypt -t openai_chatgpt --fingerprint 4a8f9b2c3d4e5f6a < api_key.txt
```

```go
// Go
pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, key, pxencrypt.WithFingerprint("4a8f9b2c3d4e5f6a"))
```

```python
# Python
encrypt(ChannelType.OPENAI_CHATGPT, key, fingerprint="4a8f9b2c3d4e5f6a")
```

The fingerprint is a prefix match (at least 16 hex chars), case-insensitive and colon-tolerant. Cross-reference the DNS TXT record against the security.json fingerprint for independent verification.
