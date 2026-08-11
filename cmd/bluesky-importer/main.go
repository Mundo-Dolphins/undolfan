package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Mundo-Dolphins/undolfan/internal/bluesky"
	"github.com/Mundo-Dolphins/undolfan/internal/importer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return fmt.Errorf("usage: bluesky-importer sync [--actor handle] [--full] [--dry-run]")
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	actor := fs.String("actor", "undolfan.mundodolphins.es", "Bluesky handle or DID to import")
	full := fs.Bool("full", false, "run full backfill")
	dryRun := fs.Bool("dry-run", false, "fetch and render decisions without writing files")
	repo := fs.String("repo", ".", "repository root")
	minThreadPosts := fs.Int("minimum-thread-posts", 2, "minimum own posts required for article content")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := bluesky.New()
	imp := importer.New(client)
	log.Printf("Starting Bluesky sync actor=%s full=%v dry_run=%v", *actor, *full, *dryRun)
	res, err := imp.Sync(ctx, importer.Config{
		RepoRoot:           *repo,
		Actor:              *actor,
		Full:               *full,
		DryRun:             *dryRun,
		MinimumThreadPosts: *minThreadPosts,
	})
	if err != nil {
		return err
	}
	log.Printf("Posts fetched: %d", res.PostsFetched)
	log.Printf("Candidate roots: %d", res.CandidateRoots)
	log.Printf("Sync completed: %d new, %d updated, %d unchanged", res.New, res.Updated, res.Unchanged)
	return nil
}
