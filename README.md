# dotserve :file_folder:

This project is a Go application that serves a directory over HTTP. It provides options for setting the directory to
serve, the port to listen on, and basic authentication credentials. It also supports exposing the server to the internet
using [ngrok](https://ngrok.com/).

## :wrench: Configuration

You can configure the application by passing command line flags. The available flags are:

- `-dir`: Set the directory to serve (default is the current directory)
- `-port`: Set the port to listen on (default is `8080`)
- `-user`: Set the username for basic authentication (default is `admin`)
- `-password-stdin`: Read the password for basic authentication from stdin (default is `false`)
- `-ngrok`: Expose the server to the internet using ngrok (default is `false`)

## :computer: Usage

To start the server, simply run the application with the desired flags. For example:

```shell
go run main.go -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```

or

```shell
echo "password" | go run main.go -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```

This will serve the `./public` directory on port `3000`, with the username `frodo` for basic authentication. The
password will be read from stdin. The server will be exposed to the internet using ngrok.

## :closed_lock_with_key: Basic Authentication

You can set the username for basic authentication using the `-user` flag. By default, the username is `admin`.

If the `-password-stdin` flag is set, the application will prompt for a password on startup. This password will be used
for basic authentication along with the username set by the `-user` flag.

## :globe_with_meridians: Ngrok Integration

If the `-ngrok` flag is set, the server will be exposed to the internet using ngrok. To use ngrok, you must specify your
auth token using the `NGROK_AUTHTOKEN` environment variable.

For example:

```shell
NGROK_AUTHTOKEN=your_ngrok_auth_token go run main.go -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```
