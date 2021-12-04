package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jjviana/codex/pkg/codex"
	"github.com/jjviana/codex/pkg/witty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var completionParameters codex.CompletionParameters

func init() {
	completionParameters.APIKey = os.Getenv("OPENAPI_API_KEY")
	if completionParameters.APIKey == "" {
		log.Fatal().Msg("OPENAPI_API_KEY not set")
		os.Exit(1)
	}
	completionParameters.MaxTokens = 64
	completionParameters.Temperature = 0.0
	completionParameters.Stop = []string{"\n"}
}

var (
	debugFile string
	shell     string
	shellArgs []string
)

// Parse args.
// -d <file>: turn on debug mode and write to file.
// -s <shell>: select the shell to use.
// -c <color>: select suggested completionParameters color.
// -h: show help.
// -- passes the rest of the args to the shell
func parseArgs() (color tcell.Color, shell string, shellArgs []string) {
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-c":
			i++
			if i < len(os.Args) {
				var ok bool
				color, ok = tcell.ColorNames[strings.ToLower(os.Args[i])]
				if !ok {
					log.Fatal().Msgf("invalid color %s", os.Args[i])
					os.Exit(1)
				}
			}
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
	return
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
	suggestionColor, shell, shellArgs := parseArgs()
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

	w := witty.New(completionParameters, suggestionColor, shell, shellArgs)

	if err := w.Run(); err != nil {
		log.Err(err).Msgf("failed to run : %s", err)
	}
}
