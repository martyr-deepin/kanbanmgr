package main

import (
	"context"
	"sync"

	"github.com/google/go-github/github"
)

type team struct {
	*github.Team
	Members []*github.User
}

var (
	metaTeams []*team
	teamsLock sync.Mutex
)

func updateTeams() (err error) {
	ctx := context.Background()
	opts := &github.ListOptions{}

	// clear the teams
	metaTeams = []*team{}

	for {
		teams, resp, err := client.Teams.ListTeams(ctx, OrgName, opts)
		if err != nil {
			return err
		}

		for _, t := range teams {
			metaTeams = append(metaTeams, &team{t, []*github.User{}})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func updateTeamMembers(team *team) (err error) {
	ctx := context.Background()
	opts := &github.TeamListTeamMembersOptions{}

	for {
		members, resp, err := client.Teams.ListTeamMembers(ctx, team.GetID(), opts)
		if err != nil {
			return err
		}

		team.Members = append(team.Members, members...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

// UpdateTeamsMetadata updates the metaTeams of all teams.
func UpdateTeamsMetadata() error {
	teamsLock.Lock()
	defer teamsLock.Unlock()

	err := updateTeams()
	if err != nil {
		return err
	}
	for _, t := range metaTeams {
		err := updateTeamMembers(t)
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckUserMemeberOfQATeam checks if an user belongs to the QA team.
func CheckUserMemeberOfQATeam(loginName string) bool {
	teamsLock.Lock()
	defer teamsLock.Unlock()

	for _, t := range metaTeams {
		if t.GetName() == QATeamName {
			for _, m := range t.Members {
				if m.GetLogin() == loginName {
					return true
				}
			}
		}
	}
	return false
}

// CheckUserMemeberOfDevTeam checks if an user belongs to the dev team.
func CheckUserMemeberOfDevTeam(loginName string) bool {
	teamsLock.Lock()
	defer teamsLock.Unlock()

	for _, t := range metaTeams {
		if t.GetName() == DevTeamName {
			for _, m := range t.Members {
				if m.GetLogin() == loginName {
					return true
				}
			}
		}
	}
	return false
}
