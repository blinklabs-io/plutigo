package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/blinklabs-io/plutigo/replay"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	code := runContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, stdout, stderr)
}

func runContext(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
) int {
	flags := flag.NewFlagSet("plutigo-replay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var corpusPath string
	var pretty bool
	flags.StringVar(&corpusPath, "corpus", "", "path to a replay corpus JSON file")
	flags.BoolVar(&pretty, "pretty", false, "indent the JSON report")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if corpusPath == "" {
		fmt.Fprintln(stderr, "plutigo-replay: -corpus is required")
		return 2
	}

	corpus, err := replay.LoadFile(corpusPath)
	if err != nil {
		fmt.Fprintf(stderr, "plutigo-replay: %v\n", err)
		return 2
	}
	report, err := replay.Run(ctx, corpus)
	if err != nil {
		fmt.Fprintf(stderr, "plutigo-replay: %v\n", err)
		return 2
	}

	encoder := json.NewEncoder(stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(stderr, "plutigo-replay: encode report: %v\n", err)
		return 2
	}
	if report.Summary.Failed > 0 {
		return 1
	}
	return 0
}
