package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/dexidp/dex/server"
	"github.com/xanzy/go-gitlab"
)

func main() {
	gitlabcli, err := gitlab.NewClient("", gitlab.WithBaseURL(""))
	if err != nil {
		log.Fatal(err)
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	projects, err := server.GetUserProjects(gitlabcli, logger, "")

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(projects)
}
