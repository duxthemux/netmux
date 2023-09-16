package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"go.digitalcircle.com.br/dc/netmux/foundation/buildinfo"
)

func main() {
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, request *http.Request) {
		slog.Info(fmt.Sprintf("Got request: %s %s\n", request.Method, request.URL.String()))
		responseWriter.Header().Set("content-type", "text/plain")
		buf := &bytes.Buffer{}
		buf.WriteString("SampleService:\n")
		buf.WriteString(buildinfo.StringOneLine(""))

		buf.WriteString("Env:\n")

		for _, envEntry := range os.Environ() {
			buf.WriteString(fmt.Sprintf("*** %s\n", envEntry))
		}

		buf.WriteString(fmt.Sprintf("Got request: %s %s\n", request.Method, request.URL.String()))

		defer func() {
			_ = request.Body.Close()
		}()

		_, _ = io.Copy(buf, request.Body)
		buf.WriteString("\n==EOR==\n")
		_, _ = responseWriter.Write(buf.Bytes())
	})

	slog.Info("Sample Reverse Service")
	slog.Info(buildinfo.StringOneLine(""))

	err := http.ListenAndServe(":8081", nil) //nolint:gosec
	if err != nil {
		panic(err)
	}
}
