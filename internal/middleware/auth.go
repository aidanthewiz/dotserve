package middleware

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"

	"golang.org/x/term"
)

func AuthHandler(handler http.Handler, user string) http.Handler {
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

func getPasswordFromStdin() (string, error) {
	var password string
	var err error

	if term.IsTerminal(syscall.Stdin) {
		fmt.Print("Specify a password for authentication: ")
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
