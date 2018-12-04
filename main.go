package main

import (
	"database/sql"
	"fmt"
	"github.com/whiteShtef/clockwork"
	"io/ioutil"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

var (
	client *github.Client
)

func initGithubData() {
	var err error

	// setup the github apps client
	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, AppID, AppInstallationID, PEMFilePath)
	if err != nil {
		logrus.Fatalf("failed to init %v", err)
	}
	client = github.NewClient(&http.Client{Transport: itr})

	// update metadata
	err = UpdateTeamsMetadata()
	if err != nil {
		logrus.Fatalf("failed to update teams metadata: %v", err)
	}

	err = PrepareKanbanMetadata()
	if err != nil {
		logrus.Fatalf("failed to update kanban metadata: %v", err)
	}

	logrus.Printf("initialized successfully.")
}

func githubWebhooks(rw http.ResponseWriter, r *http.Request) {
	var event interface{}

	payload, err := github.ValidatePayload(r, []byte(WebhookSecret))
	if err != nil {
		logrus.Errorf("validate payload failed: %v", err)
	} else {
		event, err = github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			logrus.Errorf("parse webhook failed: %v", err)
		}
	}

	if err != nil {
		body, _ := ioutil.ReadAll(r.Body)
		logrus.Errorf("request body: %v", string(body))

		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
		return
	}

	switch event := event.(type) {
	case *github.IssuesEvent:
		// FIXME(hualet): don't know why GetLogin or GetName both returns empty
		// inTargetOrganization := event.GetRepo().GetOrganization().GetLogin() == OrgName
		// if !inTargetOrganization {
		// 	break
		// }

		issue := event.GetIssue()
		issue.Repository = event.GetRepo()
		action := event.GetAction()

		switch action {
		case "edited":
			processIssueDeadline(issue)
		case "assigned", "unassigned":
			handleIssueAssigneeChanged(issue)
		}

	case *github.ProjectCardEvent:
		card := event.GetProjectCard()
		action := event.GetAction()

		inTargetOrganization := event.GetOrg().GetLogin() == OrgName
		if !inTargetOrganization {
			break
		}

		logrus.Infof("project card %d %v ", card.GetID(), action)

		switch action {
		case "created":
			handleCardCreated(card)

		case "deleted":
			handleCardDeleted(card)

		case "converted":
			handleCardConverted(card)

		case "moved":
			handleCardMoved(card)
		}
	}
}

func handleIssueAssigneeChanged(issue *github.Issue) {
	var assignees []string
	for _, ass := range issue.Assignees {
		assignees = append(assignees, ass.GetLogin())
	}
	if len(issue.Assignees) == 1 && issue.GetState() == "open" {
		assignee := issue.Assignees[0]
		column, err := GetIssueColumn(issue)
		if err != nil {
			return
		}
		logrus.Infof("issue %q in column %v is now only assigned to %v",
			issue.GetTitle(), column.GetName(), assignee.GetLogin())

		if CheckUserMemeberOfQATeam(assignee.GetLogin()) &&
			column.GetName() == DevelopingColumnName {
			logrus.Infof("moving it to %v", TestingColumnName)
			err := MoveToTesting(issue)
			if err != nil {
				logrus.Errorf("failed to move issue %q to %v: %v",
					issue.GetTitle(), TestingColumnName, err)
			}
		} else if CheckUserMemeberOfDevTeam(assignee.GetLogin()) &&
			column.GetName() == TestingColumnName {
			logrus.Infof("moving it to %v", DevelopingColumnName)
			err := MoveToDeveloping(issue)
			if err != nil {
				logrus.Errorf("failed to move issue %q to %v: %v",
					issue.GetTitle(), DevelopingColumnName, err)
			}
		}
	}
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "kanbanmgr.db")
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS issue_deadline (
		id INTEGER PRIMARY KEY NOT NULL,
		date DATE NOT NULL,
		url TEXT NOT NULL,
		directive TEXT NOT NULL
		)`)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var err error
	err = initDB()
	if err != nil {
		logrus.Fatal("failed to init db:", err)
	}
	initGithubData()
	checkIssueDeadlineForAllCards()

	scheduler := clockwork.NewScheduler()
	scheduler.Schedule().Every().Day().At("1:00").Do(checkIssueDeadlineForAllCards)
	go scheduler.Run()

	http.HandleFunc("/", githubWebhooks)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", ServePort), nil))
}
