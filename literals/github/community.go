package github

import (
	"github.com/sebach1/git-crud/git"
)

const baseURL = "https://api.github.com"

// OpenSource is the open source code community
var OpenSource = &git.Community{
	&git.Team{
		AssignedSchema: "github",
		Members: []*git.Member{
			&git.Member{AssignedTable: "repositories", Collab: new(repositories)},
			&git.Member{AssignedTable: "organizations", Collab: new(organizations)},
		},
	},
}
