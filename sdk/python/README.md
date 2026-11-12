# poixe-encrypt

Python SDK for [Poixe](https://poixe.com) API key encryption.

Encrypt your API keys **locally** before submitting them to Poixe. The plaintext key never leaves your machine — only the `enc:v2:...` ciphertext is transmitted.

## Installation

```sh
pip install poixe-encrypt
```

Requires Python 3.8+ and the `cryptography` package.

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

# Google Vertex Gemini (Service Account JSON string)
import json
sa = json.loads(open("sa.json").read())
cipher = encrypt(ChannelType.GOOGLE_VERTEX_GEMINI, json.dumps(sa))
```

Output is a single `enc:v2:...` ciphertext string.

## Public Key

The public key is fetched from `https://poixe.com/security.json` and cached for the process lifetime. It can also be pinned by fingerprint:

```python
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-...",
    fingerprint="19bb2f01...")
```

## Supported Channel Types

All 32+ channel types are available as constants on `ChannelType`. See the [full list](https://github.com/poixeai/poixe-security/blob/main/docs/channel-types.md).

## License

MIT
