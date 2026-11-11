# Envelope Encryption

Poixe uses an RSA-OAEP + AES-256-GCM **envelope** scheme to protect API keys in transit. The plaintext key is encrypted client-side and never transmitted to Poixe servers.

## Format

```
enc:v2:<kid>:<wrapped_key>:<nonce>:<ciphertext>
```

All binary fields are **base64url encoded without padding** (RFC 4648 §5, no `=`).

| Field | Description |
|---|---|
| `enc:v2` | Version prefix — always literal |
| `kid` | Key ID — identifies which public key was used |
| `wrapped_key` | RSA-OAEP encrypted AES-256 key (256 bytes for a 2048-bit RSA key) |
| `nonce` | 12-byte random AES-GCM nonce |
| `ciphertext` | AES-256-GCM ciphertext + 16-byte GCM tag |

## Payload

The AES-GCM plaintext is a compact JSON object:

```json
{"v":2,"t":<typeInt>,"k":"<apiKey>","p":{"endpoint":"...","region":"..."}}
```

| Field | Type | Description |
|---|---|---|
| `v` | int | Envelope version — always `2` |
| `t` | int | Channel type integer (see Channel Types) |
| `k` | string | The API key |
| `p` | object | Optional routing params: `endpoint` (Azure), `region` (AWS) |

The `p` field is omitted when neither endpoint nor region applies.

## Cryptographic Parameters

**RSA-OAEP:**
- Hash: SHA-256 (for both OAEP hash and MGF1)
- Label: empty
- Minimum key size: 2048 bits

**AES-256-GCM:**
- Key: 32 random bytes, freshly generated per encryption
- Nonce: 12 random bytes, freshly generated per encryption
- Tag: 128 bits (appended to ciphertext)

**Additional Authenticated Data (AAD):**

```
px-egress-v2|<kid>
```

(UTF-8 encoded). The AAD binds the key ID to the ciphertext; a ciphertext produced with one key cannot be injected as if encrypted with another.

## Step-by-step

1. Encode the payload as compact JSON (no extra spaces).
2. Generate a 32-byte random AES key.
3. Generate a 12-byte random nonce.
4. Set `aad = "px-egress-v2|" + kid` (UTF-8).
5. Encrypt payload with AES-256-GCM using key, nonce, and aad → `ct` (includes 16-byte tag appended by GCM).
6. RSA-OAEP-SHA256 encrypt the AES key with the public key → `wrapped`.
7. Assemble: `enc:v2:<kid>:<b64url(wrapped)>:<b64url(nonce)>:<b64url(ct)>`.

## Example (illustrative)

```
enc:v2:k1:ABCD...==...:xyz...:ciphertext...
```

(Real output will be ~500+ characters.)
