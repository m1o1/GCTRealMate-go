package main

import (
	"context"
	"os"
	"os/signal"

	"gctrm/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	executable, _ := os.Executable()
	code := cli.Run(ctx, os.Args[1:], executable, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
