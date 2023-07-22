package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
	"golang.org/x/term"
)

type Config struct {
	dir           string
	disableBrotli bool
	disableGzip   bool
	enableNgrok   bool
	logging       bool
	passwordStdin bool
	port          string
	user          string
}

type App struct {
	config *Config
	server *http.Server
	wg     sync.WaitGroup
}

func main() {
	cfg := &Config{}
	parseFlags(cfg)

	fileServer, err := createFileServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create file server: %v", err)
	}

	app := &App{config: cfg}
	addr := fmt.Sprintf(":%s", cfg.port)
	if err := app.startServer(context.Background(), addr, fileServer); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	waitForShutdownSignal()

	if err := app.shutdownGracefully(context.Background()); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
}

func parseFlags(cfg *Config) {
	flag.StringVar(&cfg.dir, "dir", ".", "directory to serve")
	flag.BoolVar(&cfg.disableGzip, "no-gzip", false, "disable gzip compression")
	flag.BoolVar(&cfg.disableBrotli, "no-brotli", false, "disable brotli compression")
	flag.BoolVar(&cfg.enableNgrok, "ngrok", false, "expose the server to the internet using ngrok")
	flag.BoolVar(&cfg.logging, "no-logging", false, "disable request logging")
	flag.BoolVar(&cfg.passwordStdin, "password-stdin", false, "read password from stdin")
	flag.StringVar(&cfg.port, "port", "8080", "port to listen on")
	flag.StringVar(&cfg.user, "user", "admin", "username for basic auth")
	flag.Parse()
}

func createFileServer(cfg *Config) (http.Handler, error) {
	if _, err := os.Stat(cfg.dir); err != nil {
		return nil, fmt.Errorf("failed to access the directory %s: %w", cfg.dir, err)
	}

	fileServer := http.FileServer(http.Dir(cfg.dir))

	if cfg.logging {
		fileServer = logRequest(fileServer)
	}

	if !cfg.disableGzip || !cfg.disableBrotli {
		fileServer = compressHandler(fileServer, cfg.disableGzip, cfg.disableBrotli)
	}

	if cfg.passwordStdin {
		password, err := getPasswordFromStdin()
		if err != nil {
			return nil, err
		}

		fileServer = basicAuth(fileServer, cfg.user, password)
	}

	log.Printf("Serving directory \"%s\"\n", cfg.dir)
	return fileServer, nil
}

func getPasswordFromStdin() (string, error) {
	var password string
	var err error

	if term.IsTerminal(syscall.Stdin) {
		fmt.Print("Enter Password: ")
		bytePassword, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read password from stdin: %w", err)
		}
		password = string(bytePassword)
		fmt.Println()
	} else {
		reader := bufio.NewReader(os.Stdin)
		password, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read password from stdin: %w", err)
		}
	}

	return password, nil
}

func (app *App) startServer(ctx context.Context, addr string, handler http.Handler) error {
	app.server = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		log.Printf("Serving internally at \"http://localhost%s\"\n", addr)
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP Server Error: %s", err)
		}
	}()

	if app.config.enableNgrok {
		if err := app.createNgrokTunnel(ctx); err != nil {
			return fmt.Errorf("failed to create ngrok tunnel: %w", err)
		}
	}

	return nil
}

func waitForShutdownSignal() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("Shutdown signal received, shutting down gracefully...")
}

func (app *App) shutdownGracefully(ctx context.Context) error {
	ctxShutDown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if app.config.enableNgrok {
		if err := app.ngrokStopHandler(ctxShutDown, nil); err != nil {
			log.Printf("Failed to close ngrok tunnel: %v", err)
		}
	}

	if err := app.server.Shutdown(ctxShutDown); err != nil {
		return err
	}

	app.wg.Wait()

	return nil
}

func (app *App) createNgrokTunnel(ctx context.Context) error {
	tun, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(
			config.WithHTTPServer(app.server),
		),
		ngrok.WithAuthtokenFromEnv(),
		ngrok.WithStopHandler(app.ngrokStopHandler),
		ngrok.WithRestartHandler(app.ngrokRestartHandler),
	)
	if err != nil {
		return fmt.Errorf("failed to create ngrok tunnel: %w", err)
	}

	log.Printf("Serving externally at \"%s\"", tun.URL())
	return nil
}

func (app *App) ngrokStopHandler(_ context.Context, sess ngrok.Session) error {
	if sess == nil {
		return nil
	}

	go func() {
		log.Println("Stopping ngrok tunnel...")
		if err := sess.Close(); err != nil {
			log.Printf("Error closing ngrok tunnel: %s", err)
		}
	}()

	return nil
}

func (app *App) ngrokRestartHandler(ctx context.Context, sess ngrok.Session) error {
	if sess == nil {
		return nil
	}

	go func() {
		log.Println("Restarting ngrok tunnel...")
		if err := sess.Close(); err != nil {
			log.Printf("Error closing ngrok tunnel: %s", err)
		}

		if err := app.createNgrokTunnel(ctx); err != nil {
			log.Printf("Error restarting tunnel: %s", err)
		}
	}()

	return nil
}
