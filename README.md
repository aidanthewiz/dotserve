# dotserve :file_folder:

This project is a Go application that serves a directory over HTTP. It provides options for setting the directory to
serve, the port to listen on, and basic authentication credentials. It also supports exposing the server to the internet
using [ngrok](https://ngrok.com/).

## :zap: Installation

First, you need to have Go installed on your machine. If you haven't installed it yet, you can download it from [here](https://go.dev/dl/).
Alternatively, you can install Go with your preferred package manager.

Once Go is installed, you can install dotserve by running the following command:
```bash
go install github.com/aidanthewiz/dotserve/cmd/dotserve@latest
```

## :wrench: Configuration

You can configure the application by passing command line flags. The available flags are:

- `-dir`: Set the directory to serve (default is the current directory)
- `-no-brotli`: Disable brotli compression (default is `false`)
- `-no-gzip`: Disable gzip compression (default is `false`)
- `-no-logging`: Disable request logging (default is `false`)
- `-ngrok`: Expose the server to the internet using ngrok (default is `false`)
- `-password-stdin`: Read the password for basic authentication from stdin (default is `false`)
- `-port`: Set the port to listen on, use 0 to choose a random port (default is `0`)
- `-user`: Set the username for basic authentication (default is `admin`)

## :computer: Usage

To start the server, simply run the application with the desired flags. For example:

```shell
dotserve -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```

or

```shell
echo "password" | dotserve -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```

This will serve the `./public` directory on port `3000`, with the username `frodo` for basic authentication. The
password will be read from stdin. The server will be exposed to the internet using ngrok.

## :closed_lock_with_key: Basic Authentication

You can set the username for basic authentication using the `-user` flag. By default, the username is `admin`.

If the `-password-stdin` flag is set, the application will prompt for a password on startup. This password will be used
for basic authentication along with the username set by the `-user` flag.

## :package: Compression

This application uses brotli and gzip compression for the HTTP responses by default.
The Accept-Encoding request header is used to determine which type of compression should be applied.

You can disable brotli and gzip compression individually by setting the -no-brotli and -no-gzip flags respectively when running the application.

If the request contains the conditional headers If-None-Match or If-Modified-Since, compression is bypassed.

Here's how to run the application with brotli compression disabled:

```shell
dotserve -dir ./public -port 3000 -user frodo -password-stdin -no-brotli
````

This will serve the ./public directory on port 3000, with the username frodo for basic authentication, with brotli compression disabled.
Therefore, the application will only use gzip compression.

## :globe_with_meridians: Ngrok Integration

If the `-ngrok` flag is set, the server will be exposed to the internet using ngrok. To use ngrok, you must specify your
auth token using the `NGROK_AUTHTOKEN` environment variable.

For example:

```shell
NGROK_AUTHTOKEN=your_ngrok_auth_token dotserve -dir ./public -port 3000 -user frodo -password-stdin -ngrok
```
