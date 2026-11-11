# poixe-security

Client-side API key encryption toolkit for the [Poixe](https://poixe.com) platform.

Encrypt your API keys **locally** before submitting them. The plaintext key never leaves your machine — only the `enc:v2:...` ciphertext is transmitted.

## Tools

| | Description |
|---|---|
| **CLI** (`poixe`) | Command-line tool — pipe a key in, get a ciphertext out |
| **Go SDK** (`sdk/go`) | Go package `pxencrypt` with typed channel constants |
| **Python SDK** (`sdk/python`) | PyPI package `poixe-encrypt` |
| **HTML tool** (`html/encryptor.html`) | Single-file offline encryptor, runs in any browser |

## CLI

```sh
# macOS / Linux
brew install poixeai/tap/poixe   # coming soon
# or download from GitHub Releases

# Encrypt an OpenAI key
echo "sk-proj-..." | poixe encrypt -t openai_chatgpt

# Azure (requires endpoint)
echo "my-key" | poixe encrypt -t azure_chatgpt --endpoint https://my-resource.openai.azure.com

# AWS Bedrock (requires region)
echo "AKID..." | poixe encrypt -t aws_bedrock_claude --region us-east-1

# Vertex Gemini (Service Account JSON file)
poixe encrypt -t google_vertex_gemini --key-file sa.json

# Pin the public key fingerprint (recommended for CI/CD)
echo "sk-..." | poixe encrypt -t openai_chatgpt --fingerprint 4a8f9b2c3d4e5f6a

# List all supported channel types
poixe meta --types
```

Output is a single `enc:v2:...` ciphertext line (or one per key when using `--key-file` multiple times).

## Go SDK

```sh
go get github.com/poixeai/poixe-security/sdk/go
```

```go
import pxencrypt "github.com/poixeai/poixe-security/sdk/go"

cipher, err := pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, "sk-proj-...")
// enc:v2:k1:...

// Azure
cipher, err = pxencrypt.Encrypt(pxencrypt.AzureChatGPT, "key",
    pxencrypt.WithEndpoint("https://my-resource.openai.azure.com"))

// AWS Bedrock
cipher, err = pxencrypt.Encrypt(pxencrypt.AWSBedrockClaude, "AKID...",
    pxencrypt.WithRegion("us-east-1"))

// Pin fingerprint
cipher, err = pxencrypt.Encrypt(pxencrypt.AnthropicClaude, "sk-ant-...",
    pxencrypt.WithFingerprint("4a8f9b2c3d4e5f6a"))
```

## Python SDK

```sh
pip install poixe-encrypt
```

```python
from poixe_encrypt import ChannelType, encrypt

cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...")

cipher = encrypt(ChannelType.AZURE_CHATGPT, "key",
    endpoint="https://my-resource.openai.azure.com")

cipher = encrypt(ChannelType.AWS_BEDROCK_CLAUDE, "AKID...",
    region="us-east-1")
```

## HTML Tool

Open `html/encryptor.html` in any browser — no server needed, no network required after the page loads. Uses the browser's native WebCrypto API.

## Encryption Scheme

`RSA-OAEP-SHA256` + `AES-256-GCM` envelope:

```
enc:v2:<kid>:<b64url(RSA-OAEP(aesKey))>:<b64url(nonce)>:<b64url(AES-GCM(payload))>
```

Payload is `{"v":2,"t":<typeInt>,"k":"<apiKey>","p":{...}}`. AAD is `px-egress-v2|<kid>`.

See [docs/envelope-encryption.md](docs/envelope-encryption.md) for the full spec.

## Public Key

The public key is published at `https://poixe.com/security.json` and mirrored in this repository (`security.json`). It is independently verifiable via DNS:

```sh
dig TXT _encryption.poixe.com +short
```

See [docs/verification.md](docs/verification.md) for fingerprint verification steps.

## Channel Types

32 channel types are supported. See [docs/channel-types.md](docs/channel-types.md) for the full list, or run `poixe meta --types`.

## License

MIT
