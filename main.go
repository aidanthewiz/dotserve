package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
	"golang.org/x/term"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	dir           string
	port          string
	user          string
	passwordStdin bool
	enableNgrok   bool
}

type App struct {
	server *http.Server
	wg     sync.WaitGroup
	config *Config
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
	flag.StringVar(&cfg.port, "port", "8080", "port to listen on")
	flag.StringVar(&cfg.user, "user", "admin", "username for basic auth")
	flag.BoolVar(&cfg.passwordStdin, "password-stdin", false, "read password from stdin")
	flag.BoolVar(&cfg.enableNgrok, "ngrok", false, "expose the server to the internet using ngrok")
	flag.Parse()
}

func createFileServer(cfg *Config) (http.Handler, error) {
	if _, err := os.Stat(cfg.dir); err != nil {
		return nil, fmt.Errorf("failed to access the directory %s: %w", cfg.dir, err)
	}

	fileServer := http.FileServer(http.Dir(cfg.dir))

	if cfg.passwordStdin {
		var password string
		var err error

		if term.IsTerminal(syscall.Stdin) {
			fmt.Print("Enter Password: ")
			bytePassword, err := term.ReadPassword(syscall.Stdin)
			if err != nil {
				return nil, fmt.Errorf("failed to read password from stdin: %w", err)
			}
			password = string(bytePassword)
			fmt.Println()
		} else {
			reader := bufio.NewReader(os.Stdin)
			password, err = reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("failed to read password from stdin: %w", err)
			}
		}

		fileServer = basicAuth(fileServer, cfg.user, password)
	}

	log.Printf("Serving directory \"%s\"\n", cfg.dir)
	return fileServer, nil
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

func basicAuth(handler http.Handler, user, pass string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()

		userLengthMatch := subtle.ConstantTimeEq(int32(len(u)), int32(len(user)))
		passLengthMatch := subtle.ConstantTimeEq(int32(len(p)), int32(len(pass)))
		userMatch := subtle.ConstantTimeCompare([]byte(u), []byte(user))
		passMatch := subtle.ConstantTimeCompare([]byte(p), []byte(pass))
		isEqual := userLengthMatch & passLengthMatch & userMatch & passMatch

		if !ok || isEqual != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="."`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			log.Printf("Unauthorized access attempt from %s", r.RemoteAddr)
			return
		}

		log.Printf("Successful connection from %s", r.RemoteAddr)
		handler.ServeHTTP(w, r)
	})
}
