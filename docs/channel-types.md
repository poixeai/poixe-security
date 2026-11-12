# Channel Types

Each channel type maps to a provider + model family combination. Use the slug when calling the CLI or any SDK.

| Slug | Display Name | Type Int | Needs Endpoint | Needs Region |
|---|---|:---:|:---:|:---:|
| `openai_chatgpt` | OpenAI ChatGPT | 2 | | |
| `azure_chatgpt` | Azure ChatGPT | 3 | ✓ | |
| `deepseek_deepseek` | DeepSeek | 4 | | |
| `anthropic_claude` | Anthropic Claude | 5 | | |
| `google_aistudio_gemini` | Google AI Studio Gemini | 6 | | |
| `xai_grok` | xAI Grok | 7 | | |
| `volcengine_deepseek` | Volcengine DeepSeek | 8 | | |
| `volcengine_doubao` | Volcengine Doubao | 9 | | |
| `aliyun_deepseek` | Aliyun DeepSeek | 10 | | |
| `aliyun_qwen` | Aliyun Qwen | 11 | | |
| `siliconflow_deepseek` | SiliconFlow DeepSeek | 12 | | |
| `siliconflow_qwen` | SiliconFlow Qwen | 13 | | |
| `chutes_deepseek` | Chutes DeepSeek | 14 | | |
| `chutes_qwen` | Chutes Qwen | 15 | | |
| `moonshot_kimi` | Moonshot Kimi | 16 | | |
| `aliyun_kimi` | Aliyun Kimi | 17 | | |
| `siliconflow_kimi` | SiliconFlow Kimi | 18 | | |
| `chutes_kimi` | Chutes Kimi | 19 | | |
| `chutes_chatgpt` | Chutes ChatGPT | 20 | | |
| `cerebras_chatgpt` | Cerebras ChatGPT | 21 | | |
| `groq_chatgpt` | Groq ChatGPT | 22 | | |
| `fireworks_chatgpt` | Fireworks ChatGPT | 23 | | |
| `novita_chatgpt` | Novita ChatGPT | 24 | | |
| `together_chatgpt` | Together ChatGPT | 25 | | |
| `nebius_chatgpt` | Nebius ChatGPT | 26 | | |
| `aws_bedrock_claude` | AWS Bedrock Claude | 29 | | ✓ |
| `aws_platform_claude` | AWS Platform Claude | 30 | | ✓ |
| `azure_claude` | Azure Claude | 31 | ✓ | |
| `google_vertex_gemini` | Google Vertex Gemini | 32 | | |
| `openrouter_chatgpt` | OpenRouter ChatGPT | 33 | | |
| `openrouter_claude` | OpenRouter Claude | 34 | | |
| `openrouter_gemini` | OpenRouter Gemini | 35 | | |

## Special key formats

**`azure_chatgpt` / `azure_claude`** — requires `--endpoint https://your-resource.openai.azure.com`. Paths and trailing slashes are stripped automatically; the host must end in `.azure.com`.

**`aws_bedrock_claude` / `aws_platform_claude`** — requires `--region <aws-region>`. Supported regions: `eu-central-1`, `eu-west-1`, `eu-west-2`, `us-east-1`, `us-east-2`, `us-west-2`.

**`aws_platform_claude`** — the key value must be `workspace_id api_key` (two tokens separated by a single space).

**`google_vertex_gemini`** — the key is the full Service Account JSON content (must include `project_id`, `client_email`, `private_key`).

## Listing types via CLI

```sh
poixe meta --types
```

## Keeping up to date

When Poixe adds support for new channel types, the type integer must be part of the encrypted payload. **Clients on older versions will not have the new constants.** Update before encrypting keys for newly added channel types:

```sh
poixe update                                                 # CLI
go get github.com/poixeai/poixe-security/sdk/go@latest      # Go
pip install -U poixe-encrypt                                 # Python
```

The HTML tool always reflects the latest channel list when loaded from the repository.
