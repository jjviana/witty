package witty

import (
	"time"

	"github.com/rivo/tview"
)

func (w *Witty) showCompletionsUI() {
	app := tview.NewApplication()
	list := tview.NewList().
		AddItem("List item 1", "Some explanatory text", 'a', nil).
		AddItem("List item 2", "Some explanatory text", 'b', nil).
		AddItem("List item 3", "Some explanatory text", 'c', nil).
		AddItem("List item 4", "Some explanatory text", 'd', nil).
		AddItem("Quit", "Press to exit", 'q', func() {
			app.Stop()
		})
	go func() {
		// sleep for 5 seconds
		time.Sleep(5 * time.Second)
		app.QueueUpdateDraw(func() {
			list.AddItem("List item 5", "Some explanatory text", 'e', nil)
		})
	}()
	if err := app.SetRoot(list, true).SetFocus(list).Run(); err != nil {
		panic(err)
	}
}
