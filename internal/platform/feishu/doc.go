package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

type DocAdapter struct {
	Client      *http.Client
	TokenClient *TenantTokenClient
	BaseURL     string
}

type docResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Document struct {
			Title string `json:"title"`
		} `json:"document"`
	} `json:"data"`
}

type blockListResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items []docBlock `json:"items"`
	} `json:"data"`
}

type docBlock struct {
	BlockID   string        `json:"block_id"`
	BlockType int           `json:"block_type"`
	Text      richTextBlock `json:"text"`
	Heading1  richTextBlock `json:"heading1"`
	Heading2  richTextBlock `json:"heading2"`
	Heading3  richTextBlock `json:"heading3"`
	Bullet    richTextBlock `json:"bullet"`
	Ordered   richTextBlock `json:"ordered"`
	Code      richTextBlock `json:"code"`
}

type richTextBlock struct {
	Elements []richTextElement `json:"elements"`
}

type richTextElement struct {
	TextRun struct {
		Content string `json:"content"`
	} `json:"text_run"`
}

func (a DocAdapter) FetchIntake(ctx context.Context, ref platform.IntakeRef) (platform.IntakeSnapshot, error) {
	token := strings.TrimSpace(ref.Token)
	if token == "" && strings.TrimSpace(ref.URL) != "" {
		token = ParseDocToken(ref.URL)
	}
	if token == "" {
		return platform.IntakeSnapshot{}, fmt.Errorf("feishu doc intake requires doc token or doc URL")
	}
	tenantToken, err := a.tenantToken(ctx)
	if err != nil {
		return platform.IntakeSnapshot{}, err
	}
	baseURL := a.normalizedBaseURL()
	var doc docResponse
	if err := a.getJSON(ctx, baseURL+"/open-apis/docx/v1/documents/"+url.PathEscape(token), tenantToken, &doc); err != nil {
		return platform.IntakeSnapshot{}, fmt.Errorf("fetch feishu doc metadata: %w", err)
	}
	if doc.Code != 0 {
		return platform.IntakeSnapshot{}, fmt.Errorf("feishu doc metadata error %d: %s", doc.Code, doc.Msg)
	}
	var blocks blockListResponse
	blocksEndpoint := baseURL + "/open-apis/docx/v1/documents/" + url.PathEscape(token) + "/blocks/" + url.PathEscape(token) + "/children?with_descendants=true&page_size=500&document_revision_id=-1"
	if err := a.getJSON(ctx, blocksEndpoint, tenantToken, &blocks); err != nil {
		return platform.IntakeSnapshot{}, fmt.Errorf("fetch feishu doc blocks: %w", err)
	}
	if blocks.Code != 0 {
		return platform.IntakeSnapshot{}, fmt.Errorf("feishu doc blocks error %d: %s", blocks.Code, blocks.Msg)
	}
	return platform.IntakeSnapshot{
		Provider:   platform.ProviderFeishu,
		Kind:       platform.SourceFeishuDoc,
		ExternalID: token,
		Title:      doc.Data.Document.Title,
		Body:       RenderDocBlocks(blocks.Data.Items),
		URL:        ref.URL,
		FetchedAt:  time.Now().UTC(),
	}, nil
}

func ParseDocToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Path == "" {
		return trimmed
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return trimmed
}

func RenderDocBlocks(blocks []docBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		switch block.BlockType {
		case 3:
			writeDocLine(&b, "# "+richTextContent(block.Heading1))
		case 4:
			writeDocLine(&b, "## "+richTextContent(block.Heading2))
		case 5:
			writeDocLine(&b, "### "+richTextContent(block.Heading3))
		case 12:
			writeDocLine(&b, "- "+richTextContent(block.Bullet))
		case 13:
			writeDocLine(&b, "1. "+richTextContent(block.Ordered))
		default:
			content := richTextContent(block.Text)
			if content == "" {
				content = richTextContent(block.Code)
			}
			writeDocLine(&b, content)
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func writeDocLine(b *strings.Builder, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	b.WriteString(line)
	b.WriteString("\n\n")
}

func richTextContent(block richTextBlock) string {
	var b strings.Builder
	for _, element := range block.Elements {
		b.WriteString(element.TextRun.Content)
	}
	return b.String()
}

func (a DocAdapter) tenantToken(ctx context.Context) (string, error) {
	client := a.TokenClient
	if client == nil {
		client = &TenantTokenClient{Client: a.Client, BaseURL: a.BaseURL}
	}
	return client.Token(ctx)
}

func (a DocAdapter) normalizedBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return baseURL
}

func (a DocAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a DocAdapter) getJSON(ctx context.Context, endpoint, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build feishu request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send feishu request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode feishu response: %w", err)
	}
	return nil
}
