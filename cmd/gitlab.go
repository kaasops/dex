package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/dexidp/dex/server"
	"github.com/xanzy/go-gitlab"
)

func main() {
	gitlabcli, err := gitlab.NewClient("", gitlab.WithBaseURL(""))
	if err != nil {
		log.Fatal(err)
	}

	logger := slog.Default()
	projects, err := server.GetUserProjects(gitlabcli, logger, "")

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(projects)
}
