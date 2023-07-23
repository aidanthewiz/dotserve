package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/aidanthewiz/dotserve/internal/middleware"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

type App struct {
	config *Config
	tunnel ngrok.Tunnel
	server *http.Server
	wg     sync.WaitGroup
}

func (app *App) initConfig() error {
	cfg := &Config{}

	flag.StringVar(&cfg.dir, "dir", ".", "Set the directory to serve")
	flag.BoolVar(&cfg.disableBrotli, "no-brotli", false, "Disable brotli compression")
	flag.BoolVar(&cfg.disableGzip, "no-gzip", false, "Disable gzip compression")
	flag.BoolVar(&cfg.disableLogging, "no-logging", false, "Disable request logging")
	flag.BoolVar(&cfg.enableNgrok, "ngrok", false, "Expose the server to the internet using ngrok")
	flag.BoolVar(&cfg.passwordStdin, "password-stdin", false, "Read the password for basic authentication from stdin")
	flag.StringVar(&cfg.port, "port", "0", "Set the port to listen on, use 0 to choose a random port")
	flag.StringVar(&cfg.user, "user", "admin", "Set the username for basic authentication")

	flag.Parse()

	err := cfg.validate()
	if err != nil {
		return err
	}

	app.config = cfg
	return nil
}

func (app *App) createFileServer() (http.Handler, error) {
	fileServer := http.FileServer(http.Dir(app.config.dir))

	if !app.config.disableLogging {
		fileServer = middleware.LogRequest(fileServer)
	}

	if !app.config.disableGzip || !app.config.disableBrotli {
		fileServer = middleware.CompressHandler(fileServer, app.config.disableGzip, app.config.disableBrotli)
	}

	if app.config.passwordStdin {
		fileServer = middleware.BasicAuth(fileServer, app.config.user)
	}

	log.Printf("Serving directory \"%s\"\n", app.config.dir)
	return fileServer, nil
}

func (app *App) startServer(ctx context.Context) error {
	addr := net.JoinHostPort("", app.config.port)
	handler, err := app.createFileServer()
	if err != nil {
		return fmt.Errorf("failed to create file server: %w", err)
	}

	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	app.server = &http.Server{
		Handler: handler,
	}

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		log.Println("Serving internally at http://" + listener.Addr().String())
		if err := app.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Println("HTTP Server Error:", err)
		}
	}()

	if app.config.enableNgrok {
		if err := app.createNgrokTunnel(ctx); err != nil {
			return fmt.Errorf("failed to create ngrok tunnel: %w", err)
		}
	}

	return nil
}

func (app *App) shutdownGracefully(ctx context.Context) {
	ctxShutDown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if app.tunnel != nil {
		if err := app.tunnel.CloseWithContext(ctxShutDown); err != nil {
			log.Println("Failed to gracefully close ngrok tunnel:", err)
		}
	}

	if err := app.server.Shutdown(ctxShutDown); err != nil {
		log.Println("Failed to gracefully shutdown HTTP server:", err)
	}

	app.wg.Wait()
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

	app.tunnel = tun

	log.Println("Serving externally at", tun.URL())
	return nil
}

func (app *App) ngrokStopHandler(_ context.Context, sess ngrok.Session) error {
	if sess != nil {
		go func() {
			log.Println("Stopping ngrok tunnel...")
			if err := sess.Close(); err != nil {
				log.Println("Error closing ngrok tunnel:", err)
			}
		}()
	}
	return nil
}

func (app *App) ngrokRestartHandler(ctx context.Context, sess ngrok.Session) error {
	if sess != nil {
		go func() {
			log.Println("Restarting ngrok tunnel...")
			if err := sess.Close(); err != nil {
				log.Println("Error closing ngrok tunnel:", err)
			}
			if err := app.createNgrokTunnel(ctx); err != nil {
				log.Println("Error restarting tunnel:", err)
			}
		}()
	}
	return nil
}
