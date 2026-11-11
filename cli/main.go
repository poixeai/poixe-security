package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=X.Y.Z"
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "encrypt":
		runEncrypt(os.Args[2:])
	case "meta":
		runMeta(os.Args[2:])
	case "update":
		runUpdate(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("poixe version", version)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`poixe — Poixe API key encryption tool

USAGE:
  poixe <command> [flags]

COMMANDS:
  encrypt    Encrypt an API key into an enc:v2 envelope
  meta       Show public key info and supported channel types
  update     Update poixe to the latest release
  version    Print the current version
  help       Show this help

EXAMPLES:
  # Encrypt an OpenAI key (reads from stdin)
  echo "sk-..." | poixe encrypt -t openai_chatgpt

  # Encrypt an Azure key
  echo "my-azure-key" | poixe encrypt -t azure_chatgpt --endpoint https://my-res.openai.azure.com

  # Encrypt an AWS Bedrock key with region
  echo "AKID..." | poixe encrypt -t aws_bedrock_claude --region us-east-1

  # Encrypt a Vertex Service Account JSON file
  poixe encrypt -t google_vertex_gemini --key-file sa.json

  # Pin the pubkey fingerprint (recommended for production)
  echo "sk-..." | poixe encrypt -t openai_chatgpt --fingerprint 4a8f9b2c3d4e5f6a

  # List all supported channel types
  poixe meta --types

Run "poixe <command> --help" for command-specific flags.
`)
}
