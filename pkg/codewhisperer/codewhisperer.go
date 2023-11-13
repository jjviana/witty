package codewhisperer

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/jjviana/codex/pkg/codewhisperer/service"
	"github.com/jjviana/codex/pkg/engine"
	"github.com/rs/zerolog/log"
	"strings"
)

// CodeWhisperer implements a suggestion engine for the Amazon CodeWhisperer service.
type CodeWhisperer struct {
	sessionManager *SessionManager
}

// NewSuggestionEngine creates a new CodeWhisperer suggestion engine.
func NewSuggestionEngine(config configRepository, display display) (*CodeWhisperer, error) {
	sessionManager := NewSessionManager(config, display)
	err := sessionManager.Start()
	if err != nil {
		return nil, err
	}
	return &CodeWhisperer{
		sessionManager: sessionManager,
	}, nil

}

type codeWhispererSuggestion struct {
	prompt          string
	completion      *service.GenerateCompletionsOutput
	completionIndex int
}

func (s *codeWhispererSuggestion) Text() string {
	if len(s.completion.Completions) > s.completionIndex {
		// We do not want multiline suggestions as this is a terminal.
		// Use the first line only.
		content := *s.completion.Completions[s.completionIndex].Content
		newLineIndex := strings.Index(content, "\n")
		if newLineIndex >= 0 {
			content = content[:newLineIndex]
		}
		log.Debug().Msgf("Suggestion: %s", content)
		return content
	}
	return ""
}

const fileName = "script.sh"
const languageName = "shell"

// Suggest returns a suggestion for the given prompt.
func (c *CodeWhisperer) Suggest(prompt string) (engine.Suggestion, error) {

	log.Debug().Msgf("Fetching suggestions with CodeWhisperer")

	// Call the CodeWhisperer recommendation completion api.
	result, err := c.sessionManager.GenerateCompletions(&service.GenerateCompletionsInput{
		FileContext: &service.FileContext{
			Filename:         aws.String(fileName),
			LeftFileContent:  &prompt,
			RightFileContent: aws.String(""),
			ProgrammingLanguage: &service.ProgrammingLanguage{
				LanguageName: aws.String(languageName),
			},
		},
		MaxResults: aws.Int64(5),
	})
	if err != nil {
		log.Debug().Msgf("Error fetching suggestions with CodeWhisperer: %s", err)
		return nil, err
	}
	log.Debug().Msgf("Fetched %d suggestions with CodeWhisperer, next token is %s", len(result.Completions),
		aws.StringValue(result.NextToken))
	return &codeWhispererSuggestion{
		prompt:     prompt,
		completion: result,
	}, nil

}

// TopSuggestions returns the top suggestions for the given prompt and current suggestion.
func (c *CodeWhisperer) TopSuggestions(prompt string, current engine.Suggestion) ([]engine.Suggestion, error) {
	suggestion, ok := current.(*codeWhispererSuggestion)
	if !ok {
		return nil, nil
	}
	// Unroll any existing completions as additional suggestions
	suggestions := make([]engine.Suggestion, 0, len(suggestion.completion.Completions))
	for i := 0; i < len(suggestion.completion.Completions); i++ {
		suggestions = append(suggestions, &codeWhispererSuggestion{
			prompt:          prompt,
			completion:      suggestion.completion,
			completionIndex: i,
		})
	}
	if suggestion.completion.NextToken != nil {
		// There may be more suggestions, fetch them
		result, err := c.sessionManager.GenerateCompletions(&service.GenerateCompletionsInput{
			FileContext: &service.FileContext{
				Filename:         aws.String(fileName),
				LeftFileContent:  &prompt,
				RightFileContent: aws.String(""),
				ProgrammingLanguage: &service.ProgrammingLanguage{
					LanguageName: aws.String(languageName),
				},
			},
			MaxResults: aws.Int64(5),
			NextToken:  suggestion.completion.NextToken,
		})
		if err != nil {
			log.Debug().Msgf("Error fetching suggestions with CodeWhisperer: %s", err)
			return nil, err
		}
		log.Debug().Msgf("Fetched %d additional suggestions with CodeWhisperer, next token is %s", len(result.Completions),
			aws.StringValue(result.NextToken))
		for i := 0; i < len(result.Completions); i++ {
			suggestions = append(suggestions, &codeWhispererSuggestion{
				prompt:          prompt,
				completion:      result,
				completionIndex: i,
			})
		}
	}
	return suggestions, nil
}
