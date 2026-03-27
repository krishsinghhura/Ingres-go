package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"


	"github.com/ingres/ingres-agent-go/internal/prompts"
	apitypes "github.com/ingres/ingres-agent-go/internal/types"
)

type GroqProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func NewGroqProvider() *GroqProvider {
	model := os.Getenv("GROQ_MODEL")
	if model == "" {
		model = "openai/gpt-oss-120b"
	}
	baseURL := os.Getenv("GROQ_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &GroqProvider{
		apiKey:  os.Getenv("GROQ_API_KEY"),
		model:   model,
		baseURL: baseURL,
	}
}

type GroqMessage struct {
	Role      string          `json:"role"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []GroqToolCall  `json:"tool_calls,omitempty"`
	ToolID    string          `json:"tool_call_id,omitempty"`
}

type GroqToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type GroqRequest struct {
	Model    string         `json:"model"`
	Messages []GroqMessage  `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type GroqResponse struct {
	Choices []struct {
		Message      GroqMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

func (p *GroqProvider) HandleUserQuery(ctx context.Context, userQuery string, previousChats []apitypes.ChatMessage) (string, bool, error) {
	if p.apiKey == "" {
		return "Groq API key not set", false, nil
	}

	messages := []GroqMessage{
		{Role: "system", Content: prompts.SystemInstruction},
	}

	for _, m := range previousChats {
		role := m.Role
		if role == "BOT" || role == "model" || role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		messages = append(messages, GroqMessage{Role: role, Content: m.Content})
	}
	messages = append(messages, GroqMessage{Role: "user", Content: userQuery})

	return p.callWithTools(messages)
}

func (p *GroqProvider) callWithTools(messages []GroqMessage) (string, bool, error) {
	fullURL := p.baseURL + "/chat/completions"
	
	for i := 0; i < 5; i++ { // Max 5 iterations for tool calls
		reqBody := GroqRequest{
			Model:    p.model,
			Messages: messages,
			Tools:    GetToolsSchema(),
		}

		body, _ := json.Marshal(reqBody)
		httpReq, _ := http.NewRequest("POST", fullURL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

		client := http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", false, err
		}
		
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Groq API error (status %d): %s\n", resp.StatusCode, string(respBody))
			return "", false, fmt.Errorf("groq api error: %s", string(respBody))
		}

		var gResp GroqResponse
		if err := json.Unmarshal(respBody, &gResp); err != nil {
			fmt.Printf("Groq parse error: %v\nResponse: %s\n", err, string(respBody))
			return "", false, fmt.Errorf("failed to parse groq response: %s", string(respBody))
		}


		if len(gResp.Choices) == 0 {
			return "", false, fmt.Errorf("no choice returned from groq")
		}

		msg := gResp.Choices[0].Message
		messages = append(messages, msg)

		if gResp.Choices[0].FinishReason == "tool_calls" {
			for _, tc := range msg.ToolCalls {
				result, err := ExecuteTool(tc.Function.Name, tc.Function.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
				}
				messages = append(messages, GroqMessage{
					Role:    "tool",
					ToolID:  tc.ID,
					Content: result,
				})
			}
			continue // Go for another round
		}

		return msg.Content, false, nil
	}

	return "", false, fmt.Errorf("too many tool calls")
}

func (p *GroqProvider) GetBusinessDataInterpretation(ctx context.Context, userQuery string) (*apitypes.GetBusinessDataResult, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("groq api key not set")
	}

	fullURL := p.baseURL + "/chat/completions"
	reqBody := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompts.GetBusinessDataPrompt + "\n\nUser Query: " + userQuery},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", fullURL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var gResp GroqResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, err
	}

	if len(gResp.Choices) == 0 {
		return nil, fmt.Errorf("no choice returned from groq")
	}

	outputText := gResp.Choices[0].Message.Content
	var interpretation apitypes.GetBusinessDataInterpretation
	if err := json.Unmarshal([]byte(outputText), &interpretation); err != nil {
		return nil, fmt.Errorf("failed to parse groq response: %w", err)
	}

	return &apitypes.GetBusinessDataResult{
		Interpretation: interpretation,
		Data:           nil,
	}, nil
}
