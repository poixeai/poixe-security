// Package pxencrypt provides client-side API key encryption for Poixe channels.
//
// API keys are encrypted locally into an enc:v2 envelope before being sent to
// the Poixe platform. The platform never sees the plaintext key; only the egress
// nodes hold the private key required for decryption.
//
// Quick start:
//
//	cipher, err := pxencrypt.Encrypt(pxencrypt.OpenAIChatGPT, "sk-...")
//
// For types requiring extra parameters:
//
//	cipher, err := pxencrypt.Encrypt(pxencrypt.AzureChatGPT, "key",
//	    pxencrypt.WithEndpoint("https://my-resource.openai.azure.com"))
//
//	cipher, err := pxencrypt.Encrypt(pxencrypt.AWSBedrockClaude, "AKID...",
//	    pxencrypt.WithRegion("us-east-1"))
package pxencrypt

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Channel type slug constants — use these instead of raw strings for IDE completion.
const (
	OpenAIChatGPT         = "openai_chatgpt"
	AzureChatGPT          = "azure_chatgpt"
	DeepSeekDeepSeek      = "deepseek_deepseek"
	AnthropicClaude       = "anthropic_claude"
	GoogleAIStudioGemini  = "google_aistudio_gemini"
	XAIGrok               = "xai_grok"
	VolcengineDeepSeek    = "volcengine_deepseek"
	VolcengineDoubao      = "volcengine_doubao"
	AliyunDeepSeek        = "aliyun_deepseek"
	AliyunQwen            = "aliyun_qwen"
	SiliconFlowDeepSeek   = "siliconflow_deepseek"
	SiliconFlowQwen       = "siliconflow_qwen"
	ChutesDeepSeek        = "chutes_deepseek"
	ChutesQwen            = "chutes_qwen"
	MoonshotKimi          = "moonshot_kimi"
	AliyunKimi            = "aliyun_kimi"
	SiliconFlowKimi       = "siliconflow_kimi"
	ChutesKimi            = "chutes_kimi"
	ChutesChatGPT         = "chutes_chatgpt"
	CerebrasChatGPT       = "cerebras_chatgpt"
	GroqChatGPT           = "groq_chatgpt"
	FireworksChatGPT      = "fireworks_chatgpt"
	NovitaChatGPT         = "novita_chatgpt"
	TogetherChatGPT       = "together_chatgpt"
	NebiusChatGPT         = "nebius_chatgpt"
	AWSBedrockClaude      = "aws_bedrock_claude"
	AWSPlatformClaude     = "aws_platform_claude"
	AzureClaude           = "azure_claude"
	GoogleVertexGemini    = "google_vertex_gemini"
	OpenRouterChatGPT     = "openrouter_chatgpt"
	OpenRouterClaude      = "openrouter_claude"
	OpenRouterGemini      = "openrouter_gemini"
)

// DefaultPubkeyURL is the default source for the Poixe public key.
const DefaultPubkeyURL = "https://poixe.com/security.json"

const sdkDefaultAWSRegion = "eu-central-1"

var sdkValidAWSRegions = []string{
	"eu-central-1", "eu-west-1", "eu-west-2",
	"us-east-1", "us-east-2", "us-west-2",
}

// ---- Options ----

// Option configures an Encrypt call.
type Option func(*encryptOptions)

type encryptOptions struct {
	endpoint    string
	region      string
	fingerprint string
}

// WithEndpoint sets the Azure endpoint URL (required for azure_chatgpt and azure_claude).
// Trailing slashes and paths are stripped automatically.
func WithEndpoint(ep string) Option {
	return func(o *encryptOptions) { o.endpoint = ep }
}

// WithRegion sets the AWS region (for aws_bedrock_claude and aws_platform_claude).
// Defaults to "eu-central-1" when not set.
func WithRegion(r string) Option {
	return func(o *encryptOptions) { o.region = r }
}

// WithFingerprint pins the expected pubkey fingerprint prefix (hex, >= 16 chars).
// Encrypt returns an error if the fetched key's fingerprint doesn't match.
// Recommended for production use to detect key substitution.
func WithFingerprint(fp string) Option {
	return func(o *encryptOptions) { o.fingerprint = fp }
}

// ---- Package-level key override ----

var (
	pkgMu          sync.Mutex
	pkgPubkeyURL   = DefaultPubkeyURL
	pkgOverridePEM string
	pkgOverrideKID string
	pkgCachedKey   *sdkLoadedKey
)

