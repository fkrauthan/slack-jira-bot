package main

import (
	"fmt"
	"bytes"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	gojira "github.com/plouc/go-jira-client"
)

// Configuration for the bot
type BotConfig struct {
	Username     string
	SlackAPIKey  string
	JiraUsername string
	JiraPassword string
	JiraBaseURL  string
}

func main() {
	api := getSlackAPI()

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	log.Print("main: Now listening for events")

	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.MessageEvent:
				handleIncomingMessage(ev.Msg)
			case *slack.LatencyReport:
				log.Printf("main: Current latency: %v\n", ev.Value)
			case *slack.RTMError:
				log.Printf("main: Error: %s\n", ev.Error())
			case *slack.InvalidAuthEvent:
				log.Print("main: Invalid credentials")
			default:
				// Ignore other events..
			}
		}
	}
}

func handleIncomingMessage(message slack.Msg) {
	messageText := message.Text

	if shouldIgnoreMessage(message) {
		log.Print("handleMessage: Ignoring message")
		return
	}

	matches := extractIssueIDs(messageText)

	for i := 0; i < len(matches); i++ {
		issueID := matches[i]
		log.Printf("handleMessage: Identified " + issueID + " in message")

		respondToIssueMentioned(message.Channel, issueID)
	}
}

func respondToIssueMentioned(channel string, issueID string) {
	defer func() {
		if e := recover(); e != nil {
			log.Printf("Exception responding to issue %s: %v", issueID, e)
		}
	}()

	api := getSlackAPI()

	params := slack.PostMessageParameters{
		Username: getConfig().Username,
		Markdown: true,
	}

	issueData := getJiraIssue(issueID)

	api.PostMessage(channel, formatMessage(issueData), params)
}

func getSlackAPI() *slack.Client {
	return slack.New(getConfig().SlackAPIKey)
}

func getChannel(channelID string) (*slack.Channel, error) {
	api := getSlackAPI()

	return api.GetChannelInfo(channelID)
}

func formatMessage(issue gojira.Issue) string {
	var message bytes.Buffer

	message.WriteString(fmt.Sprintf(
		"> <%s|%s> :traffic_light: *Status:* %s :memo: *Summary:* %s\n",
		getJiraURL(issue.Key),
		issue.Key,
		issue.Fields.Status.Name,
		issue.Fields.Summary,
	))
	message.WriteString(fmt.Sprintf(
		"> :bust_in_silhouette: *Creator:* %s, *Assignee:* %s\n",
		issue.Fields.Reporter.DisplayName,
		issue.Fields.Assignee.DisplayName,
	))
	message.WriteString(fmt.Sprintf(
		"> :calendar: *Created:* <!date^%d^{date} at {time}|%s>",
		issue.CreatedAt.Unix(),
		issue.Fields.Created,
	))

	return message.String()
}

func getJiraURL(issueKey string) string {
	return getConfig().JiraBaseURL + "/browse/" + issueKey
}

func getJiraIssue(issueID string) gojira.Issue {
	jiraAPIPath := "/rest/api/latest"
	jiraActivityPath := ""

	jira := gojira.NewJira(
		getConfig().JiraBaseURL,
		jiraAPIPath,
		jiraActivityPath,
		&gojira.Auth{
			Login:    getConfig().JiraUsername,
			Password: getConfig().JiraPassword,
		},
	)

	issueData := jira.Issue(issueID)

	return issueData
}

func shouldIgnoreMessage(message slack.Msg) bool {
	return message.Username == getConfig().Username || message.SubType == "bot_message"
}

func extractIssueIDs(message string) []string {
	re := regexp.MustCompile(`\b(\w+)-(\d+)\b`)
	matches := re.FindAllString(message, -1)

	// @see http://www.dotnetperls.com/remove-duplicates-slice
	encountered := map[string]bool{}
	result := []string{}

	for v := range matches {
		// convert all match to upper case.
		matches[v] = strings.ToUpper(matches[v])
		if encountered[matches[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[matches[v]] = true
			// Append to result slice.
			result = append(result, matches[v])
		}
	}
	// Return the new slice.
	return result
}

func getConfig() BotConfig {
	return BotConfig{
		Username:     "JiraBot",
		SlackAPIKey:  os.Getenv("SLACK_API_KEY"),
		JiraBaseURL:  os.Getenv("JIRA_BASEURL"),
		JiraUsername: os.Getenv("JIRA_USERNAME"),
		JiraPassword: os.Getenv("JIRA_PASSWORD"),
	}
}
