package middleware

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/term"
)

type compressResponseWriter struct {
	http.ResponseWriter
	cw io.WriteCloser
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	return w.cw.Write(b)
}

func CompressHandler(h http.Handler, disableGzip, disableBrotli bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass compression if the request contains the conditional headers "If-None-Match" or "If-Modified-Since".
		// These headers may lead to a 304 Not Modified response,
		// which should not contain a body as per HTTP specification.
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
			h.ServeHTTP(w, r)
			return
		}

		var cw io.WriteCloser
		var crw *compressResponseWriter
		encoding := r.Header.Get("Accept-Encoding")

		if !disableBrotli && strings.Contains(encoding, "br") {
			w.Header().Set("Content-Encoding", "br")
			cw = brotli.NewWriterLevel(w, brotli.DefaultCompression)
		} else if !disableGzip && strings.Contains(encoding, "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			cw = gzip.NewWriter(w)
		} else {
			h.ServeHTTP(w, r)
			return
		}

		crw = &compressResponseWriter{
			ResponseWriter: w,
			cw:             cw,
		}

		h.ServeHTTP(crw, r)
		err := cw.Close()
		if err != nil {
			log.Printf("Error closing writer: %s", err)
		}
	})
}

func BasicAuth(handler http.Handler, user string) http.Handler {
	password, err := getPasswordFromStdin()
	if err != nil {
		log.Fatal("Failed to read password from stdin: ", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()

		userLengthMatch := subtle.ConstantTimeEq(int32(len(u)), int32(len(user)))
		passLengthMatch := subtle.ConstantTimeEq(int32(len(p)), int32(len(password)))
		userMatch := subtle.ConstantTimeCompare([]byte(u), []byte(user))
		passMatch := subtle.ConstantTimeCompare([]byte(p), []byte(password))
		isEqual := userLengthMatch & passLengthMatch & userMatch & passMatch

		if !ok || isEqual != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="."`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			log.Printf("Unauthorized access attempt from %s", r.RemoteAddr)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func LogRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Remote address: %s, Method: %s, URL: %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
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
