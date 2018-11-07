package main

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
)

var (
	client *github.Client
)

func init() {
	var err error

	// setup the github apps client
	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, 20288, AppInstallationID, PEMFilePath)
	if err != nil {
		logrus.Fatalf("failed to init %v", err)
	}
	client = github.NewClient(&http.Client{Transport: itr})

	// update metadata
	err = UpdateTeamsMetadata()
	if err != nil {
		logrus.Fatalf("failed to update teams metadata: %v", err)
	}

	err = UpdateKanbanMetadata()
	if err != nil {
		logrus.Fatalf("failed to update kanban metadata: %v", err)
	}

	logrus.Printf("initialized successfully.")
}

func githubWebhooks(rw http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(WebhookSecret))
	if err != nil {
		logrus.Infof("validate payload failed: %v", err)
		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		logrus.Infof("parse webhook failed: %v", err)
		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
		return
	}
	switch event := event.(type) {
	case *github.IssuesEvent:
		inTargetOrganization := event.GetRepo().GetOrganization().GetLogin() == OrgName
		logrus.Infof("%v %v", event.GetRepo().GetOrganization().GetName(), event.GetRepo().GetOrganization().GetLogin())
		if !inTargetOrganization {
			break
		}

		issue := event.GetIssue()
		action := event.GetAction()
		inTargetActions := action == "assigned" || action == "unassigned"
		if !inTargetActions {
			break
		}

		assignees := []string{}
		for _, ass := range issue.Assignees {
			assignees = append(assignees, ass.GetLogin())
		}
		logrus.Infof("issue \"%v\" has new assignees: %v", issue.GetTitle(), assignees)
		if len(issue.Assignees) == 1 && *issue.State == "open" {
			assignee := issue.Assignees[0]
			logrus.Infof("the issue \"%v\" is now only assigned to %v", issue.GetTitle(), assignee.GetLogin())
			column, err := GetIssueColumn(issue)
			if err != nil {
				logrus.Infof("cant't get the column the issue \"%v\" belongs to , maybe not in the project %v ?", issue.GetTitle(), TargetProject)
				break
			}
			logrus.Infof("the issue \"%v\" is now in column %v", issue.GetTitle(), column.GetName())
			if CheckUserMemeberOfQATeam(assignee.GetLogin()) && column.GetName() == DevelopingColumnName {
				logrus.Infof("moving it to %v", TestingColumnName)
				err := MoveToTesting(issue)
				if err != nil {
					logrus.Errorf("failed to move issue \"%v\" to %v: %v", issue.GetTitle(), TestingColumnName, err)
				}
			} else if CheckUserMemeberOfDevTeam(assignee.GetLogin()) && column.GetName() == TestingColumnName {
				logrus.Infof("moving it to %v", DevelopingColumnName)
				err := MoveToDeveloping(issue)
				if err != nil {
					logrus.Errorf("failed to move issue \"%v\" to %v: %v", issue.GetTitle(), DevelopingColumnName, err)
				}
			}
		}
	case *github.ProjectCardEvent:
		card := event.GetProjectCard()
		action := event.GetAction()

		inTargetOrganization := event.GetOrg().GetLogin() == OrgName
		if !inTargetOrganization {
			break
		}

		logrus.Infof("project card \"%v\" %v ", card.GetURL(), action)
		if action == "created" {
			err := AppendCard(card)
			if err != nil {
				logrus.Errorf("failed to append new card \"v\": v", card.GetURL(), err)
			}
		} else if action == "deleted" {
			err := RemoveCard(card)
			if err != nil {
				logrus.Errorf("failed to remove card \"v\": v", card.GetURL(), err)
			}
		} else if action == "converted" {
			err := ConvertCard(card)
			if err != nil {
				logrus.Errorf("failed to update card \"v\": v", card.GetURL(), err)
			}
		}
	}

}

func main() {
	http.HandleFunc("/", githubWebhooks)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", ServePort), nil))
}
