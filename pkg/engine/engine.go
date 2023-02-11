package engine

type Suggestion interface {
	Text() string
}

type SuggestionEngine interface {
	Suggest(prompt string) (Suggestion, error)
	TopSuggestions(prompt string, current Suggestion) ([]Suggestion, error)
}
