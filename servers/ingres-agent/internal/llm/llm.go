package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ingres/ingres-agent-go/internal/prompts"
	apitypes "github.com/ingres/ingres-agent-go/internal/types"
)

type Provider interface {
	HandleUserQuery(ctx context.Context, userQuery string, previousChats []apitypes.ChatMessage) (string, bool, error)
	GetBusinessDataInterpretation(ctx context.Context, userQuery string) (*apitypes.GetBusinessDataResult, error)
}

var currentProvider Provider

func GetProvider() Provider {
	if currentProvider != nil {
		return currentProvider
	}

	providerType := os.Getenv("LLM_PROVIDER")
	if providerType == "" {
		providerType = "gemini"
	}

	switch providerType {
	case "groq":
		currentProvider = NewGroqProvider()
	default:
		currentProvider = NewGeminiProvider()
	}
	return currentProvider
}

type GeminiProvider struct {
	apiKey string
}

func NewGeminiProvider() *GeminiProvider {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	return &GeminiProvider{apiKey: key}
}

func (p *GeminiProvider) HandleUserQuery(ctx context.Context, userQuery string, previousChats []apitypes.ChatMessage) (string, bool, error) {
	if p.apiKey == "" {
		return fmt.Sprintf("Agent response to: %s", userQuery), false, nil
	}

	resp, err := p.callGeminiAPI(userQuery, previousChats)
	if err != nil {
		fmt.Printf("gemini error: %v\n", err)
		return fmt.Sprintf("Agent response to: %s", userQuery), false, nil
	}

	return resp, false, nil
}


type GeminiRequest struct {
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	SystemInstruction struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"systemInstruction"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

func (p *GeminiProvider) callGeminiAPI(userQuery string, previousChats []apitypes.ChatMessage) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", p.apiKey)


	req := GeminiRequest{
		SystemInstruction: struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			Parts: []struct {
				Text string `json:"text"`
			}{
				{Text: prompts.SystemInstruction},
			},
		},
	}

	req.Contents = make([]struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}, 0)

	for _, chat := range previousChats {
		role := "user"
		if chat.Role == "assistant" || chat.Role == "model" {
			role = "model"
		}
		req.Contents = append(req.Contents, struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			Role: role,
			Parts: []struct {
				Text string `json:"text"`
			}{
				{Text: chat.Content},
			},
		})
	}

	req.Contents = append(req.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{
		Role: "user",
		Parts: []struct {
			Text string `json:"text"`
		}{
			{Text: userQuery},
		},
	})

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var gResp GeminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return "", err
	}

	if len(gResp.Candidates) > 0 && len(gResp.Candidates[0].Content.Parts) > 0 {
		return gResp.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("no response from gemini")
}

func (p *GeminiProvider) GetBusinessDataInterpretation(ctx context.Context, userQuery string) (*apitypes.GetBusinessDataResult, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("gemini api key not set")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", p.apiKey)


	req := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompts.GetBusinessDataPrompt + "\n\nUser Query: " + userQuery},
				},
			},
		},
		"generationConfig": map[string]string{
			"responseMimeType": "application/json",
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var gResp GeminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, err
	}

	outputText := ""
	if len(gResp.Candidates) > 0 && len(gResp.Candidates[0].Content.Parts) > 0 {
		outputText = gResp.Candidates[0].Content.Parts[0].Text
	}

	// Clean JSON if wrapped in markdown
	outputText = strings.TrimPrefix(outputText, "```json\n")
	outputText = strings.TrimSuffix(outputText, "\n```")

	var interpretation apitypes.GetBusinessDataInterpretation
	if err := json.Unmarshal([]byte(outputText), &interpretation); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	return &apitypes.GetBusinessDataResult{
		Interpretation: interpretation,
		Data:           nil,
	}, nil
}
