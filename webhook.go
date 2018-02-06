package chompy

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
)

func HandleWebhook(w http.ResponseWriter, r *http.Request, c context.Context) {
	cfg, err := getConfig(c)
	if err != nil {
		log.Criticalf(c, "Cannot load configuration: %v", err)
		http.Error(w, "Cannot load configuration", http.StatusInternalServerError)
		return
	}

	if strings.HasPrefix(r.Header.Get("User-Agent"), "GitHub-Hookshot/") {
		handleGithubWebhook(w, r, c, cfg)
	} else {
		http.Error(w, "Unknown webhook type", http.StatusBadRequest)
	}
}

func handleGithubWebhook(w http.ResponseWriter, r *http.Request, c context.Context, cfg Configuration) {
	(&GithubWebhookRequest{w, r, c, cfg.SecretAuthToken, cfg.GithubUsers}).Handle()
}

func validateGithubWebhook(payload []byte, key, sig string) error {
	h := hmac.New(sha1.New, []byte(key))
	h.Write(payload)
	computedSig := "sha1=" + hex.EncodeToString(h.Sum(nil))
	if computedSig != sig {
		return fmt.Errorf("Computed signature [%s] does not match provided signature [%s]",
			computedSig, sig)
	}
	return nil
}

type GithubWebhookRequest struct {
	w http.ResponseWriter
	r *http.Request
	c context.Context

	SecretAuthToken string
	Users           []GithubUserInfo
}

func (g *GithubWebhookRequest) LookGithubUser(username string) *GithubUserInfo {
	for _, user := range g.Users {
		if strings.EqualFold(user.Username, username) {
			return &user
		}
	}
	return nil
}

func (g *GithubWebhookRequest) Handle() {
	event := g.r.Header.Get("X-GitHub-Event")

	body, err := ioutil.ReadAll(g.r.Body)
	if err != nil {
		log.Errorf(g.c, "Can't read request body: %v", err)
		http.Error(g.w, "Can't read request body", http.StatusBadRequest)
		return
	}

	if err := validateGithubWebhook(body, g.SecretAuthToken, g.r.Header.Get("X-Hub-Signature")); err != nil {
		log.Errorf(g.c, "Bad webhook signature: %v", err)
		http.Error(g.w, "Bad signature", http.StatusBadRequest)
		return
	}

	log.Debugf(g.c, "Webhook Event: %s", event)
	log.Debugf(g.c, "Webhook body: %s", string(body))

	switch event {
	// case "issues":
	// 	g.HandleIssue(body)
	case "pull_request":
		g.HandlePullRequest(body)
	case "pull_request_review":
		g.HandlePullRequestReview(body)
	default:
		g.w.WriteHeader(http.StatusNoContent)
	}
	return
}

func (g *GithubWebhookRequest) Grant(githubUserName, typ, desc string) {
	user := g.LookGithubUser(githubUserName)
	if user == nil {
		log.Errorf(g.c, "Github username %q not configured.", githubUserName)
		g.w.WriteHeader(http.StatusNoContent)
		return
	}

	if code, err := grantReward(g.c, g.r, user.Email, typ, desc); err != nil {
		http.Error(g.w, err.Error(), code)
		return
	}

	fmt.Fprintln(g.w, "Thanks github\n")
}

func (g *GithubWebhookRequest) HandleIssue(body []byte) {
	type EventData struct {
		Action string
		Issue  struct {
			HtmlUrl  string `json:"html_url"`
			Number   int
			Assignee struct{ Login string }
		}
	}
	var eventData EventData
	if err := json.Unmarshal(body, &eventData); err != nil {
		log.Criticalf(g.c, "Cannot parse json payload: %v", err)
		http.Error(g.w, "Can't parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Debugf(g.c, "Parsed JSON: %#v", eventData)

	if eventData.Action != "closed" {
		g.w.WriteHeader(http.StatusNoContent)
		return
	}

	g.Grant(eventData.Issue.Assignee.Login, "issue-closed", eventData.Issue.HtmlUrl)
}
func (g *GithubWebhookRequest) HandlePullRequest(body []byte) {
	type EventData struct {
		Action      string
		PullRequest struct {
			HtmlUrl string `json:"html_url"`
			Number  int
			Merged  bool
			User    struct{ Login string }
		} `json:"pull_request"`
	}
	var eventData EventData
	if err := json.Unmarshal(body, &eventData); err != nil {
		log.Criticalf(g.c, "Cannot parse json payload: %v", err)
		http.Error(g.w, "Can't parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Debugf(g.c, "Parsed JSON: %#v", eventData)
	log.Debugf(g.c, "PR Action: %q merged: %v", eventData.Action, eventData.PullRequest.Merged)

	if eventData.Action != "closed" || !eventData.PullRequest.Merged {
		g.w.WriteHeader(http.StatusNoContent)
		return
	}

	g.Grant(eventData.PullRequest.User.Login, "pull-request-merged", eventData.PullRequest.HtmlUrl)
}

func (g *GithubWebhookRequest) HandlePullRequestReview(body []byte) {
	type EventData struct {
		Action string
		Review struct {
			User  struct{ Login string }
			State string
		}
		PullRequest struct {
			HtmlUrl  string `json:"html_url"`
			Number   int
			MergedAt time.Time `json:"merged_at"`
			User     struct{ Login string }
		} `json:"pull_request"`
	}
	var eventData EventData
	if err := json.Unmarshal(body, &eventData); err != nil {
		log.Criticalf(g.c, "Cannot parse json payload: %v", err)
		http.Error(g.w, "Can't parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Debugf(g.c, "Parsed JSON: %#v", eventData)
	log.Debugf(g.c, "Review Action: %q  state: %q  merged-at: %v",
		eventData.Action, eventData.Review.State, eventData.PullRequest.MergedAt)

	if eventData.Action != "submitted" || eventData.Review.State != "approved" || !eventData.PullRequest.MergedAt.IsZero() {
		g.w.WriteHeader(http.StatusNoContent)
		return
	}

	g.Grant(eventData.Review.User.Login, "pull-request-reviewed", eventData.PullRequest.HtmlUrl)
}
