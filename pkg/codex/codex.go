package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jjviana/codex/pkg/engine"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
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
	LogProbs         int
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
  "logprobs": %d,
  "stop": %s
}`, promptJSON, params.Temperature, params.MaxTokens, params.TopP, params.FrequencyPenalty, params.PresencePenalty, params.LogProbs, string(stopJSON))

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
		return completion, fmt.Errorf("request error: %+v", completion.Error)
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
	ChoiceText string   `json:"text"`
	Index      int      `json:"index"`
	Logprobs   Logprobs `json:"logprobs"`
}

func (c *Choice) Text() string {
	return c.ChoiceText
}

type Logprobs struct {
	TextOffset    []float64            `json:"text_offset"`
	TokenLogProbs []float64            `json:"token_logprobs"`
	Tokens        []string             `json:"tokens"`
	TopLogProbs   []map[string]float64 `json:"top_logprobs"`
}

func (l Logprobs) TokenProbabilities() []float64 {
	// convert from logprobs to probabilities
	probs := make([]float64, len(l.TokenLogProbs))
	for i, logprob := range l.TokenLogProbs {
		probs[i] = math.Exp(logprob)
	}
	return probs
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

type SuggestionEngine struct {
	completionParameters CompletionParameters
}

const (
	cushman = "code-cushman-001"
	davinci = "code-davinci-002"
)

func (s *SuggestionEngine) suggestWithEngine(engine, prompt string) (*Choice, error) {
	log.Debug().Msgf("requesting suggestion to %s  with  prompt: %s", engine, prompt)

	request := s.completionParameters
	request.Prompt = prompt
	request.EngineID = engine

	completion, err := GenerateCompletions(request)
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("Got completions: %+v", completion)

	if len(completion.Choices) > 0 {
		choice := completion.Choices[0]
		probabilities := choice.Logprobs.TokenProbabilities()
		for i, p := range probabilities {
			log.Debug().Msgf("Token %s probability %.3f", choice.Logprobs.Tokens[i], p)
		}
		return &completion.Choices[0], nil
	}
	return nil, nil
}
func (s *SuggestionEngine) Suggest(prompt string) (engine.Suggestion, error) {
	// Try cushman first, as it is faster and cheaper
	suggestion, err := s.suggestWithEngine(cushman, prompt)
	if err != nil || suggestion == nil || len(suggestion.ChoiceText) == 0 {
		// Try davinci as a fallback
		suggestion, err = s.suggestWithEngine(davinci, prompt)
	}
	return suggestion, err
}

type topChoice struct {
	text        string
	probability float64
}

func topChoices(m map[string]float64) []string {
	var choices []topChoice
	for k, v := range m {
		choices = append(choices, topChoice{k, v})
	}
	// sort by probability
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].probability > choices[j].probability
	})
	var returnChoices []string
	for _, c := range choices {
		returnChoices = append(returnChoices, c.text)
	}
	return returnChoices
}

func (s *SuggestionEngine) TopSuggestions(prompt string, current engine.Suggestion) ([]engine.Suggestion, error) {
	choice := current.(*Choice)
	topProbs := choice.Logprobs.TopLogProbs[0]
	topChoices := topChoices(topProbs)
	var suggestions []engine.Suggestion
	for _, c := range topChoices {
		suggestion, err := s.Suggest(prompt + c)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, nil
}

type configRepository interface {
	Store(name string, config interface{}) error
	Load(name string, config interface{}) error
}

func NewSuggestionEngine(configRepository configRepository) (*SuggestionEngine, error) {

	completionParameters := CompletionParameters{}
	err := configRepository.Load("OPENAI_COMPLETION_PARAMETERS", &completionParameters)

	if err != nil {
		completionParameters.MaxTokens = 64
		completionParameters.Temperature = 0.0
		completionParameters.Stop = []string{"\n"}
		completionParameters.LogProbs = 10
		// Read the API key from stdin
		fmt.Print("Enter OpenAI API key: ")
		reader := bufio.NewReader(os.Stdin)
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		completionParameters.APIKey = strings.TrimSpace(apiKey)

		err = configRepository.Store("OPENAI_COMPLETION_PARAMETERS", completionParameters)
		if err != nil {
			return nil, err
		}
	}
	return &SuggestionEngine{
		completionParameters: completionParameters,
	}, nil
}
