package server

import "github.com/xanzy/go-gitlab"

// temporary solution until this issue is resolved
// hardcoded appending gitlab projects to groups claim
// https://github.com/kubernetes/kubernetes/issues/128438

const (
	SudoHeader = "Sudo"
)

func GetUserProjects(client *gitlab.Client, username string) ([]string, error) {
	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		Simple:         gitlab.Ptr(true),
		Membership:     gitlab.Ptr(true),
		MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
	}

	var projects []*gitlab.Project
	var projectNames []string

	for {
		userProjects, resp, err := client.Projects.ListProjects(opt, gitlab.WithHeader(SudoHeader, username))
		if err != nil {
			return nil, err
		}

		projects = append(projects, userProjects...)

		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opt.Page = resp.NextPage
	}

	for _, project := range projects {
		projectNames = append(projectNames, project.PathWithNamespace)
	}

	return projectNames, nil
}
