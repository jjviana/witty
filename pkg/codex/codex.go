package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rs/zerolog/log"
)

type CompletionParameters struct {
	Prompt           string
	EngineID         string
	APIKey           string
	Temperature      float64
	MaxTokens        int
	TopP             float64
	FrequencyPenalty float64
	PresencePenalty  float64
	Stop             []string
}

// GenerateCompletions generates a list of possible completions for the given prompt.
func GenerateCompletions(params CompletionParameters) (Completion, error) {
	var completion Completion
	var err error
	if params.Prompt == "" {
		return completion, fmt.Errorf("prompt is required")
	}
	if params.EngineID == "" {
		return completion, fmt.Errorf("engine_id is required")
	}
	if params.APIKey == "" {
		return completion, fmt.Errorf("api_key is required")
	}
	if params.MaxTokens == 0 {
		params.MaxTokens = 64
	}
	if params.TopP == 0 {
		params.TopP = 1
	}

	url := fmt.Sprintf("%s/%s/completions", baseURL, params.EngineID)

	// Convert params.Stop into a valid json string
	stopJSON, err := json.Marshal(params.Stop)
	if err != nil {
		return completion, err
	}

	// Convert params.Prompt into a valid json string
	promptJSON, err := json.Marshal(params.Prompt)
	if err != nil {
		return completion, err
	}

	body := fmt.Sprintf(`{
  "prompt": %s,
  "temperature": %f,
  "max_tokens": %d,
  "top_p": %f,
  "frequency_penalty": %f,
  "presence_penalty": %f,
  "logprobs": 0,
  "stop": %s
}`, promptJSON, params.Temperature, params.MaxTokens, params.TopP, params.FrequencyPenalty, params.PresencePenalty, string(stopJSON))

	resp, err := httpPost(url, params.APIKey, body)
	if err != nil {
		return completion, err
	}

	log.Debug().Msgf("response: %s", resp)

	err = json.Unmarshal(resp, &completion)
	if err != nil {
		return completion, err
	}
	if len(completion.Error) > 0 {
		return completion, fmt.Errorf("request error: %_v", completion.Error)
	}

	return completion, nil
}

const baseURL = "https://api.openai.com/v1/engines"

type Completion struct {
	ID           string                 `json:"id"`
	Object       string                 `json:"object"`
	Created      int                    `json:"created"`
	Model        string                 `json:"model"`
	Choices      []Choice               `json:"choices"`
	FinishReason string                 `json:"finish_reason"`
	Error        map[string]interface{} `json:"error"`
}

type Choice struct {
	Text     string   `json:"text"`
	Index    int      `json:"index"`
	Logprobs Logprobs `json:"logprobs"`
}

type Logprobs struct {
	TextOffset    []float64 `json:"text_offset"`
	TokenLogProbs []float64 `json:"token_logprobs"`
	Tokens        []string  `json:"tokens"`
	TopLogProbs   []float64 `json:"top_logprobs"`
}

func httpPost(url, apiKey, body string) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}
