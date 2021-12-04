package witty

import (
	"github.com/jjviana/codex/pkg/codex"
	"github.com/rs/zerolog/log"
)

const (
	cushman = "cushman-codex"
	davinci = "davinci-codex"
)

func (w *Witty) suggest(prompt string) (string, error) {
	// Try cushman first, as it is faster and cheaper
	suggestion, err := w.suggestWithEngine(cushman, prompt)
	if err != nil || len(suggestion) == 0 {
		// Try davinci as a fallback
		suggestion, err = w.suggestWithEngine(davinci, prompt)
	}
	return suggestion, err
}

func (w *Witty) suggestWithEngine(engine, prompt string) (string, error) {
	log.Debug().Msgf("requesting suggestion to %s  with  prompt: %s", engine, prompt)

	request := w.completionParameters
	request.Prompt = prompt
	request.EngineID = engine

	completion, err := codex.GenerateCompletions(request)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("Got completions: %+v", completion)

	if len(completion.Choices) > 0 {
		choice := completion.Choices[0]
		probabilities := choice.Logprobs.TokenProbabilities()
		for i, p := range probabilities {
			log.Debug().Msgf("Token %s probability %.3f", choice.Logprobs.Tokens[i], p)
		}
		return completion.Choices[0].Text, nil
	}
	return "", nil
}
