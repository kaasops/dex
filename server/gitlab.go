package server

import (
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/xanzy/go-gitlab"
)

// temporary solution until this issue is resolved
// hardcoded appending gitlab projects to groups claim
// https://github.com/kubernetes/kubernetes/issues/128438

const (
	SudoHeader       = "Sudo"
	errNotFound      = "404 Not Found"
	adminGroupPrefix = "admin:"
)

var notRetryableErrors = []string{
	errNotFound,
}

var (
	retryAttempts uint = 3
	perPage            = 50
)

func GetUserProjects(client *gitlab.Client, logger *slog.Logger, username string, privilegedGroups []int) ([]string, error) {
	var privileged bool
	var err error
	if len(privilegedGroups) > 0 {
		privileged, err = isPrivelegedUser(client, logger, username, privilegedGroups)
		if err != nil {
			return nil, err
		}
	}
	return getUserProjects(client, logger, username, privileged)
}

// Get user projects with minimum developer permissions from gitlab
func getUserProjects(client *gitlab.Client, logger *slog.Logger, username string, privileged bool) ([]string, error) {

	start := time.Now()
	var projectPaths []string

	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
		},
		Simple:         gitlab.Ptr(true),
		Membership:     gitlab.Ptr(true),
		MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
	}

	projects, resp, err := ListProjectsWithRetry(client, logger, opt, gitlab.WithHeader(SudoHeader, username))

	if err != nil {
		return nil, err
	}

	logger.Info("multipage gitlab request", "user", username, "pages", strconv.Itoa((resp.TotalPages)))

	for _, project := range projects {
		path := project.PathWithNamespace
		if privileged {
			path = adminPrefix(project.PathWithNamespace)
		}
		projectPaths = append(projectPaths, strings.ToLower(path))
	}

	if resp.TotalPages < 2 {
		logger.Info("request completed", "user", username, "total projects", strconv.Itoa(len(projectPaths)), "took", time.Since(start).String())
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
			opt := &gitlab.ListProjectsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: perPage,
				},
				Simple:         gitlab.Ptr(true),
				Membership:     gitlab.Ptr(true),
				MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
			}
			userProjects, _, err := ListProjectsWithRetry(client, logger, opt, gitlab.WithHeader(SudoHeader, username))
			if err != nil {
				errChan <- err
				return
			}
			for _, project := range userProjects {
				mu.Lock()
				path := project.PathWithNamespace
				if privileged {
					path = adminPrefix(project.PathWithNamespace)
				}
				projectPaths = append(projectPaths, strings.ToLower(path))
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

	logger.Info("request completed", "user", username, "total projects", strconv.Itoa(len(projectPaths)), "took", time.Since(start).String())

	return projectPaths, nil
}

func isPrivelegedUser(client *gitlab.Client, logger *slog.Logger, username string, privilegedGroups []int) (bool, error) {
	options := &gitlab.ListGroupMembersOptions{
		ListOptions: gitlab.ListOptions{
			// TODO: pagination reuqest is not implemented
			PerPage: 100,
		},
	}
	for _, group := range privilegedGroups {
		members, _, err := ListGroupMembersWithRetry(client, logger, group, options)
		if err != nil {
			logger.Error("error getting group members", "group", group, "error", err.Error())
		}
		for _, member := range members {
			if member.Username == username {
				return true, err
			}
		}
	}
	return false, nil
}

func adminPrefix(path string) string {
	return adminGroupPrefix + path
}

func ListGroupMembersWithRetry(client *gitlab.Client, logger *slog.Logger, groupID int, opt *gitlab.ListGroupMembersOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.GroupMember, *gitlab.Response, error) {
	var members []*gitlab.GroupMember
	var responce *gitlab.Response
	err := retry.Do(func() error {
		var err error
		members, responce, err = client.Groups.ListGroupMembers(groupID, opt, options...)
		if err != nil {
			if !contains(notRetryableErrors, err.Error()) {
				logger.Error("error during gitlab request, retrying", "error", err.Error())
				return err
			}
		}
		return nil
	}, retry.Attempts(retryAttempts))
	return members, responce, err
}

func ListProjectsWithRetry(client *gitlab.Client, logger *slog.Logger, opt *gitlab.ListProjectsOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
	var projects []*gitlab.Project
	var responce *gitlab.Response
	err := retry.Do(func() error {
		var err error
		projects, responce, err = client.Projects.ListProjects(opt, options...)
		if err != nil {
			if !contains(notRetryableErrors, err.Error()) {
				logger.Error("error during gitlab request, retrying", "error", err.Error())
				return err
			}
		}
		return nil
	}, retry.Attempts(retryAttempts))
	return projects, responce, err
}
