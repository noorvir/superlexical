package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"superlexical/core"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: superlexical validate [--schema FILE] [--watch] ROOT")
		return 0
	}

	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(stderr, "Usage: superlexical validate [--schema FILE] [--watch] ROOT")
		return 2
	}

	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	watchChanges := flags.Bool("watch", false, "revalidate the full directory after file changes")
	schemaPath := flags.String("schema", "", "schema for one folder; otherwise read workspace extensions")

	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: superlexical validate [--schema FILE] [--watch] ROOT")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}

		return 2
	}

	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}

	if *watchChanges {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return watch(ctx, flags.Arg(0), *schemaPath, stdout, stderr)
	}

	return validate(flags.Arg(0), *schemaPath, stdout, stderr)
}

func validate(root, schemaPath string, stdout, stderr io.Writer) int {
	var diagnostics []core.Diagnostic
	var err error

	if schemaPath == "" {
		diagnostics, err = core.CompileWorkspace(root)
	} else {
		var schema []byte
		schema, err = os.ReadFile(schemaPath)
		if err != nil {
			fmt.Fprintf(stderr, "Cannot read schema: %s\n", err)
			return 2
		}

		diagnostics, err = core.Compile(root, schema)
	}

	if err != nil {
		fmt.Fprintf(stderr, "Cannot validate %q: %s\n", root, err)
		return 2
	}

	if len(diagnostics) == 0 {
		fmt.Fprintf(stdout, "Valid: %s\n", root)
		return 0
	}

	fmt.Fprintf(stderr, "Errors in %s:\n\n", root)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(stderr, diagnostic)
	}

	count := fmt.Sprintf("%d errors", len(diagnostics))
	if len(diagnostics) == 1 {
		count = "1 error"
	}

	fmt.Fprintf(stderr, "\nValidation failed: %s. Fix them and run validation again.\n", count)
	return 1
}
