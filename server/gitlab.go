package server

import (
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/xanzy/go-gitlab"
)

// temporary solution until this issue is resolved
// hardcoded appending gitlab projects to groups claim
// https://github.com/kubernetes/kubernetes/issues/128438

const (
	SudoHeader  = "Sudo"
	errNotFound = "404 Not Found"
)

var notRetryableErrors = []string{
	errNotFound,
}

var retryAttempts uint = 3

// Get user projects with minimum developer permissions from gitlab
func GetUserProjects(client *gitlab.Client, logger *slog.Logger, username string) ([]string, error) {

	start := time.Now()
	var projectPaths []string

	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Simple:         gitlab.Ptr(true),
		Membership:     gitlab.Ptr(true),
		MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
	}

	projects, resp, err := ListProjectsWithRetry(client, opt, gitlab.WithHeader(SudoHeader, username))

	if err != nil {
		return nil, err
	}

	logger.Info("multipage gitlab request", "user", username, "pages", strconv.Itoa((resp.TotalPages)))

	for _, project := range projects {
		projectPaths = append(projectPaths, project.PathWithNamespace)
	}

	if resp.TotalPages < 2 {
		logger.Info("request completed", "user", username, "total projects", strconv.Itoa(len(projectPaths)), "took", time.Since(start))
		return projectPaths, nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, resp.TotalPages)

	// request all pages concurrently
	for page := resp.NextPage; page <= resp.TotalPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			opt.ListOptions.Page = page
			userProjects, _, err := client.Projects.ListProjects(opt, gitlab.WithHeader(SudoHeader, username))
			if err != nil {
				errChan <- err
				return
			}
			for _, project := range userProjects {
				mu.Lock()
				projectPaths = append(projectPaths, project.PathWithNamespace)
				mu.Unlock()
			}
		}(page)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		logger.Error("error during multipage gitlab request", "user", username, "error", <-errChan)
		return nil, <-errChan
	}

	logger.Info("request completed", "user", username, "total projects", strconv.Itoa(len(projectPaths)), "took", time.Since(start))

	return projectPaths, nil
}

func ListProjectsWithRetry(client *gitlab.Client, opt *gitlab.ListProjectsOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
	var projects []*gitlab.Project
	var responce *gitlab.Response
	err := retry.Do(func() error {
		var err error
		projects, responce, err = client.Projects.ListProjects(opt, options...)
		if err != nil {
			if !contains(notRetryableErrors, err.Error()) {
				return err
			}
		}
		return nil
	}, retry.Attempts(retryAttempts))
	return projects, responce, err
}
