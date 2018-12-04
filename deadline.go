package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"regexp"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

type IssueDeadline struct {
	id        int64
	date      time.Time
	directive string
	url       string
}

var regDirectiveDay = regexp.MustCompile(`<(\d+)>`)
var regDirectiveMonthDay = regexp.MustCompile(`<(\d+)-(\d+)>`)
var regDirectiveYMD = regexp.MustCompile(`<(\d+)-(\d+)-(\d+)>`)
var regDirectiveThisWeekEN = regexp.MustCompile(`<z(\d)>`)
var regDirectiveNextWeekEN = regexp.MustCompile(`<xz(\d)>`)
var regDirectiveThisWeekCN = regexp.MustCompile(`<周([一二三四五六日])>`)
var regDirectiveNextWeekCN = regexp.MustCompile(`<下周([一二三四五六日])>`)

var defaultLoc *time.Location

func init() {
	var err error
	defaultLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logrus.Fatal("failed to load location Asia/Shanghai: ", err)
	}
}

// 获取 t 所在的这周内，星期几的日期。
func getDateInWeek(t time.Time, weekday int) time.Time {
	tWeekday := int(t.Weekday())
	if tWeekday == 0 {
		tWeekday = 7
	}
	return t.AddDate(0, 0, weekday-tWeekday)
}

var chineseWeekdays = []string{"", "一", "二", "三", "四", "五", "六", "日"}

func parseChineseWeekday(str string) (int, error) {
	if str == "" {
		return 0, errors.New("invalid value")
	}
	for idx, value := range chineseWeekdays {
		if value == str {
			return idx, nil
		}
	}
	return 0, errors.New("invalid value")
}

const layoutYMD = "2006-01-02"

func formatDate(t time.Time) string {
	return t.Format(layoutYMD)
}

func getDeadlineFromTitle(now time.Time, str string) (date time.Time, directive string, err error) {
	now = now.In(defaultLoc)
	var day int
	var month int
	var year int
	var n int

	match := regDirectiveDay.FindStringSubmatch(str)
	if match != nil {
		day, err = strconv.Atoi(match[1])
		if err != nil {
			return
		}
		date = time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, defaultLoc)
		directive = match[0]
		return
	}

	match = regDirectiveMonthDay.FindStringSubmatch(str)
	if match != nil {
		month, err = strconv.Atoi(match[1])
		if err != nil {
			return
		}
		if month < int(time.January) || month > int(time.December) {
			err = errors.New("invalid month")
			return
		}

		day, err = strconv.Atoi(match[2])
		if err != nil {
			return
		}
		date = time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, defaultLoc)
		directive = match[0]
		return
	}

	match = regDirectiveYMD.FindStringSubmatch(str)
	if match != nil {
		year, err = strconv.Atoi(match[1])
		if err != nil {
			return
		}

		month, err = strconv.Atoi(match[2])
		if err != nil {
			return
		}
		if month < int(time.January) || month > int(time.December) {
			err = errors.New("invalid month")
			return
		}

		day, err = strconv.Atoi(match[3])
		if err != nil {
			return
		}
		date = time.Date(year, time.Month(month), day, 0, 0, 0, 0, defaultLoc)
		directive = match[0]
		return
	}

	match = regDirectiveThisWeekEN.FindStringSubmatch(str)
	if match != nil {
		n, err = strconv.Atoi(match[1])
		if err != nil {
			return
		}

		// n range [1,7]
		if n < 1 || n > 7 {
			err = errors.New("invalid value")
			return
		}

		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, defaultLoc)
		date = getDateInWeek(date, n)
		directive = match[0]
		return
	}

	match = regDirectiveNextWeekEN.FindStringSubmatch(str)
	if match != nil {
		n, err = strconv.Atoi(match[1])
		if err != nil {
			return
		}

		// n range [1,7]
		if n < 1 || n > 7 {
			err = errors.New("invalid value")
			return
		}

		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, defaultLoc)
		date = getDateInWeek(date, n).AddDate(0, 0, 7)
		directive = match[0]
		return
	}

	match = regDirectiveThisWeekCN.FindStringSubmatch(str)
	if match != nil {
		n, err = parseChineseWeekday(match[1])
		if err != nil {
			return
		}

		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, defaultLoc)
		date = getDateInWeek(date, n)
		directive = match[0]
		return
	}

	match = regDirectiveNextWeekCN.FindStringSubmatch(str)
	if match != nil {
		n, err = parseChineseWeekday(match[1])
		if err != nil {
			return
		}

		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, defaultLoc)
		date = getDateInWeek(date, n).AddDate(0, 0, 7)
		directive = match[0]
		return
	}

	err = errors.New("not found directive")
	return
}

func isDeadlinePassed(t time.Time) bool {
	now := time.Now().In(defaultLoc)
	return now.After(t.AddDate(0, 0, 1))
}

func getIssueDeadline(id int64) (*IssueDeadline, error) {
	var issueDeadline IssueDeadline
	err := db.QueryRow(`SELECT date,url,directive FROM issue_deadline WHERE id = ?`,
		id).Scan(&issueDeadline.date, &issueDeadline.url, &issueDeadline.directive)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, err
	default:
		issueDeadline.id = id
		return &issueDeadline, nil
	}
}

