"""Example usage of the poixe-encrypt Python SDK."""

from poixe_encrypt import ChannelType, encrypt

# Basic: encrypt an OpenAI key
cipher = encrypt(ChannelType.OPENAI_CHATGPT, "sk-proj-example")
print("OpenAI:", cipher)

# Azure ChatGPT: requires endpoint
cipher = encrypt(
    ChannelType.AZURE_CHATGPT,
    "my-azure-key",
    endpoint="https://my-resource.openai.azure.com",
)
print("Azure ChatGPT:", cipher)

# AWS Bedrock: requires region
cipher = encrypt(
    ChannelType.AWS_BEDROCK_CLAUDE,
    "AKIDEXAMPLE",
    region="us-east-1",
)
print("AWS Bedrock Claude:", cipher)

# AWS Platform Claude: key is "workspace_id api_key" space-separated
cipher = encrypt(
    ChannelType.AWS_PLATFORM_CLAUDE,
    "ws-123 AKIDEXAMPLE",
    region="eu-central-1",
)
print("AWS Platform Claude:", cipher)

# Anthropic
cipher = encrypt(ChannelType.ANTHROPIC_CLAUDE, "sk-ant-example")
print("Anthropic Claude:", cipher)

# Google Vertex Gemini: key is SA JSON (pass the JSON string)
import json
sa_json = json.dumps({
    "project_id": "my-project",
    "client_email": "sa@my-project.iam.gserviceaccount.com",
    "private_key": "-----BEGIN RSA PRIVATE KEY-----\nEXAMPLE\n-----END RSA PRIVATE KEY-----\n",
})
cipher = encrypt(ChannelType.GOOGLE_VERTEX_GEMINI, sa_json)
print("Google Vertex Gemini:", cipher)
