package main

import (
	"os"
	"strconv"
)

var (
	// OrgName is the organization name to be working on.
	OrgName = "linuxdeepin"
	// WebhookSecret is the webhook secret set in the Github Apps installation page.
	WebhookSecret = ""
	//TargetProject is the project that this app will try to manage.
	TargetProject = "deepin 系统发布看板"
	// TestingColumnName is the name of the column intend to be used as in the testing phase.
	TestingColumnName = "测试"
	// DevelopingColumnName is the name of the column intend to be used as in the developing phase.
	DevelopingColumnName = "开发"
	// QATeamName is the name of the testers' team.
	QATeamName = "QA Team"
	// DevTeamName is the name of the devs' team.
	DevTeamName = "Developer Team"
	// PEMFilePath is path to the pem file.
	PEMFilePath = ""
	// AppInstallationID is the ID of the installation,
	// which will show in the address bar if you're trying to configure a github app.
	AppInstallationID int
	// ServePort is the port will be used.
	ServePort = 7788
)

func init() {
	orgname, found := os.LookupEnv("ORG_NAME")
	if found {
		OrgName = orgname
	}
	webhooksecret, found := os.LookupEnv("WEBHOOK_SECRET")
	if found {
		WebhookSecret = webhooksecret
	}
	targetproject, found := os.LookupEnv("PROJECT_NAME")
	if found {
		TargetProject = targetproject
	}
	testingcolumnname, found := os.LookupEnv("TESTING_COL_NAME")
	if found {
		TestingColumnName = testingcolumnname
	}
	developingcolumnname, found := os.LookupEnv("DEVELOPING_COL_NAME")
	if found {
		DevelopingColumnName = developingcolumnname
	}
	qateamname, found := os.LookupEnv("QA_TEAM_NAME")
	if found {
		QATeamName = qateamname
	}
	devteamname, found := os.LookupEnv("DEV_TEAM_NAME")
	if found {
		DevTeamName = devteamname
	}
	pemfilepath, found := os.LookupEnv("PEM_FILE")
	if found {
		PEMFilePath = pemfilepath
	}
	appinstallationid, found := os.LookupEnv("APP_INSTALLATION_ID")
	if found {
		AppInstallationID, _ = strconv.Atoi(appinstallationid)
	}
	serveport, found := os.LookupEnv("SERVE_PORT")
	if found {
		ServePort, _ = strconv.Atoi(serveport)
	}
}