// SetPubkeyURL overrides the URL used to fetch the public key.
// Call once at program startup before using Encrypt.
func SetPubkeyURL(u string) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	pkgPubkeyURL = u
	pkgCachedKey = nil
}

// SetPubkeyPEM configures a local PEM key instead of fetching from the network.
// Useful for offline environments, testing, or air-gapped deployments.
func SetPubkeyPEM(pemStr, kid string) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	pkgOverridePEM = pemStr
	pkgOverrideKID = kid
	pkgCachedKey = nil
}

// ---- Main API ----

// Encrypt encrypts apiKey for the given channelType and returns an enc:v2 envelope.
//
// The envelope can be submitted directly as the key value when creating a channel
// via the Poixe API. The plaintext key never leaves the caller's process.
//
// channelType must be one of the slug constants defined in this package (e.g. OpenAIChatGPT).
// Use WithEndpoint for Azure types, WithRegion for AWS types.
func Encrypt(channelType string, apiKey string, opts ...Option) (string, error) {
	o := &encryptOptions{}
	for _, opt := range opts {
		opt(o)
	}

	spec, ok := sdkChannelTypes[channelType]
	if !ok {
		return "", fmt.Errorf("unknown channel type %q", channelType)
	}
	if apiKey == "" {
		return "", fmt.Errorf("apiKey must not be empty")
	}

	// Build envelope params.
	var ep sdkEnvelopeParams
	if spec.needsEndpoint {
		if o.endpoint == "" {
			return "", fmt.Errorf("channel type %q requires WithEndpoint(...)", channelType)
		}
		norm, err := sdkNormalizeAzureEndpoint(o.endpoint)
		if err != nil {
			return "", err
		}
		ep.Endpoint = norm
	}
	if spec.needsRegion {
		region := o.region
		if region == "" {
			region = sdkDefaultAWSRegion
		}
		if !sdkIsValidAWSRegion(region) {
			return "", fmt.Errorf("invalid AWS region %q; valid: %s", region, strings.Join(sdkValidAWSRegions, ", "))
		}
		ep.Region = region
	}

	// Get public key (cached).
	pub, err := sdkGetKey()
	if err != nil {
		return "", err
	}

	// Check fingerprint pin.
	if o.fingerprint != "" {
		if err := sdkCheckFingerprint(pub, o.fingerprint); err != nil {
			return "", err
		}
	}

	return sdkSeal(pub.Key, pub.Kid, spec.typeInt, apiKey, ep)
}

// ---- Internal: channel types ----

type sdkChannelSpec struct {
	typeInt       int
	needsEndpoint bool
	needsRegion   bool
}

var sdkChannelTypes = map[string]sdkChannelSpec{
	OpenAIChatGPT:         {typeInt: 2},
	AzureChatGPT:          {typeInt: 3, needsEndpoint: true},
	DeepSeekDeepSeek:      {typeInt: 4},
	AnthropicClaude:       {typeInt: 5},
	GoogleAIStudioGemini:  {typeInt: 6},
	XAIGrok:               {typeInt: 7},
	VolcengineDeepSeek:    {typeInt: 8},
	VolcengineDoubao:      {typeInt: 9},
	AliyunDeepSeek:        {typeInt: 10},
	AliyunQwen:            {typeInt: 11},
	SiliconFlowDeepSeek:   {typeInt: 12},
	SiliconFlowQwen:       {typeInt: 13},
	ChutesDeepSeek:        {typeInt: 14},
	ChutesQwen:            {typeInt: 15},
	MoonshotKimi:          {typeInt: 16},
	AliyunKimi:            {typeInt: 17},
	SiliconFlowKimi:       {typeInt: 18},
	ChutesKimi:            {typeInt: 19},
	ChutesChatGPT:         {typeInt: 20},
	CerebrasChatGPT:       {typeInt: 21},
	GroqChatGPT:           {typeInt: 22},
	FireworksChatGPT:      {typeInt: 23},
	NovitaChatGPT:         {typeInt: 24},
	TogetherChatGPT:       {typeInt: 25},
	NebiusChatGPT:         {typeInt: 26},
	AWSBedrockClaude:      {typeInt: 29, needsRegion: true},
	AWSPlatformClaude:     {typeInt: 30, needsRegion: true},
	AzureClaude:           {typeInt: 31, needsEndpoint: true},
	GoogleVertexGemini:    {typeInt: 32},
	OpenRouterChatGPT:     {typeInt: 33},
	OpenRouterClaude:      {typeInt: 34},
	OpenRouterGemini:      {typeInt: 35},
}

