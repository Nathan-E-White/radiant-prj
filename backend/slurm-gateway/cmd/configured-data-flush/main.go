package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("configured-data-flush", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dsn := flags.String("dsn", os.Getenv("CONFIGURED_DATA_FLUSH_POSTGRES_DSN"), "PostgreSQL DSN for the local-demo platform")
	applyPlan := flags.String("apply-plan", "", "exact planId from a reviewed dry-run plan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *dsn == "" {
		return fmt.Errorf("--dsn or CONFIGURED_DATA_FLUSH_POSTGRES_DSN is required")
	}
	repository, err := gateway.NewPostgresConfiguredDataFlushRepository(*dsn)
	if err != nil {
		return err
	}
	defer repository.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return execute(ctx, gateway.NewConfiguredDataFlushService(repository), *applyPlan, output)
}

type configuredDataFlushExecutor interface {
	Plan(context.Context) (gateway.ConfiguredDataFlushPlan, error)
	Apply(context.Context, string) (gateway.ConfiguredDataFlushResult, error)
}

func execute(ctx context.Context, service configuredDataFlushExecutor, applyPlan string, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if applyPlan == "" {
		plan, err := service.Plan(ctx)
		if err != nil {
			return err
		}
		return encoder.Encode(plan)
	}
	result, err := service.Apply(ctx, applyPlan)
	if err != nil {
		return err
	}
	return encoder.Encode(result)
}
