package witty

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ActiveState/vt10x"
	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"github.com/jjviana/codex/pkg/codex"
	"github.com/rs/zerolog/log"
)

const (
	StateNormal = iota
	StateFetchingSuggestions
	StateSuggesting
)

type Witty struct {
	shellCommand         string
	shellArgs            []string
	shellState           int
	currentSuggestion    string
	completionParameters codex.CompletionParameters
	terminalState        vt10x.State
	vterm                *vt10x.VT
	screen               tcell.Screen
	shellPty             *os.File
	suggestionColor      tcell.Color
}

func New(completionParameters codex.CompletionParameters, color tcell.Color, shell string, args []string) *Witty {
	return &Witty{
		shellState:           StateNormal,
		completionParameters: completionParameters,
		suggestionColor:      color,
		shellCommand:         shell,
		shellArgs:            args,
	}
}

func (w *Witty) Run() error {
	c := exec.Command(w.shellCommand, w.shellArgs...)

	// Start the shell with a pty
	var err error
	w.shellPty, err = pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = w.shellPty.Close() }() // Best effort.

	// Create the virtual terminal to interpret the shell output
	w.vterm, err = vt10x.Create(&w.terminalState, w.shellPty)
	if err != nil {
		return err
	}
	defer w.vterm.Close()

	stdInChan := make(chan []byte)
	tty, err := NewMirrorTty(stdInChan)
	if err != nil {
		return err
	}

	// Create the screen to render the shell output
	w.screen, err = tcell.NewTerminfoScreenFromTty(tty)
	if err != nil {
		return err
	}
	defer w.screen.Fini()

	err = w.screen.Init()
	if err != nil {
		return err
	}
	go w.stdinToShellLoop(stdInChan)

	width, height := w.screen.Size()
	vt10x.ResizePty(w.shellPty, width, height)
	w.vterm.Resize(width, height)

	endc := make(chan bool)
	updatec := make(chan struct{}, 1)
	go func() {
		defer close(endc)
		// Parses the shell output
		for {
			err := w.vterm.Parse()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				break
			}
			if w.shellState == StateSuggesting {
				// Reset the state as output has change
				w.shellState = StateNormal
				w.currentSuggestion = ""
			}
			select {
			case updatec <- struct{}{}:
			default:
			}
		}
	}()

	// Polls input events from the screen
	eventc := make(chan tcell.Event, 4)
	go func() {
		for {
			eventc <- w.screen.PollEvent()
		}
	}()

	// Main event loop
	for {
		select {
		case event := <-eventc:
			switch ev := event.(type) {
			case *tcell.EventResize:
				width, height = ev.Size()
				log.Debug().Msgf("Resize: %d x %d", width, height)
				vt10x.ResizePty(w.shellPty, width, height)
				w.vterm.Resize(width, height)
				w.screen.Sync()
			}
		case <-endc:
			return nil

		case <-updatec:
			w.updateScreen(w.screen, &w.terminalState, width, height)

		case <-time.After(1 * time.Second):
			log.Debug().Msg("shell is idle, state is " + string(w.shellState))
			if w.shellState == StateNormal {
				w.shellState = StateFetchingSuggestions
				go w.fetchSuggestions(updatec)
			}

		}
	}
}

func (w *Witty) fetchSuggestions(updatec chan struct{}) {
	prompt := getPrompt(w.terminalState)
	if len(prompt) > 0 {
		log.Debug().Msgf("prompt: %s", prompt)
		suggestion, err := w.suggest(prompt)
		if err != nil {
			log.Error().Err(err).Msg("error fetching suggestion")
			w.shellState = StateNormal
			w.currentSuggestion = ""
			return
		}
		if w.shellState == StateFetchingSuggestions { // someone else might have already changed the state
			w.shellState = StateSuggesting
			w.currentSuggestion = strings.TrimRight(suggestion, " ")
			updatec <- struct{}{} // trigger a screen updateScreen
		}
	} else {
		w.shellState = StateNormal
		w.currentSuggestion = ""
	}
}

func getPrompt(state vt10x.State) string {
	prompt := state.StringBeforeCursor()
	if len(prompt) > 0 {
		prompt = prompt[:len(prompt)-1] // remove the trailing newline inserted wrongly by the vt10x parser
	}
	return prompt
}

func (w *Witty) updateScreen(s tcell.Screen, state *vt10x.State, width, height int) {
	state.Lock()
	defer state.Unlock()
	log.Debug().Msgf("updating screen, width: %d, height: %d", width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c, fg, bg := state.Cell(x, y)

			style := tcell.StyleDefault
			if fg != vt10x.DefaultFG {
				style = style.Foreground(tcell.Color(fg))
			}
			if bg != vt10x.DefaultBG {
				style = style.Background(tcell.Color(bg))
			}

			s.SetContent(x, y, c, nil, style)

		}
	}
	if state.CursorVisible() {
		curx, cury := state.Cursor()
		s.ShowCursor(curx, cury)
		if w.currentSuggestion != "" {
			style := tcell.StyleDefault.Foreground(w.suggestionColor)
			x := curx
			y := cury
			for i := 0; i < len(w.currentSuggestion); i++ {
				if w.currentSuggestion[i] == '\n' {
					y++
					x = 0
				}
				s.SetContent(x, y, rune(w.currentSuggestion[i]), nil, style)
				x++
			}
		}
	} else {
		s.HideCursor()
	}
	s.Show()
}

func (w *Witty) stdinToShellLoop(stdin chan []byte) {
	for data := range stdin {
		log.Debug().Msgf("stdin: %+v", data)
		switch w.shellState {
		case StateSuggesting:
			if data[0] == '\t' && len(w.currentSuggestion) > 0 {
				_, err := w.shellPty.Write([]byte(w.currentSuggestion))
				if err != nil {
					log.Error().Err(err).Msg("failed to write to shell")
					os.Exit(1)
				}
				data = data[1:]
			}

			w.shellState = StateNormal
			w.currentSuggestion = ""
		case StateFetchingSuggestions:
			// invalidate the suggestion fetch request as it is based on a stale prompt at this point
			w.shellState = StateNormal
			w.currentSuggestion = ""
		}
		_, err := w.shellPty.Write(data)
		if err != nil {
			log.Error().Err(err).Msg("failed to write to shell")
			os.Exit(1)
		}
	}
}
