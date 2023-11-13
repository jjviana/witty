package main

import (
	_ "embed"
	"fmt"
	"github.com/jjviana/codex/pkg/codewhisperer"
	"github.com/jjviana/codex/pkg/config"
	"github.com/jjviana/codex/pkg/engine"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jjviana/codex/pkg/codex"
	"github.com/jjviana/codex/pkg/witty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type appConfig struct {
	engine    string
	color     tcell.Color
	debugFile string
	shell     string
	shellArgs []string
}

func parseArgs() *appConfig {
	conf := &appConfig{}

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-c":
			i++
			if i < len(os.Args) {
				var ok bool
				conf.color, ok = tcell.ColorNames[strings.ToLower(os.Args[i])]
				if !ok {
					log.Fatal().Msgf("invalid color %s", os.Args[i])
					os.Exit(1)
				}
			}
		case "-d":
			if i+1 < len(os.Args) {
				conf.debugFile = os.Args[i+1]
				i++
			} else {
				log.Print("-d requires an argument")
				os.Exit(1)
			}
		case "-s":
			if i+1 < len(os.Args) {
				conf.shell = os.Args[i+1]
				i++
			} else {
				log.Print("-s requires an argument value")
				os.Exit(1)
			}
		case "-h":
			printUsage()
			os.Exit(0)
		case "--":
			conf.shellArgs = os.Args[i+1:]
			break

		case "-e":
			if i+1 < len(os.Args) {
				conf.engine = os.Args[i+1]
				if conf.engine != "gpt3.5" && conf.engine != "codewhisperer" {
					log.Fatal().Msgf("invalid engine %s", conf.engine)
					os.Exit(1)
				}
				i++
			} else {
				log.Print("-e requires an argument value")
				os.Exit(1)
			}
		}
	}
	return conf
}

func printUsage() {
	log.Printf("Usage: %s [options] [shell args]", os.Args[0])
	log.Printf("Options:")
	log.Printf("  -e <engine>: Selects the completion engine. Valid values are: gpt3.5 or codewhisperer")
	log.Printf("  -d <file>: turn on debug mode and write to file.")
	log.Printf("  -s shell: select shell to run (default $SHELL)")
	log.Printf("  --: pass the rest of the args to the shell.")
	log.Printf("  -h: show help.")
}

type stdoutDisplay struct {
}

func (s stdoutDisplay) ShowMessage(msg string) {
	print(msg)
}

func main() {
	c := parseArgs()
	if c.shell == "" {
		// Finds the current shell based on the $SHELL environment variable
		c.shell = os.Getenv("SHELL")
		if c.shell == "" {
			c.shell = "/bin/sh"
		}
	}
	if c.debugFile != "" {
		// set zerolog debug level
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		f, err := os.OpenFile(c.debugFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		if err != nil {
			log.Err(err).Msgf("failed to open debug file %s", c.debugFile)
			os.Exit(1)
		}
		defer f.Close()
		// Change the zerolog global logger to write to the file.
		log.Logger = zerolog.New(f).With().Timestamp().Logger()

	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	configRepo := config.NewRepository(configDirectory())

	var e engine.SuggestionEngine
	var err error

	switch c.engine {
	case "gpt3.5":
		e, err = codex.NewSuggestionEngine(configRepo)
		if err != nil {
			fmt.Printf("failed to create codex engine: %s", err)
			return
		}

	case "codewhisperer":
		e, err = codewhisperer.NewSuggestionEngine(configRepo, stdoutDisplay{})
		if err != nil {
			fmt.Printf("failed to create codewhisperer engine: %s", err)
			return
		}

	default:
		fmt.Printf("Invalid engine specified: %s. Choose between gpt3.5 or codewhisperer\n", c.engine)
		return
	}

	w := witty.New(e, c.color, c.shell, c.shellArgs)

	if err := w.Run(); err != nil {
		log.Err(err).Msgf("failed to run : %s", err)
	}
}

func configDirectory() string {
	homeDir := os.Getenv("HOME")
	wittyDir := homeDir + "/.witty"
	dir, err := os.Stat(wittyDir)
	if err != nil {
		err = os.Mkdir(wittyDir, 0700)
		if err != nil {
			panic(err)
		}
	} else {
		if !dir.IsDir() {
			panic("File " + wittyDir + "is not a directory")
		}
	}
	return wittyDir
}
