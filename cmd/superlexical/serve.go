package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"superlexical/api"
	"superlexical/store"
)

func runServe(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:4100", "address to listen on")

	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: superlexical serve [--addr ADDRESS] ROOT")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}

		return 2
	}

	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}

	root := flags.Arg(0)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "Cannot serve %q: select an existing directory.\n", root)
		return 2
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot listen on %s: %s\n", *addr, err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workspace, err := store.Open(root)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot open the workspace %q: %s\n", root, err)
		return 2
	}

	handler, _ := api.New(workspace)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	failed := make(chan error, 1)
	go func() {
		failed <- server.Serve(listener)
	}()

	fmt.Fprintf(stdout, "Serving %s on http://%s\n", root, listener.Addr())

	select {
	case err := <-failed:
		fmt.Fprintf(stderr, "Server stopped: %s\n", err)
		return 2
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "Cannot stop the server: %s\n", err)
		return 2
	}

	return 0
}

func runOpenAPI(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "Usage: superlexical openapi")
		return 2
	}

	_, spec := api.New(nil)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(spec.OpenAPI()); err != nil {
		fmt.Fprintf(stderr, "Cannot write the OpenAPI specification: %s\n", err)
		return 2
	}

	return 0
}
