package witty

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rs/zerolog/log"
)

type topChoice struct {
	text        string
	probability float64
}

func (w *Witty) showCompletionsUI() {
	if w.currentSuggestion == nil || w.currentSuggestion.Text == "" {
		return
	}
	app := tview.NewApplication()
	list := tview.NewList()
	topChoices := topChoices(w.currentSuggestion.Logprobs.TopLogProbs[0])
	shortcut := 'a'
	for _, choice := range topChoices {
		list.AddItem(choice.text, fmt.Sprintf("%.0f%%", 100*choice.probability), shortcut, nil)
		shortcut++
	}
	list.SetBorder(true).SetTitle("Suggestions")
	list.SetBorderPadding(10, 10, 10, 10)
	// escape key closes the list
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		return event
	})

	prompt := w.getPrompt()
	for i, choice := range topChoices {
		go func(itemIndex int, choice topChoice) {
			s, err := w.suggest(prompt + choice.text)
			if err != nil {
				log.Debug().Msgf("error suggesting %s: %v", choice.text, err)
			}
			app.QueueUpdateDraw(func() {
				_, secondary := list.GetItemText(itemIndex)
				list.SetItemText(itemIndex, choice.text+s.Text, secondary)
			})
		}(i, choice)
	}

	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		app.Stop()
		w.currentSuggestion.Text = strings.TrimRight(mainText, " ")
	})

	if err := app.SetRoot(list, true).SetFocus(list).Run(); err != nil {
		panic(err)
	}
}

func topChoices(m map[string]float64) []topChoice {
	var choices []topChoice
	for k, v := range m {
		choices = append(choices, topChoice{k, v})
	}
	// sort by probability
	sort.Slice(choices, func(i, j int) bool {
		return choices[i].probability > choices[j].probability
	})
	// transform logprob to probability
	for i := range choices {
		choices[i].probability = math.Exp(choices[i].probability)
	}
	return choices
}
