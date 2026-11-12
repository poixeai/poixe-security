# poixe-security

**Client-side encryption toolkit for Poixe vendor API keys — CLI · Go · Python · HTML**

Vendors on the Poixe platform configure upstream model provider API keys (OpenAI, Anthropic, AWS Bedrock, etc.) as *channels*. Because these keys grant direct billing access to third-party providers, Poixe requires that they be encrypted on the client before any network transmission. This repository contains the official tools to do that.

> **GitHub description:** Client-side encryption for Poixe vendor channel API keys — CLI, Go SDK, Python SDK, and browser tool

## Why client-side encryption?

Poixe operates as an egress layer between vendors and upstream model APIs. The security model is:

- **Plaintext keys never reach Poixe application servers.** Keys are encrypted on the client using the Poixe public key before any network request is made. The application backend only ever sees and stores the `enc:v2:...` ciphertext.
- **Decryption happens only in the egress layer.** The isolated, high-trust egress machines are the sole environment where keys are decrypted — at the moment an upstream API call is dispatched. No other system component has access.

## Adding a channel API key

There are two ways to submit a channel API key to Poixe:

### 1 — Web dashboard (encrypted in-browser)

In the Poixe dashboard under **Channels → Add**, paste the API key into the form. The browser fetches the current public key from `https://poixe.com/security.json`, encrypts the key locally using WebCrypto (RSA-OAEP-SHA256 + AES-256-GCM), and transmits only the `enc:v2:...` ciphertext. The Poixe application server never receives the plaintext.

### 2 — Management API (pre-encrypt locally)

