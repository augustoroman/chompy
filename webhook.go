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
	event := r.Header.Get("X-GitHub-Event")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf(c, "Can't read request body: %v", err)
		http.Error(w, "Can't read request body", http.StatusBadRequest)
		return
	}

	if err := validateGithubWebhook(body, cfg.SecretAuthToken, r.Header.Get("X-Hub-Signature")); err != nil {
		log.Errorf(c, "Bad webhook signature: %v", err)
		http.Error(w, "Bad signature", http.StatusBadRequest)
		return
	}

	log.Infof(c, "Webhook Event: %s", event)
	log.Debugf(c, "Webhook body: %s", string(body))

	if event != "issues" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type EventData struct {
		Action string
		Issue  struct {
			HtmlUrl  string `json:"html_url"`
			Number   int
			Assignee struct {
				Login string
			}
		}
	}
	var eventData EventData
	if err := json.Unmarshal(body, &eventData); err != nil {
		log.Criticalf(c, "Cannot parse json payload: %v", err)
		http.Error(w, "Can't parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Infof(c, "Parsed JSON: %#v", eventData)

	if eventData.Action != "closed" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	user := cfg.LookGithubUser(eventData.Issue.Assignee.Login)
	if user == nil {
		log.Errorf(c, "Unknown github username: %q", eventData.Issue.Assignee.Login)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	typ := "issue-closed"
	desc := eventData.Issue.HtmlUrl

	if code, err := grantReward(c, r, user.Email, typ, desc); err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	fmt.Fprintln(w, "Thanks github\n")
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
