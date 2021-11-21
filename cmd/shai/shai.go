package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/ahmetb/go-cursor"
	"github.com/creack/pty"
	"github.com/jjviana/codex/pkg/codex"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

//go:embed prompt.txt
var prompt string

func run() error {
	// Create arbitrary command.
	c := exec.Command("bash", "--login")

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.
	ich := make(chan string)
	go readerLoop(ptmx, ich)
	go writerLoop(ptmx, ich)
	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}

func readerLoop(r *os.File, out chan string) {
	b := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil {
			log.Err(err).Msgf("error reading from stdin: %s", err)
			os.Exit(1)
		}
		if n == 0 {
			continue
		}
		log.Debug().Msgf("stdin: %s", string(b[:n]))
		out <- string(b[:n])
	}
}

func writerLoop(w *os.File, in chan string) {
	var currentInput strings.Builder
	var history []string
	var suggesting bool
	var suggestion string
	var err error

	for {
		select {
		case str := <-in:
			for _, c := range str {
				switch c {
				case '\n', '\r':
					if currentInput.Len() > 0 {
						history = append(history, currentInput.String())
						log.Debug().Msgf("command: %s", currentInput.String())
						currentInput.Reset()

					}
				case 0x007f: // Backspace

					if currentInput.Len() > 0 {
						current := currentInput.String()
						currentInput.Reset()
						currentInput.WriteString(current[:len(current)-1])
					}
				case '\t':
					if suggesting && len(suggestion) > 0 {
						// Writes the suggestion and adds to the current input.
						_, _ = w.Write([]byte(suggestion))
						currentInput.WriteString(suggestion)
						suggestion = ""
						suggesting = false
						continue
					}
				default:
					// Adds to the current input if it's not a control character.
					if !unicode.IsControl(c) {
						currentInput.WriteRune(c)
					}

				}
			}
			_, err := w.Write([]byte(str))
			if err != nil {
				log.Err(err).Msgf("error writing to pty: %s", err)
				os.Exit(1)
			}
			suggesting = false
		case <-time.After(time.Second):
			log.Debug().Msgf("timeout")
			if !suggesting {
				suggesting = true
				suggestion, err = suggest(currentInput.String(), history)
				if err != nil {
					log.Err(err).Msgf("error suggesting: %s", err)
				} else {
					if suggestion != "" {
						os.Stdout.Write([]byte(Cyan + suggestion + Reset))
						os.Stdout.Write([]byte(cursor.MoveLeft(len(suggestion))))
						if err != nil {
							log.Err(err).Msgf("error writing suggestion to pty: %s", err)
						}
					}
				}
			}

		}
	}
}

var (
	Reset      = "\033[0m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Purple     = "\033[35m"
	Cyan       = "\033[36m"
	Gray       = "\033[37m"
	White      = "\033[97m"
	completion codex.CompletionParameters
)

func init() {
	completion.EngineID = "davinci-codex"
	completion.MaxTokens = 64
	completion.Stop = []string{"\n", "\r"}
	completion.Temperature = 0.0
	completion.APIKey = os.Getenv("OPENAPI_API_KEY")
	if completion.APIKey == "" {
		log.Fatal().Msg("OPENAPI_API_KEY not set")
		os.Exit(1)
	}
}

func suggest(input string, history []string) (string, error) {
	var currentPrompt strings.Builder
	currentPrompt.WriteString(prompt)

	for _, h := range history {
		format := promptLineFormat(h, false)
		currentPrompt.Write([]byte(fmt.Sprintf(format, h)))
	}
	currentPrompt.Write([]byte(fmt.Sprintf(promptLineFormat(input, true), input)))
	log.Debug().Msgf("current prompt: %s", currentPrompt.String())

	request := completion
	request.Prompt = currentPrompt.String()

	completion, err := codex.GenerateCompletions(request)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("Got completions: %+v", completion)
	if len(completion.Choices) > 0 {
		return completion.Choices[0].Text, nil
	}
	return "", nil
}

func promptLineFormat(h string, lastLine bool) string {
	var format string
	if !strings.HasPrefix(h, "#") {
		format = ">%s"
	} else {
		format = "%s"
	}
	if !lastLine {
		format = format + "\n"
	}
	return format
}

// Parse args.
// -d <file>: turn on debug mode and write to file.
// -h: show help.
func parseArgs() (string, bool) {
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-d":
			if i+1 < len(os.Args) {
				return os.Args[i+1], true
			}
		case "-h":
			printUsage()
			os.Exit(0)
		}
	}
	return "", false
}

func printUsage() {
	log.Printf("Usage: %s [-d <file>] [-h]", os.Args[0])
	log.Printf("Options:")
	log.Printf("  -d <file>: turn on debug mode and write to file.")
	log.Printf("  -h: show help.")
}

func main() {
	// Parse args.
	debugFile, debug := parseArgs()
	if debug {
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

	}

	if err := run(); err != nil {
		log.Err(err).Msgf("failed to run shai: %s", err)
	}
}
