# poixe-encrypt

Python SDK for encrypting vendor API keys for the [Poixe](https://poixe.com) platform.

Encrypt upstream model provider API keys (OpenAI, Anthropic, AWS Bedrock, etc.) **locally in your process** before submitting them to Poixe. The plaintext key never leaves your machine — only the `enc:v2:...` ciphertext is transmitted or stored.

## Installation

```sh
pip install poixe-encrypt
```

Requires Python 3.8+ and the `cryptography` package (installed automatically).

## Usage

```python
from poixe_encrypt import ChannelType, encrypt

# OpenAI
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...")

# Anthropic
cipher = encrypt(ChannelType.ANTHROPIC_CLAUDE, "sk-ant-...")

# Azure (requires endpoint)
cipher = encrypt(ChannelType.AZURE_CHATGPT, "my-key",
    endpoint="https://my-resource.openai.azure.com")

# AWS Bedrock (requires region)
cipher = encrypt(ChannelType.AWS_BEDROCK_CLAUDE, "AKID...",
    region="us-east-1")

# Google Vertex Gemini (Service Account JSON)
import json
cipher = encrypt(ChannelType.GOOGLE_VERTEX_GEMINI, json.dumps(sa_dict))

# Pin the public key fingerprint (recommended for production / CI)
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...",
    fingerprint="19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761")
```

Output is a single `enc:v2:...` string ready to submit to the Poixe System API or dashboard.

## Updating

```sh
pip install -U poixe-encrypt
```

Update when Poixe adds new channel types — older versions will not have the new type constants.

## Public key

The SDK fetches `https://poixe.com/security.json` once per process and caches the result. The key's SHA-256 fingerprint can be verified independently:

```sh
dig TXT _encryption.poixe.com +short
# "v=v2; kid=k1; fp=19bb2f0101de7bb3e1ca6fea27fa06474b37de654b0191e3430b4e30f5c6e761"
```

See the [verification guide](https://github.com/poixeai/poixe-security/blob/main/docs/verification.md) for details.

## Supported channel types

All 32+ types are available as constants on `ChannelType`. See the [full list](https://github.com/poixeai/poixe-security/blob/main/docs/channel-types.md).

## License

MIT