func getIssueDeadlineByURL(issueURL string) (*IssueDeadline, error) {
	var issueDeadline IssueDeadline
	err := db.QueryRow(`SELECT id,date,directive FROM issue_deadline WHERE url = ?`,
		issueURL).Scan(&issueDeadline.id, &issueDeadline.date, &issueDeadline.directive)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, err
	default:
		issueDeadline.url = issueURL
		return &issueDeadline, nil
	}
}

func addIssueDeadline(issueDeadline *IssueDeadline) error {
	_, err := db.Exec(`INSERT INTO issue_deadline (id,date,url,directive) VALUES (?,?,?,?)`,
		issueDeadline.id, issueDeadline.date, issueDeadline.url, issueDeadline.directive)
	return err
}

func updateIssueDeadline(issueDeadline *IssueDeadline) error {
	_, err := db.Exec(`UPDATE issue_deadline SET date = ?, url = ?, directive = ?  WHERE id = ?`,
		issueDeadline.date, issueDeadline.url, issueDeadline.directive, issueDeadline.id)
	return err
}

func deleteIssueDeadline(id int64) error {
	_, err := db.Exec("DELETE FROM issue_deadline WHERE id = ?", id)
	return err
}

const delayedLabelName = "delayed"

func addDelayedLabelToIssue(issue *github.Issue) error {
	for _, label := range issue.Labels {
		if label.GetName() == delayedLabelName {
			return nil
		}
	}

	owner := issue.GetRepository().GetOwner().GetLogin()
	repo := issue.GetRepository().GetName()
	num := issue.GetNumber()
	return addDelayedLabelToIssueAux(owner, repo, num)
}

func addDelayedLabelToIssueAux(owner, repo string, num int) error {
	ctx := context.Background()
	_, _, err := client.Issues.AddLabelsToIssue(ctx, owner, repo, num, []string{delayedLabelName})
	return err
}

func removeDelayedLabelForIssue(issue *github.Issue) error {
	var found bool
	for _, label := range issue.Labels {
		if label.GetName() == delayedLabelName {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	ctx := context.Background()
	owner := issue.GetRepository().GetOwner().GetLogin()
	repo := issue.GetRepository().GetName()
	num := issue.GetNumber()
	_, err := client.Issues.RemoveLabelForIssue(ctx, owner, repo, num, delayedLabelName)
	return err
}

func createIssueComment(issue *github.Issue, commentBody string) error {
	ctx := context.Background()

	var owner string
	var repo string
	if issue.Repository == nil {
		var err error
		owner, repo, err = parseRepoURL(issue.GetRepositoryURL())
		if err != nil {
			return err
		}
	} else {
		owner = issue.GetRepository().GetOwner().GetLogin()
		repo = issue.GetRepository().GetName()
	}

	num := issue.GetNumber()
	comment := new(github.IssueComment)
	comment.Body = &commentBody
	_, _, err := client.Issues.CreateComment(ctx, owner, repo, num, comment)
	return err
}

func processIssueDeadline(issue *github.Issue) {
	if !isIssueInTargetColumns(issue) {
		logrus.Infof("issue %d not in target columns", issue.GetNumber())
		return
	}

	title := issue.GetTitle()
	logrus.Infof("processIssueDeadline title: %q", title)
	id := issue.GetID()
	now := time.Now()
	date, directive, err := getDeadlineFromTitle(now, title)
	if err == nil {

		oldIssueDeadline, err := getIssueDeadline(id)
		if err != nil {
			logrus.Warning("failed to get issue deadline: ", err)
			return
		}

		var oldDirective string
		if oldIssueDeadline != nil {
			oldDirective = oldIssueDeadline.directive
		}

		if oldDirective != directive {
			// set new deadline
			logrus.Infof("set new deadline to %s %s", formatDate(date), directive)
			issueDeadline := IssueDeadline{
				id:        id,
				date:      date,
				directive: directive,
				url:       issue.GetURL(),
			}
			if oldIssueDeadline == nil {
				err = addIssueDeadline(&issueDeadline)
				if err != nil {
					logrus.Warning("failed to add issue deadline: ", err)
					return
				}
			} else {
				err = updateIssueDeadline(&issueDeadline)
				if err != nil {
					logrus.Warning("failed to update issue deadline: ", err)
					return
				}
			}

			commentBody := fmt.Sprintf("设置截止日期到 %s", formatDate(date))
			err = createIssueComment(issue, commentBody)
			if err != nil {
				logrus.Warning("failed to create issue comment: ", err)
			}
		}

		if isDeadlinePassed(date) {
			logrus.Info("deadline has passed")
			err = addDelayedLabelToIssue(issue)
			if err != nil {
				logrus.Warning("failed to add delayed label to issue: ", err)
			}
		} else {
			logrus.Info("deadline has not passed")
			err = removeDelayedLabelForIssue(issue)
			if err != nil {
				logrus.Warning("failed to remove delayed label for issue: ", err)
			}
		}

	} else {
		logrus.Info("cancel set deadline")
		err = deleteIssueDeadline(id)
		if err != nil {
			logrus.Warning("failed to delete issue deadline: ", err)
			return
		}

		err = removeDelayedLabelForIssue(issue)
		if err != nil {
			logrus.Warning("failed to remove delayed label for issue: ", err)
		}
	}
}
