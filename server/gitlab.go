package server

import (
	"errors"
	"fmt"
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

func GetUserProjects(client *gitlab.Client, username string) ([]string, error) {

	start := time.Now()
	var projectPaths []string
	var projects []*gitlab.Project
	var retryAttempts uint = 3

	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Simple:         gitlab.Ptr(true),
		Membership:     gitlab.Ptr(true),
		MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
	}

	var resp *gitlab.Response
	var err error
	err = retry.Do(func() error {
		projects, resp, err = client.Projects.ListProjects(opt, gitlab.WithHeader(SudoHeader, username))
		if err != nil {
			return err
		}
		return nil
	}, retry.Attempts(retryAttempts))

	if err != nil {
		return nil, err
	}

	fmt.Println("Total pages:", resp.TotalPages)

	for _, project := range projects {
		projectPaths = append(projectPaths, project.PathWithNamespace)
	}

	if resp.TotalPages == 1 {
		return projectPaths, nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, resp.TotalPages)

	for page := resp.NextPage; page <= resp.TotalPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			userProjects := []*gitlab.Project{}
			opt.ListOptions.Page = page
			err := retry.Do(func() error {
				userProjects, _, err = client.Projects.ListProjects(opt, gitlab.WithHeader(SudoHeader, username))
				if err != nil {
					errChan <- errors.New("error getting page")
					return err
				}
				return nil
			}, retry.Attempts(retryAttempts))
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
		return nil, <-errChan
	}
	fmt.Println("Total projects:", len(projects))
	fmt.Printf("took %v\n", time.Since(start))

	return projectPaths, nil
}