// ---- Internal: envelope ----

type sdkEnvelopeParams struct {
	Endpoint string `json:"endpoint,omitempty"`
	Region   string `json:"region,omitempty"`
}

type sdkEnvelopePayload struct {
	V      int               `json:"v"`
	Type   int               `json:"t"`
	Key    string            `json:"k"`
	Params sdkEnvelopeParams `json:"p,omitempty"`
}

func sdkSeal(pub *rsa.PublicKey, kid string, typeNum int, apiKey string, ep sdkEnvelopeParams) (string, error) {
	pl := sdkEnvelopePayload{V: 2, Type: typeNum, Key: apiKey, Params: ep}
	plain, err := json.Marshal(pl)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", fmt.Errorf("generate aes key: %w", err)
	}

	blk, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	aad := []byte("px-egress-v2|" + kid)
	ct := gcm.Seal(nil, nonce, plain, aad)

	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return "", fmt.Errorf("rsa wrap: %w", err)
	}

	b64 := base64.RawURLEncoding.EncodeToString
	return "enc:v2:" + kid + ":" + b64(wrapped) + ":" + b64(nonce) + ":" + b64(ct), nil
}

// ---- Internal: pubkey management ----

type sdkLoadedKey struct {
	Kid         string
	Key         *rsa.PublicKey
	Fingerprint string
}

func sdkGetKey() (*sdkLoadedKey, error) {
	pkgMu.Lock()
	defer pkgMu.Unlock()

	if pkgCachedKey != nil {
		return pkgCachedKey, nil
	}

	var k *sdkLoadedKey
	var err error

	if pkgOverridePEM != "" {
		k, err = sdkParsePEM(pkgOverrideKID, pkgOverridePEM)
	} else {
		k, err = sdkFetchKey(pkgPubkeyURL)
	}
	if err != nil {
		return nil, err
	}
	pkgCachedKey = k
	return k, nil
}

type sdkSecurityDoc struct {
	Pubkey struct {
		Kid         string `json:"kid"`
		PEM         string `json:"pem"`
		Fingerprint string `json:"fingerprint"`
	} `json:"pubkey"`
}

func sdkFetchKey(rawURL string) (*sdkLoadedKey, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("pubkey URL must be https")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pxencrypt-go/0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch pubkey from %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch pubkey: HTTP %d", resp.StatusCode)
	}

	var doc sdkSecurityDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128<<10)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse security.json: %w", err)
	}
	return sdkParsePEM(doc.Pubkey.Kid, doc.Pubkey.PEM)
}

func sdkParsePEM(kid, pemStr string) (*sdkLoadedKey, error) {
	if strings.Contains(pemStr, "REPLACE_WITH_ACTUAL") {
		return nil, fmt.Errorf("placeholder public key detected — use SetPubkeyURL or SetPubkeyPEM with the real key")
	}
	blk, _ := pem.Decode([]byte(pemStr))
	if blk == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}
	if rsaPub.N.BitLen() < 2048 {
		return nil, fmt.Errorf("public key too small (%d bits)", rsaPub.N.BitLen())
	}
	sum := sha256.Sum256(blk.Bytes)
	return &sdkLoadedKey{Kid: kid, Key: rsaPub, Fingerprint: hex.EncodeToString(sum[:])}, nil
}

func sdkCheckFingerprint(k *sdkLoadedKey, expected string) error {
	expected = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(expected), ":", ""))
	if len(expected) < 16 {
		return fmt.Errorf("fingerprint must be at least 16 hex chars")
	}
	if !strings.HasPrefix(k.Fingerprint, expected) {
		return fmt.Errorf("pubkey fingerprint mismatch: expected prefix %s, got %s", expected, k.Fingerprint)
	}
	return nil
}

func sdkNormalizeAzureEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("azure endpoint must be https://xxx.azure.com (got %q)", raw)
	}
	host := strings.ToLower(u.Host)
	hostOnly := host
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		hostOnly = host[:idx]
	}
	if !strings.HasSuffix(hostOnly, ".azure.com") {
		return "", fmt.Errorf("azure endpoint must end with .azure.com (got %q)", host)
	}
	return "https://" + host, nil
}

func sdkIsValidAWSRegion(r string) bool {
	for _, v := range sdkValidAWSRegions {
		if r == v {
			return true
		}
	}
	return false
}
