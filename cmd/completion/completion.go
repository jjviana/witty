package main

import (
	"fmt"
	"os"

	"github.com/jjviana/codex/pkg/codex"
	"github.com/spf13/cobra"
)

// Parse command line arguments using cobra.
func parseArgs() (*cobra.Command, error) {
	var params codex.CompletionParameters
	rootCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate OpenAI completions",
		Long:  "Generate OpenAI completions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(params)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&params.Prompt, "prompt", "p", "", "Prompt")
	rootCmd.PersistentFlags().StringVarP(&params.EngineID, "engine-id", "e", "davinci-codex", "Engine ID")
	rootCmd.PersistentFlags().Float64VarP(&params.Temperature, "temperature", "t", 0.7, "Temperature")
	rootCmd.PersistentFlags().IntVarP(&params.MaxTokens, "max-tokens", "m", 64, "Max Tokens")
	rootCmd.PersistentFlags().Float64VarP(&params.TopP, "top-p", "o", 1, "Top P")
	rootCmd.PersistentFlags().Float64VarP(&params.FrequencyPenalty, "frequency-penalty", "f", 0.0, "Frequency Penalty")
	rootCmd.PersistentFlags().Float64VarP(&params.PresencePenalty, "presence-penalty", "r", 0.0, "Presence Penalty")

	// Obtains the API key from the environment variable OPENAPI_API_KEY
	params.APIKey = os.Getenv("OPENAPI_API_KEY")
	if params.APIKey == "" {
		return nil, fmt.Errorf("API key not found in environment variable OPENAPI_API_KEY")
	}
	// prompt is required
	rootCmd.MarkPersistentFlagRequired("prompt")
	return rootCmd, nil
}

func generateCompletion(params codex.CompletionParameters) error {
	var completion codex.Completion
	completion, err := codex.GenerateCompletions(params)
	if err != nil {
		return err
	}

	fmt.Printf("ID: %s\n", completion.ID)
	fmt.Printf("Object: %s\n", completion.Object)
	fmt.Printf("Created: %d\n", completion.Created)
	fmt.Printf("Model: %s\n", completion.Model)
	fmt.Printf("Choices:\n")
	for _, choice := range completion.Choices {
		fmt.Printf("\tText: %s\n", choice.Text)
		if len(choice.Logprobs.Tokens) > 0 {
			fmt.Printf("\tLogprobs: %v\n", choice.Logprobs)
		}
	}

	return nil
}

func main() {
	rootCmd, err := parseArgs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
