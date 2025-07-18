// Package main is the entry point for the admission channelog service.
// It initializes logging, loads configuration, sets up RabbitMQ connectivity,
// registers HTTP handlers for various Kubernetes resources, and starts an HTTPS server.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"channelog/config"
	"channelog/models"
	"channelog/service"
)

const port = ":8443"

// initLogger configures the global logger with console output, colorized levels,
// timestamps, and caller information in a consistent, readable format.
func initLogger() {
	// 1) Set the global minimum log level to Info.
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// 2) Create a ConsoleWriter to format logs with colors and custom part ordering.
	console := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC1123,
		NoColor:    false,
		PartsOrder: []string{
			"time",    // timestamp first
			"level",   // then log level
			"message", // then the message
			"caller",  // then the caller (file:line)
			"fields",  // finally any remaining fields
		},
	}

	// 3) Customize each part’s formatting.
	console.FormatTimestamp = func(i any) string {
		return fmt.Sprintf("\x1b[90m%s\x1b[0m |", i)
	}
	console.FormatLevel = func(i any) string {
		lvl := strings.ToLower(fmt.Sprint(i))
		padded := fmt.Sprintf("%-6s", strings.ToUpper(lvl))
		var color string
		switch lvl {
		case "info":
			color = "\x1b[32m" // green
		case "warn", "warning":
			color = "\x1b[33m" // yellow
		case "error":
			color = "\x1b[31m" // red
		default:
			color = "\x1b[37m" // white
		}
		return fmt.Sprintf("%s| %s|\x1b[0m |", color, padded)
	}
	console.FormatMessage = func(i any) string {
		return fmt.Sprintf("%s |", i)
	}
	console.FormatCaller = func(i any) string {
		return fmt.Sprintf("%s |", i)
	}
	console.FormatFieldName = func(i any) string {
		return fmt.Sprintf("%s:", i)
	}
	console.FormatFieldValue = func(i any) string {
		return fmt.Sprintf("%v |", i)
	}

	// 4) Build and assign the global logger with timestamp and caller.
	log.Logger = zerolog.New(console).
		With().
		Timestamp().
		Caller().
		Logger()
}

func main() {
	// Initialize structured, colorized logging.
	initLogger()

	// Define command-line flags for TLS certificate, key, and listen address.
	certFile := flag.String("tlsCertFile", "/certs/server.crt", "path to TLS certificate")
	keyFile := flag.String("tlsKeyFile", "/certs/server.key", "path to TLS private key")
	addr := flag.String("addr", port, "listen address (can be overridden by ADDR env var)")
	flag.Parse()

	// Load configuration from environment variables (AMQP URL, queue name, pool size).
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	openaiService := models.NewOpenAIService(cfg)
	log.Info().Msg("OpenAI service initialized")

	// Set up the Fiber HTTP server with panic recovery middleware.
	app := fiber.New()
	app.Use(recover.New())

	// Health check endpoint used by Kubernetes liveness probe.
	app.Get("/live", func(c *fiber.Ctx) error {
		return service.LivenessService(c, cfg)
	})

	// Register admission channelog endpoints.
	app.Post(("/validate"), func(c *fiber.Ctx) error {
		return service.CommitService(c, cfg, openaiService)
	})

	// Start listening with TLS, using the ADDR environment variable if set.
	listenAddr := getEnv("ADDR", *addr)
	if err := app.ListenTLS(listenAddr, *certFile, *keyFile); err != nil {
		log.Fatal().Err(err).Msg("failed to start HTTPS server")
	}
}

// getEnv returns the environment variable value if set, or the provided fallback.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
