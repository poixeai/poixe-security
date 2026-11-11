package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const maxKeysPerRun = 200

// stringSlice is a repeatable flag value.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func runEncrypt(args []string) {
	fs := flag.NewFlagSet("encrypt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var keyFiles stringSlice
	typSlug := fs.String("type", "", "channel type slug (required), e.g. openai_chatgpt")
	fs.StringVar(typSlug, "t", "", "channel type slug (shorthand)")
	endpoint := fs.String("endpoint", "", "Azure endpoint URL, e.g. https://my-resource.openai.azure.com")
	region := fs.String("region", defaultAWSRegion, "AWS region (aws_bedrock_claude / aws_platform_claude)")
	fingerprint := fs.String("fingerprint", "", "expected pubkey fingerprint prefix (hex, >= 16 chars)")
	pubkeyURL := fs.String("pubkey-url", defaultPubkeyURL, "public key URL")
	pubkeyFile := fs.String("pubkey-file", "", "local PEM file (requires --kid)")
	kid := fs.String("kid", "", "key ID for --pubkey-file")
	fs.Var(&keyFiles, "key-file", "key file path (repeatable; Vertex: whole file = one key)")

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *typSlug == "" {
		fmt.Fprintln(os.Stderr, "error: --type is required")
		fmt.Fprintln(os.Stderr, "\nRun `poixe meta --types` to list all supported channel types.")
		os.Exit(2)
	}

	spec, ok := channelTypes[*typSlug]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown channel type %q\n", *typSlug)
		fmt.Fprintln(os.Stderr, "\nRun `poixe meta --types` to list all supported channel types.")
		os.Exit(2)
	}

	// --- Validate type-specific flags ---
	var azureEndpoint string
	if spec.NeedsEndpoint {
		if *endpoint == "" {
			fmt.Fprintf(os.Stderr, "error: --endpoint is required for type %q\n", *typSlug)
			os.Exit(2)
		}
		var err error
		azureEndpoint, err = normalizeAzureEndpoint(*endpoint)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	}

	if spec.NeedsRegion {
		if !isValidAWSRegion(*region) {
			fmt.Fprintf(os.Stderr, "error: invalid --region %q\nvalid values: %s\n",
				*region, strings.Join(validAWSRegions, ", "))
			os.Exit(2)
		}
	}

	// --- Load public key ---
	var pub *loadedKey
	var err error
	if *pubkeyFile != "" {
		pub, err = loadPubkeyFile(*pubkeyFile, *kid)
	} else {
		pub, err = fetchPubkey(context.Background(), http.DefaultClient, *pubkeyURL)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := checkFingerprint(pub, *fingerprint); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *fingerprint == "" {
		fmt.Fprintf(os.Stderr, "warning: --fingerprint not set; consider pinning it to detect key substitution\n")
	}
	fmt.Fprintf(os.Stderr, "pubkey kid=%s fingerprint=%s\n", pub.Kid, pub.Fingerprint)

	// --- Read keys ---
	inputs, err := readInputs(keyFiles, spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "error: no keys provided (stdin was empty)")
		os.Exit(1)
	}
	if len(inputs) > maxKeysPerRun {
		fmt.Fprintf(os.Stderr, "error: too many keys (%d); max %d per run\n", len(inputs), maxKeysPerRun)
		os.Exit(1)
	}

	// --- Encrypt ---
	for i, inp := range inputs {
		ep := envelopeParams{
			Endpoint: azureEndpoint,
		}
		if spec.NeedsRegion {
			ep.Region = *region
		}

		cipher, err := seal(pub.Key, pub.Kid, spec.TypeInt, inp.key, ep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: key #%d failed to encrypt: %v\n", i+1, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, cipher)
	}
}

type parsedInput struct {
	key string
}

// readInputs reads and validates key inputs.
func readInputs(keyFiles stringSlice, spec channelSpec) ([]parsedInput, error) {
	var rawLines []string

	if len(keyFiles) == 0 {
		// Read from stdin.
		if spec.IsVertex {
			// Vertex: whole stdin = one key (SA JSON can span multiple lines).
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("read stdin: %w", err)
			}
			raw := strings.TrimSpace(string(b))
			if raw == "" {
				return nil, nil
			}
			rawLines = []string{raw}
		} else {
			lines, err := scanLines(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("read stdin: %w", err)
			}
			rawLines = lines
		}
	} else {
		for _, path := range keyFiles {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read file %q: %w", path, err)
			}
			if spec.IsVertex {
				// Each file is one Vertex SA JSON.
				raw := strings.TrimSpace(string(b))
				if raw != "" {
					rawLines = append(rawLines, raw)
				}
			} else {
				lines, err := scanLines(strings.NewReader(string(b)))
				if err != nil {
					return nil, fmt.Errorf("read file %q: %w", path, err)
				}
				rawLines = append(rawLines, lines...)
			}
		}
	}

	var out []parsedInput
	for i, raw := range rawLines {
		n := i + 1
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		switch {
		case spec.IsVertex:
			// Validate SA JSON has required fields.
			var sa map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &sa); err != nil {
				return nil, fmt.Errorf("key #%d: not valid JSON (Vertex requires a Service Account JSON)", n)
			}
			for _, f := range []string{"project_id", "client_email", "private_key"} {
				v, _ := sa[f].(string)
				if v == "" {
					return nil, fmt.Errorf("key #%d: Service Account JSON missing field %q", n, f)
				}
			}
			// Compact the JSON.
			compacted, err := json.Marshal(sa)
			if err != nil {
				return nil, fmt.Errorf("key #%d: compact JSON: %w", n, err)
			}
			out = append(out, parsedInput{key: string(compacted)})

		case spec.IsAWSPlatform:
			// Format: "workspace_id api_key"
			parts := strings.Fields(raw)
			if len(parts) != 2 {
				return nil, fmt.Errorf("key #%d: aws_platform_claude expects \"workspace_id api_key\" (two space-separated tokens)", n)
			}
			out = append(out, parsedInput{key: parts[0] + " " + parts[1]})

		default:
			out = append(out, parsedInput{key: raw})
		}
	}
	return out, nil
}

func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, sc.Err()
}
