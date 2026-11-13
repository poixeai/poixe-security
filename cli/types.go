package main

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
)

// channelSpec describes a channel type's requirements.
type channelSpec struct {
	TypeInt       int
	DisplayName   string
	NeedsEndpoint bool // azure: per-line input format is "endpoint apikey"
	NeedsRegion   bool // aws types: requires --region (default: eu-central-1)
	IsVertex      bool // google_vertex_gemini: key is SA JSON file
	IsAWSPlatform bool // aws_platform_claude: key format "workspace_id api_key"
}

// channelTypes maps slug → spec. Slugs match ChannelTypeSlug() in the platform.
var channelTypes = map[string]channelSpec{
	"openai_chatgpt":         {TypeInt: 2, DisplayName: "OpenAI ChatGPT"},
	"azure_chatgpt":          {TypeInt: 3, DisplayName: "Azure ChatGPT", NeedsEndpoint: true},
	"deepseek_deepseek":      {TypeInt: 4, DisplayName: "DeepSeek DeepSeek"},
	"anthropic_claude":       {TypeInt: 5, DisplayName: "Anthropic Claude"},
	"google_aistudio_gemini": {TypeInt: 6, DisplayName: "Google AI Studio Gemini"},
	"xai_grok":               {TypeInt: 7, DisplayName: "xAI Grok"},
	"volcengine_deepseek":    {TypeInt: 8, DisplayName: "Volcengine DeepSeek"},
	"volcengine_doubao":      {TypeInt: 9, DisplayName: "Volcengine Doubao"},
	"aliyun_deepseek":        {TypeInt: 10, DisplayName: "Aliyun DeepSeek"},
	"aliyun_qwen":            {TypeInt: 11, DisplayName: "Aliyun Qwen"},
	"siliconflow_deepseek":   {TypeInt: 12, DisplayName: "SiliconFlow DeepSeek"},
	"siliconflow_qwen":       {TypeInt: 13, DisplayName: "SiliconFlow Qwen"},
	"chutes_deepseek":        {TypeInt: 14, DisplayName: "Chutes DeepSeek"},
	"chutes_qwen":            {TypeInt: 15, DisplayName: "Chutes Qwen"},
	"moonshot_kimi":          {TypeInt: 16, DisplayName: "Moonshot Kimi"},
	"aliyun_kimi":            {TypeInt: 17, DisplayName: "Aliyun Kimi"},
	"siliconflow_kimi":       {TypeInt: 18, DisplayName: "SiliconFlow Kimi"},
	"chutes_kimi":            {TypeInt: 19, DisplayName: "Chutes Kimi"},
	"chutes_chatgpt":         {TypeInt: 20, DisplayName: "Chutes ChatGPT"},
	"cerebras_chatgpt":       {TypeInt: 21, DisplayName: "Cerebras ChatGPT"},
	"groq_chatgpt":           {TypeInt: 22, DisplayName: "Groq ChatGPT"},
	"fireworks_chatgpt":      {TypeInt: 23, DisplayName: "Fireworks ChatGPT"},
	"novita_chatgpt":         {TypeInt: 24, DisplayName: "Novita ChatGPT"},
	"together_chatgpt":       {TypeInt: 25, DisplayName: "Together ChatGPT"},
	"nebius_chatgpt":         {TypeInt: 26, DisplayName: "Nebius ChatGPT"},
	"aws_bedrock_claude":     {TypeInt: 29, DisplayName: "AWS Bedrock Claude", NeedsRegion: true},
	"aws_platform_claude":    {TypeInt: 30, DisplayName: "Claude Platform on AWS", NeedsRegion: true, IsAWSPlatform: true},
	"azure_claude":           {TypeInt: 31, DisplayName: "Azure Claude", NeedsEndpoint: true},
	"google_vertex_gemini":   {TypeInt: 32, DisplayName: "Google Vertex Gemini", IsVertex: true},
	"openrouter_chatgpt":     {TypeInt: 33, DisplayName: "OpenRouter ChatGPT"},
	"openrouter_claude":      {TypeInt: 34, DisplayName: "OpenRouter Claude"},
	"openrouter_gemini":      {TypeInt: 35, DisplayName: "OpenRouter Gemini"},
}

// validAWSRegions are the allowed AWS regions for Bedrock/Claude Platform.
var validAWSRegions = []string{
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"us-east-1",
	"us-east-2",
	"us-west-2",
}

const defaultAWSRegion = "eu-central-1"

func isValidAWSRegion(r string) bool {
	for _, v := range validAWSRegions {
		if r == v {
			return true
		}
	}
	return false
}

// normalizeAzureEndpoint strips path/query/trailing-slash from an Azure endpoint URL.
// The host must end with .azure.com and the scheme must be https.
func normalizeAzureEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("azure endpoint must be https://xxx.azure.com (got %q)", raw)
	}
	host := strings.ToLower(u.Host)
	// Strip port if present, then validate suffix.
	hostOnly := host
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		hostOnly = host[:idx]
	}
	if !strings.HasSuffix(hostOnly, ".azure.com") {
		return "", fmt.Errorf("azure endpoint host must end with .azure.com (got %q)", host)
	}
	return "https://" + host, nil
}

// printTypeTable writes a formatted table of all channel types to w.
func printTypeTable(w io.Writer) {
	type row struct {
		slug    string
		name    string
		extra   string
	}
	rows := make([]row, 0, len(channelTypes))
	for slug, spec := range channelTypes {
		extra := ""
		switch {
		case spec.NeedsEndpoint:
			extra = "[key format: endpoint apikey]"
		case spec.NeedsRegion && spec.IsAWSPlatform:
			extra = "[--region, default: " + defaultAWSRegion + "; key: workspace_id api_key]"
		case spec.NeedsRegion:
			extra = "[--region, default: " + defaultAWSRegion + "]"
		case spec.IsVertex:
			extra = "[SA JSON file]"
		}
		rows = append(rows, row{slug, spec.DisplayName, extra})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].slug < rows[j].slug })

	fmt.Fprintln(w, "Supported channel types:")
	fmt.Fprintln(w)
	for _, r := range rows {
		if r.extra != "" {
			fmt.Fprintf(w, "  %-30s  %-30s  %s\n", r.slug, r.name, r.extra)
		} else {
			fmt.Fprintf(w, "  %-30s  %s\n", r.slug, r.name)
		}
	}
}
