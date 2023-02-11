package witty

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rs/zerolog/log"
)

type topChoice struct {
	text        string
	probability float64
}

func (w *Witty) showCompletionsUI() {
	if w.currentSuggestion == nil || w.currentSuggestion.Text() == "" {
		return
	}
	app := tview.NewApplication()
	list := tview.NewList()
	choices, err := w.suggestionEngine.TopSuggestions(w.getPrompt(), w.currentSuggestion)
	if err != nil {
		log.Debug().Msgf("error getting top suggestions: %v", err)
		return
	}
	shortcut := 'a'
	for _, choice := range choices {
		list.AddItem(choice.Text(), "", shortcut, nil)
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

	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		app.Stop()
		w.currentSuggestion = choices[index]
	})

	if err := app.SetRoot(list, true).SetFocus(list).Run(); err != nil {
		panic(err)
	}
}
