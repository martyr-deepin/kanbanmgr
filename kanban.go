package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/cosiner/gohper/regexp"
	"strconv"
	"sync"

	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

var (
	metaCards   []*github.ProjectCard
	metaColumns []*github.ProjectColumn
	cardsLock   sync.Mutex

	errNotInTargetCol = errors.New("not in the target columns")
)

func getColumnCards(column *github.ProjectColumn) ([]*github.ProjectCard, error) {
	var ret []*github.ProjectCard

	ctx := context.Background()
	opts := &github.ProjectCardListOptions{}

	for {
		cds, resp, err := client.Projects.ListProjectCards(ctx, column.GetID(), opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, cds...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func getProjectColumns(project *github.Project) ([]*github.ProjectColumn, error) {
	var ret []*github.ProjectColumn

	ctx := context.Background()
	opts := &github.ListOptions{}

	for {
		colns, resp, err := client.Projects.ListProjectColumns(ctx, project.GetID(), opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, colns...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func getProjects() ([]*github.Project, error) {
	var ret []*github.Project

	ctx := context.Background()
	opts := &github.ProjectListOptions{}

	for {
		projs, resp, err := client.Organizations.ListProjects(ctx, OrgName, opts)
		if err != nil {
			return nil, err
		}

		ret = append(ret, projs...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return ret, nil
}

func findCard(card *github.ProjectCard) int {
	for i, cd := range metaCards {
		if cd.GetID() == card.GetID() {
			return i
		}
	}
	return -1
}

func isCardInTargetColumns(card *github.ProjectCard) bool {
	col, err := getCardColumn(card)
	if err != nil {
		return false
	}
	return isTargetColumn(col)
}

func isTargetColumn(column *github.ProjectColumn) bool {
	columnName := column.GetName()
	return columnName == DevelopingColumnName || columnName == TestingColumnName
}

func handleCardCreated(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	in := isCardInTargetColumns(card)
	if !in {
		return errNotInTargetCol
	}

	metaCards = append(metaCards, card)

	go processCardIssueDeadline(card)
	return nil
}

func handleCardDeleted(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	in := isCardInTargetColumns(card)
	if !in {
		return errNotInTargetCol
	}

	index := findCard(card)
	if index == -1 {
		return errNotInTargetCol
	}

	metaCards[index] = metaCards[len(metaCards)-1]
	metaCards[len(metaCards)-1] = nil
	metaCards = metaCards[:len(metaCards)-1]

	go deleteCardIssueDeadline(card)
	return nil
}

func handleCardConverted(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	in := isCardInTargetColumns(card)
	if !in {
		return errNotInTargetCol
	}

	index := findCard(card)
	if index == -1 {
		return errNotInTargetCol
	}

	metaCards[index] = card

	go processCardIssueDeadline(card)
	return nil
}

func handleCardMoved(card *github.ProjectCard) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	in := isCardInTargetColumns(card)
	index := findCard(card)

	if in {
		// move into
		if index == -1 {
			// append
			metaCards = append(metaCards, card)
			logrus.Info("handleCardMoved append")
			go processCardIssueDeadline(card)

		} else {
			// update
			metaCards[index] = card
			logrus.Info("handleCardMoved update")
		}

	} else {
		// move out
		if index == -1 {
			logrus.Info("handleCardMoved ignore")
		} else {
			// delete
			metaCards[index] = metaCards[len(metaCards)-1]
			metaCards[len(metaCards)-1] = nil
			metaCards = metaCards[:len(metaCards)-1]
			logrus.Info("handleCardMoved delete")
			go deleteCardIssueDeadline(card)
		}
	}
	return nil
}

func getIssueWithCard(card *github.ProjectCard) (*github.Issue, error) {
	contentURL := card.GetContentURL()
	if contentURL != "" {
		owner, repo, num, err := parseIssueURL(contentURL)
		if err != nil {
			return nil, err
		}

		ctx := context.Background()
		issue, _, err := client.Issues.Get(ctx, owner, repo, num)
		if err != nil {
			return nil, err
		}
		return issue, nil
	}
	return nil, errors.New("card is not issue")
}

func processCardIssueDeadline(card *github.ProjectCard) {
	issue, err := getIssueWithCard(card)
	if err != nil {
		logrus.Warning("failed to get issue with card: ", err)
		return
	}
	processIssueDeadline(issue)
}

func deleteCardIssueDeadline(card *github.ProjectCard) {
	issue, err := getIssueWithCard(card)
	if err != nil {
		logrus.Warning("failed to get issue with card: ", err)
		return
	}
	id := issue.GetID()
	logrus.Infof("delete issue %d deadline", id)
	err = deleteIssueDeadline(id)
	if err != nil {
		logrus.Warning("failed to delete issue deadline: ", err)
		return
	}
}

func PrepareKanbanMetadata() error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	metaCards = []*github.ProjectCard{}
	metaColumns = []*github.ProjectColumn{}

	projects, err := getProjects()
	if err != nil {
		return err
	}
	for _, pro := range projects {
		if pro.GetName() == TargetProject {
			columns, err := getProjectColumns(pro)
			if err != nil {
				return err
			}
			for _, col := range columns {
				if col.GetName() != TestingColumnName && col.GetName() != DevelopingColumnName {
					continue
				}

				cards, err := getColumnCards(col)
				if err != nil {
					return err
				}

				for _, card := range cards {
					columnID := col.GetID()
					card.ColumnID = &columnID
					metaCards = append(metaCards, card)
				}

				logrus.Infof("got %v cards in column \"%v\"", len(cards), col.GetName())
			}
			metaColumns = append(metaColumns, columns...)
		}
	}

	return nil
}

func moveCard(card *github.ProjectCard, column *github.ProjectColumn) error {
	ctx := context.Background()
	opts := &github.ProjectCardMoveOptions{
		Position: "top",
		ColumnID: column.GetID(),
	}

	_, err := client.Projects.MoveProjectCard(ctx, card.GetID(), opts)
	if err != nil {
		return err
	}
	return nil
}

func moveIssue(issue *github.Issue, column *github.ProjectColumn) error {
	for _, card := range metaCards {
		if card.GetContentURL() == issue.GetURL() && card.GetColumnID() != column.GetID() {
			err := moveCard(card, column)
			if err == nil {
				columnID := column.GetID()
				card.ColumnID = &columnID
			}
			return err
		}
	}
	return errNotInTargetCol
}

func moveIssueToColumn(issue *github.Issue, columnName string) error {
	for _, col := range metaColumns {
		if col.GetName() == columnName {
			return moveIssue(issue, col)
		}
	}
	return fmt.Errorf("no column named %v in project %v", columnName, TargetProject)
}

func MoveToTesting(issue *github.Issue) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	return moveIssueToColumn(issue, TestingColumnName)
}

func MoveToDeveloping(issue *github.Issue) error {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	return moveIssueToColumn(issue, DevelopingColumnName)
}

func getCardColumn(card *github.ProjectCard) (*github.ProjectColumn, error) {
	for _, col := range metaColumns {
		if col.GetID() == card.GetColumnID() {
			return col, nil
		}
	}

	return nil, errNotInTargetCol
}

func GetIssueColumn(issue *github.Issue) (*github.ProjectColumn, error) {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	for _, card := range metaCards {
		if card.GetContentURL() == issue.GetURL() {
			for _, col := range metaColumns {
				if col.GetID() == card.GetColumnID() {
					return col, nil
				}
			}
		}
	}

	return nil, errNotInTargetCol
}

func isIssueInTargetColumns(issue *github.Issue) bool {
	column, err := GetIssueColumn(issue)
	if err != nil {
		return false
	}
	return isTargetColumn(column)
}

var regIssueURL = regexp.MustCompile(`/repos/([^/]+)/([^/]+)/issues/(\d+)$`)

func parseIssueURL(contentURL string) (owner, repo string, number int, err error) {
	match := regIssueURL.FindStringSubmatch(contentURL)
	if match == nil {
		err = fmt.Errorf("invalid issue url %q", contentURL)
		return
	}
	owner = match[1]
	repo = match[2]
	number, err = strconv.Atoi(match[3])
	return
}

var regRepoURL = regexp.MustCompile(`/repos/([^/]+)/([^/]+)$`)

func parseRepoURL(repoURL string) (owner, repo string, err error) {
	match := regRepoURL.FindStringSubmatch(repoURL)
	if match == nil {
		err = fmt.Errorf("invalid repo url %q", repoURL)
		return
	}
	owner = match[1]
	repo = match[2]
	return
}

func checkIssueDeadlineForAllCards() {
	cardsLock.Lock()
	defer cardsLock.Unlock()

	for _, card := range metaCards {
		contentURL := card.GetContentURL()
		if contentURL == "" {
			// not issue
			continue
		}
		issueDeadline, err := getIssueDeadlineByURL(contentURL)
		if err != nil {
			logrus.Warningf("failed to get issue deadline by url %q: %v", contentURL, err)
			continue
		}
		if issueDeadline == nil {
			continue
		}
		if isDeadlinePassed(issueDeadline.date) {
			owner, repo, num, err := parseIssueURL(contentURL)
			if err != nil {
				logrus.Warning("failed to parse issue url:", err)
				continue
			}

			logrus.Infof("%s/%s %d deadline has passed", owner, repo, num)
			err = addDelayedLabelToIssueAux(owner, repo, num)
			if err != nil {
				logrus.Warning("failed to add delayed label to issue: ", err)
			}
		}
	}
}
