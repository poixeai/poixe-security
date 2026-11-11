// Example usage of the pxencrypt Go SDK.
// Run: go run main.go
// (Requires a real security.json at poixe.com/security.json or set a local key.)
package main

import (
	"fmt"
	"log"

	"github.com/poixeai/poixe-security/sdk/go"
)

func main() {
	// Basic: encrypt an OpenAI key (fetches pubkey from poixe.com/security.json)
	cipher, err := pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, "sk-proj-example")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("OpenAI:", cipher)

	// Azure: requires endpoint
	cipher, err = pxencrypt.Encrypt(pxencrypt.AzureChatGPT, "my-azure-key",
		pxencrypt.WithEndpoint("https://my-resource.openai.azure.com"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Azure ChatGPT:", cipher)

	// AWS Bedrock: requires region
	cipher, err = pxencrypt.Encrypt(pxencrypt.AWSBedrockClaude, "AKIDEXAMPLE",
		pxencrypt.WithRegion("us-east-1"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("AWS Bedrock Claude:", cipher)

	// AWS Platform Claude: key is "workspace_id api_key" space-separated
	cipher, err = pxencrypt.Encrypt(pxencrypt.AWSPlatformClaude, "ws-123 AKIDEXAMPLE",
		pxencrypt.WithRegion("eu-central-1"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("AWS Platform Claude:", cipher)

	// Anthropic
	cipher, err = pxencrypt.Encrypt(pxencrypt.AnthropicClaude, "sk-ant-example")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Anthropic Claude:", cipher)

	// Pin the fingerprint for extra security (prefix match, case-insensitive)
	cipher, err = pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, "sk-proj-example",
		pxencrypt.WithFingerprint("4a8f9b2c3d4e5f6a"))
	if err != nil {
		// This would error unless the actual key matches the fingerprint prefix.
		fmt.Println("Fingerprint mismatch (expected in demo):", err)
	} else {
		fmt.Println("Pinned:", cipher)
	}
}