The [Poixe System API](https://docs.poixe.com/cn/api-reference/system-api/overview) (`sys.poixe.com`) supports programmatic channel management. **Plaintext API keys are not accepted** — the `api_key` field must contain a pre-encrypted `enc:v2:...` ciphertext. Use any tool in this repository to produce the value before making the API call.

## Tools

| Tool | Best for |
|---|---|
| [CLI](#cli) | Shell scripts, CI/CD pipelines, one-shot use |
| [Go SDK](#go-sdk) | Go applications |
| [Python SDK](#python-sdk) | Python applications and scripts |
| [HTML tool](#html-tool) | Browser-based, no install, works fully offline after page load |

## Public key

The RSA public key used for encryption is published at:

```
https://poixe.com/security.json
```

and mirrored in this repository at [`security.json`](security.json). All tools in this repo fetch the key from the live endpoint at runtime; the mirrored copy is for reference and independent verification.

The key can be verified against a DNS TXT record published independently of HTTPS:

```sh
dig TXT _encryption.poixe.com +short
# "v=v2; kid=k1; fp=19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"
```

See [docs/verification.md](docs/verification.md) for fingerprint verification steps and key pinning.

## Egress IPs

Upstream API calls originate from a fixed set of IP addresses. **We recommend adding these IPs to the allowlist of your upstream provider accounts** to restrict your API keys to Poixe egress traffic only.

Fetch the current list from the live endpoint:

```sh
curl -s https://poixe.com/security.json | jq '.egress_ips'
```

See [docs/verification.md](docs/verification.md) for per-provider allowlist configuration guidance.

---

## CLI

### Download

Prebuilt binaries are available on the [GitHub Releases](https://github.com/poixeai/poixe-security/releases) page:

| Platform | Architectures |
|---|---|
| Linux | amd64, arm64 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64 |

Download the archive for your platform, extract, and place the `poixe` binary on your `PATH`.

### Update

```sh
poixe update
```

Self-updates to the latest GitHub release, verifies the SHA-256 checksum, and replaces the binary in place.

### Usage

```sh
# Basic
echo "sk-proj-..." | poixe encrypt -t openai_chatgpt

# Azure (requires endpoint)
echo "my-azure-key" | poixe encrypt -t azure_chatgpt \
  --endpoint https://my-resource.openai.azure.com

# AWS Bedrock (requires region)
echo "AKID..." | poixe encrypt -t aws_bedrock_claude \
  --region us-east-1

# Vertex Gemini (Service Account JSON file)
poixe encrypt -t google_vertex_gemini --key-file sa.json

# Pin the public key fingerprint (recommended for CI/CD)
echo "sk-..." | poixe encrypt -t openai_chatgpt \
  --fingerprint 19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761

# List all supported channel types
poixe meta --types
```

---

## Go SDK

### Install

```sh
go get github.com/poixeai/poixe-security/sdk/go
```

### Usage

```go
import pxencrypt "github.com/poixeai/poixe-security/sdk/go"

// Basic
cipher, err := pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, "sk-proj-...")

// Azure
cipher, err = pxencrypt.Encrypt(pxencrypt.AzureChatGPT, "key",
    pxencrypt.WithEndpoint("https://my-resource.openai.azure.com"))

// AWS Bedrock
cipher, err = pxencrypt.Encrypt(pxencrypt.AWSBedrockClaude, "AKID...",
    pxencrypt.WithRegion("us-east-1"))

// Pin fingerprint
cipher, err = pxencrypt.Encrypt(pxencrypt.AnthropicClaude, "sk-ant-...",
    pxencrypt.WithFingerprint("19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"))
```

### Updating

```sh
go get github.com/poixeai/poixe-security/sdk/go@latest
```

The Go module is tagged as `sdk/go/vX.Y.Z` and resolved automatically via the module path.

---

## Python SDK

### Install

```sh
pip install poixe-encrypt
```

Requires Python 3.8+.

### Usage

```python
from poixe_encrypt import ChannelType, encrypt

# Basic
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...")

# Azure
cipher = encrypt(ChannelType.AZURE_CHATGPT, "key",
    endpoint="https://my-resource.openai.azure.com")

# AWS Bedrock
cipher = encrypt(ChannelType.AWS_BEDROCK_CLAUDE, "AKID...",
    region="us-east-1")

# Vertex Gemini
import json
cipher = encrypt(ChannelType.GOOGLE_VERTEX_GEMINI, json.dumps(sa_dict))

# Pin fingerprint
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...",
    fingerprint="19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761")
```

### Updating

```sh
pip install -U poixe-encrypt
```

---

## HTML tool

Open `html/encryptor.html` in any modern browser — no server, no install. The page uses the browser's native WebCrypto API; the key never leaves your machine.

The tool supports all channel types including batch encryption (multiple keys at once). It fetches `https://poixe.com/security.json` for the public key on load, and can also work with a custom URL or a pasted PEM for air-gapped / local environments.

---

## Keeping clients up to date

When Poixe adds support for new channel types (new providers or model families), the type integer must be included in the encrypted payload. **Clients using an older version will not have the new type constants** and will be unable to encrypt for those channels.

The Poixe team maintains all tools in this repository and publishes new releases whenever channel types are added. Update regularly:

```sh
poixe update                                                 # CLI — self-update in place
go get github.com/poixeai/poixe-security/sdk/go@latest      # Go
pip install -U poixe-encrypt                                 # Python
# HTML — re-open the latest html/encryptor.html from the repository
```

---

## Encryption scheme

`RSA-OAEP-SHA256` + `AES-256-GCM` envelope:

```
enc:v2:<kid>:<b64url(RSA-OAEP(aesKey))>:<b64url(nonce)>:<b64url(AES-GCM(payload))>
```

Payload: `{"v":2,"t":<typeInt>,"k":"<apiKey>","p":{...}}`. AAD: `px-egress-v2|<kid>`.

Full spec: [docs/envelope-encryption.md](docs/envelope-encryption.md)

---

## Documentation

| File | Contents |
|---|---|
| [docs/channel-types.md](docs/channel-types.md) | Full channel type table, special key formats |
| [docs/verification.md](docs/verification.md) | Public key verification, DNS TXT, fingerprint pinning, egress IPs |
| [docs/envelope-encryption.md](docs/envelope-encryption.md) | Encryption format spec, cryptographic parameters |

---

## License

MIT
