package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/twin2ai/studio/internal/claude"
	"github.com/twin2ai/studio/internal/config"
	"github.com/twin2ai/studio/internal/gemini"
	githubclient "github.com/twin2ai/studio/internal/github"
	"github.com/twin2ai/studio/internal/gpt"
	"github.com/twin2ai/studio/internal/grok"
	"github.com/twin2ai/studio/internal/multiprovider"
	"github.com/twin2ai/studio/internal/pipeline"
	"github.com/twin2ai/studio/internal/synthesizer"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found")
	}

	// Initialize logger
	logger := setupLogger()

	// Check for subcommands
	if len(os.Args) < 2 {
		// No subcommand provided, run the default pipeline
		runPipeline(logger)
		return
	}

	// Parse subcommands
	switch os.Args[1] {
	case "synthesize":
		// Handle synthesize subcommand
		synthesizeCmd := flag.NewFlagSet("synthesize", flag.ExitOnError)
		synthesizeCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: studio synthesize [persona-name]\n")
			fmt.Fprintf(os.Stderr, "\nRegenerates synthesized.md from existing raw AI outputs.\n")
			fmt.Fprintf(os.Stderr, "If no persona name is provided, regenerates all personas.\n\n")
			synthesizeCmd.PrintDefaults()
		}

		if err := synthesizeCmd.Parse(os.Args[2:]); err != nil {
			logger.Fatalf("Failed to parse synthesize command: %v", err)
		}

		// Get optional persona name
		personaName := ""
		if synthesizeCmd.NArg() > 0 {
			personaName = synthesizeCmd.Arg(0)
		}

		runSynthesize(logger, personaName)

	case "batch":
		// Handle batch subcommand
		batchCmd := flag.NewFlagSet("batch", flag.ExitOnError)
		force := batchCmd.Bool("force", false, "Force generation even if persona already exists")
		batchCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: studio batch [options] <file.txt>\n")
			fmt.Fprintf(os.Stderr, "\nGenerates personas from a list of names in a text file.\n")
			fmt.Fprintf(os.Stderr, "The file should contain one name per line.\n")
			fmt.Fprintf(os.Stderr, "Lines starting with # are treated as comments.\n\n")
			batchCmd.PrintDefaults()
		}

		if err := batchCmd.Parse(os.Args[2:]); err != nil {
			logger.Fatalf("Failed to parse batch command: %v", err)
		}

		// Get file path
		if batchCmd.NArg() < 1 {
			batchCmd.Usage()
			os.Exit(1)
		}

		filePath := batchCmd.Arg(0)
		runBatch(logger, filePath, *force)

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Studio - Multi-AI Persona Generation Pipeline")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  studio                    Run the main pipeline (monitor for new issues)")
	fmt.Println("  studio synthesize [name]  Regenerate synthesized.md from raw AI outputs")
	fmt.Println("  studio batch <file.txt>   Generate personas from a list of names in a file")
	fmt.Println("  studio help               Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  studio                         # Run the main pipeline")
	fmt.Println("  studio synthesize              # Regenerate all personas")
	fmt.Println("  studio synthesize \"Elon Musk\"  # Regenerate specific persona")
	fmt.Println("  studio batch names.txt         # Generate personas from names in file")
	fmt.Println("  studio batch -force names.txt  # Force generation even if personas exist")
}

func runPipeline(logger *logrus.Logger) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Create and start pipeline
	p, err := pipeline.New(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create pipeline: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Shutting down studio...")
		cancel()
	}()

	// Start the pipeline
	logger.Info("Starting Studio pipeline...")
	if err := p.Start(ctx); err != nil {
		logger.Fatalf("Pipeline error: %v", err)
	}
}

func runSynthesize(logger *logrus.Logger, personaName string) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Create synthesizer
	ctx := context.Background()
	synth := synthesizer.New(cfg, logger)

	if personaName == "" {
		logger.Info("Regenerating synthesized.md for all personas...")
		if err := synth.SynthesizeAll(ctx); err != nil {
			logger.Fatalf("Failed to synthesize all personas: %v", err)
		}
	} else {
		logger.Infof("Regenerating synthesized.md for persona: %s", personaName)
		if err := synth.SynthesizeOne(ctx, personaName); err != nil {
			logger.Fatalf("Failed to synthesize persona %s: %v", personaName, err)
		}
	}

	logger.Info("Synthesis complete!")
}

func setupLogger() *logrus.Logger {
	logger := logrus.New()

	// Set log level from environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Setup dual output: console and file
	var writers []io.Writer

	// Always include console output with text formatting
	writers = append(writers, os.Stdout)

	// Add file output with JSON formatting if possible
	file, err := os.OpenFile("logs/studio.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Warnf("Failed to open log file: %v", err)
	} else {
		writers = append(writers, file)
	}

	// Create multi-writer for both console and file
	multiWriter := io.MultiWriter(writers...)
	logger.SetOutput(multiWriter)

	// Use text formatter for better console readability
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	return logger
}

func runBatch(logger *logrus.Logger, filePath string, force bool) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Create GitHub client
	githubClient := githubclient.NewClient(
		cfg.GitHub.Token,
		cfg.GitHub.Owner,
		cfg.GitHub.Repo,
		cfg.GitHub.PersonasOwner,
		cfg.GitHub.PersonasRepo,
		cfg.GitHub.PersonaLabel,
		logger,
	)

	// Create AI clients
	claudeClient := claude.NewClient(cfg.AI.Claude.APIKey, cfg.AI.Claude.Model, logger)
	geminiClient := gemini.NewClient(cfg.AI.Gemini.APIKey, cfg.AI.Gemini.Model, logger)
	grokClient := grok.NewClient(cfg.AI.Grok.APIKey, cfg.AI.Grok.Model, logger)
	gptClient := gpt.NewClient(cfg.AI.GPT.APIKey, cfg.AI.GPT.Model, logger)

	// Create multi-provider generator
	multiGenerator := multiprovider.NewGenerator(claudeClient, geminiClient, grokClient, gptClient, logger)

	// Create batch pipeline
	batchPipeline, err := pipeline.NewBatchPipeline(cfg, githubClient, multiGenerator, logger, force)
	if err != nil {
		logger.Fatalf("Failed to create batch pipeline: %v", err)
	}

	// Process the file
	ctx := context.Background()
	if err := batchPipeline.ProcessFile(ctx, filePath); err != nil {
		logger.Fatalf("Failed to process file: %v", err)
	}
}
