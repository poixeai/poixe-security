package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
)

func runMeta(args []string) {
	fs := flag.NewFlagSet("meta", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	showTypes := fs.Bool("types", false, "list all supported channel types")
	pubkeyURL := fs.String("pubkey-url", defaultPubkeyURL, "public key URL")

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *showTypes {
		printTypeTable(os.Stdout)
		return
	}

	pub, err := fetchPubkey(context.Background(), http.DefaultClient, *pubkeyURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Public Key")
	fmt.Printf("  %-12s %s\n", "kid", pub.Kid)
	fmt.Printf("  %-12s %s\n", "fingerprint", pub.Fingerprint)
	fmt.Printf("  %-12s %s\n", "algorithm", "RSA-2048/OAEP-SHA256 + AES-256-GCM")
	fmt.Printf("  %-12s %s\n", "source", *pubkeyURL)
	fmt.Println()
	fmt.Println("DNS Verification")
	fmt.Println("  dig TXT _encryption.poixe.com")
	fmt.Println()
	fmt.Println("Run `poixe meta --types` to list all supported channel types.")
}
