package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ActiveState/vt10x"
	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"github.com/jjviana/codex/pkg/codex"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	StateNormal = iota
	StateFetchingSuggestions
	StateSuggesting
)

var (
	shellState        int
	currentSuggestion string
)

func run() error {
	c := exec.Command(shell, shellArgs...)

	// Start the shell with a pty
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Create the virtual terminal to interpret the shell output
	var state vt10x.State
	vterm, err := vt10x.Create(&state, ptmx)
	if err != nil {
		return err
	}
	defer vterm.Close()

	stdInChan := make(chan []byte)
	tty, err := NewMirrorTty(stdInChan)
	if err != nil {
		return err
	}
	go stdinToShellLoop(ptmx, stdInChan)

	// Create the screen to render the shell output
	s, err := tcell.NewTerminfoScreenFromTty(tty)
	if err != nil {
		return err
	}
	defer s.Fini()

	err = s.Init()
	if err != nil {
		return err
	}

	width, height := s.Size()
	vt10x.ResizePty(ptmx, width, height)
	vterm.Resize(width, height)

	endc := make(chan bool)
	updatec := make(chan struct{}, 1)
	go func() {
		defer close(endc)
		// Parses the shell output
		for {
			err := vterm.Parse()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				break
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
			eventc <- s.PollEvent()
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
				vt10x.ResizePty(ptmx, width, height)
				vterm.Resize(width, height)
				s.Sync()
			}
		case <-endc:
			return nil

		case <-updatec:
			updateScreen(s, &state, width, height)

		case <-time.After(1 * time.Second):
			log.Debug().Msg("shell is idle, state is " + string(shellState))
			if shellState == StateNormal {
				shellState = StateFetchingSuggestions
				go fetchSuggestions(state, updatec)
			}

		}
	}
}

func fetchSuggestions(state vt10x.State, updatec chan struct{}) {
	prompt := state.StringBeforeCursor()
	if len(prompt) > 0 {
		prompt = prompt[:len(prompt)-1] // remove the trailing newline inserted wrongly by the vt10x parser
	}
	if len(prompt) > 0 {
		log.Debug().Msgf("prompt: %s", prompt)
		suggestion, err := suggest(prompt)
		if err != nil {
			log.Error().Err(err).Msg("error fetching suggestion")
			shellState = StateNormal
			currentSuggestion = ""
			return
		}
		if shellState == StateFetchingSuggestions { // someone else might have already changed the state
			shellState = StateSuggesting
			currentSuggestion = strings.TrimRight(suggestion, " ")
			updatec <- struct{}{} // trigger a screen updateScreen
		}
	} else {
		shellState = StateNormal
		currentSuggestion = ""
	}
}

const suggestionColor = tcell.ColorDarkGray

func updateScreen(s tcell.Screen, state *vt10x.State, w, h int) {
	state.Lock()
	defer state.Unlock()
	log.Debug().Msgf("updating screen, width: %d, height: %d", w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
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
		if currentSuggestion != "" {
			style := tcell.StyleDefault.Foreground(suggestionColor)
			x := curx
			y := cury
			for i := 0; i < len(currentSuggestion); i++ {
				if currentSuggestion[i] == '\n' {
					y++
					x = 0
				}
				s.SetContent(x, y, rune(currentSuggestion[i]), nil, style)
				x++
			}
		}
	} else {
		s.HideCursor()
	}
	s.Show()
}

func stdinToShellLoop(shell io.Writer, stdin chan []byte) {
	for data := range stdin {
		log.Debug().Msgf("stdin: %+v", data)
		switch shellState {
		case StateSuggesting:
			if data[0] == '\t' && len(currentSuggestion) > 0 {
				_, err := shell.Write([]byte(currentSuggestion))
				if err != nil {
					log.Error().Err(err).Msg("failed to write to shell")
					os.Exit(1)
				}
				data = data[1:]
			}
			shellState = StateNormal
			currentSuggestion = ""
		case StateFetchingSuggestions:
			// invalidate the suggestion fetch request as it is based on a stale prompt at this point
			shellState = StateNormal
			currentSuggestion = ""
		}
		_, err := shell.Write(data)
		if err != nil {
			log.Error().Err(err).Msg("failed to write to shell")
			os.Exit(1)
		}
	}
}

var completion codex.CompletionParameters

func init() {
	completion.APIKey = os.Getenv("OPENAPI_API_KEY")
	if completion.APIKey == "" {
		log.Fatal().Msg("OPENAPI_API_KEY not set")
		os.Exit(1)
	}
	completion.MaxTokens = 64
	completion.Temperature = 0.0
	completion.Stop = []string{"\n"}
}

const (
	cushman = "cushman-codex"
	davinci = "davinci-codex"
)

func suggest(prompt string) (string, error) {
	// Try cushman first, as it is faster and cheaper
	suggestion, err := suggestWithEngine(cushman, prompt)
	if err != nil || len(suggestion) == 0 {
		// Try davinci as a fallback
		suggestion, err = suggestWithEngine(davinci, prompt)
	}
	return suggestion, err
}

func suggestWithEngine(engine, prompt string) (string, error) {
	log.Debug().Msgf("requesting suggestion to %s  with  prompt: %s", engine, prompt)

	request := completion
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

var (
	debugFile string
	shell     string
	shellArgs []string
)

// Parse args.
// -d <file>: turn on debug mode and write to file.
// -s <shell>: select the shell to use.
// -h: show help.
// -- passes the rest of the args to the shell
func parseArgs() {
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-d":
			if i+1 < len(os.Args) {
				debugFile = os.Args[i+1]
				i++
			} else {
				log.Print("-d requires an argument")
				os.Exit(1)
			}
		case "-s":
			if i+1 < len(os.Args) {
				shell = os.Args[i+1]
				i++
			} else {
				log.Print("-s requires an argument value")
				os.Exit(1)
			}
		case "-h":
			printUsage()
			os.Exit(0)
		case "--":
			shellArgs = os.Args[i+1:]
			break
		}
	}
}

func printUsage() {
	log.Printf("Usage: %s [options] [shell args]", os.Args[0])
	log.Printf("Options:")
	log.Printf("  -d <file>: turn on debug mode and write to file.")
	log.Printf("  -s shell: select shell to run (default $SHELL)")
	log.Printf("  --: pass the rest of the args to the shell.")
	log.Printf("  -h: show help.")
}

func main() {
	parseArgs()
	if shell == "" {
		// Finds the current shell based on the $SHELL environment variable
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}
	if debugFile != "" {
		// set zerolog debug level
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		f, err := os.OpenFile(debugFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		if err != nil {
			log.Err(err).Msgf("failed to open debug file %s", debugFile)
			os.Exit(1)
		}
		defer f.Close()
		// Change the zerolog global logger to write to the file.
		log.Logger = zerolog.New(f).With().Timestamp().Logger()

	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	if err := run(); err != nil {
		log.Err(err).Msgf("failed to run : %s", err)
	}
}
