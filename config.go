package chompy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/appengine/user"
)

type Configuration struct {
	AgentURL        string
	SecretAuthToken string
	DispenseTime    time.Duration
	GithubUsers     []GithubUserInfo
}

// Allows sending candy to github users.
type GithubUserInfo struct {
	Username, Email string
}

func (c *Configuration) StatusUrl() string {
	return strings.TrimRight(c.AgentURL, "/") + "/status"
}
func (c *Configuration) DispenseUrl() string {
	path := fmt.Sprintf("/dispense?amount=%f", c.DispenseTime.Seconds())
	return strings.TrimRight(c.AgentURL, "/") + path
}

func (cfg Configuration) Key(c context.Context) *datastore.Key {
	return datastore.NewKey(c, "Configuration", "config", 0, nil)
}

func getConfig(c context.Context) (Configuration, error) {
	var cfg Configuration
	err := datastore.Get(c, cfg.Key(c), &cfg)
	return cfg, err
}

func Dispense(w http.ResponseWriter, r *http.Request, c context.Context) {
	u := user.Current(c)
	if !u.Admin {
		http.NotFound(w, r)
		return
	}
	cfg, err := getConfig(c)
	if err != nil {
		log.Criticalf(c, "Cannot load configuration: %v", err)
		http.Error(w, "Cannot load configuration", http.StatusInternalServerError)
		return
	}

	dt, err := time.ParseDuration(r.FormValue("time"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg.DispenseTime = dt
	client := urlfetch.Client(c)
	resp, err := client.Post(cfg.DispenseUrl(), "", nil)
	if err != nil {
		log.Criticalf(c, "Could not contact snackbot: %v", err)
		http.Error(w, "Cannot contact snackbot, please try again later.", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respContent, _ := ioutil.ReadAll(resp.Body)
		log.Criticalf(c, "Could not contact snackbot: %s", respContent)
		http.Error(w, "Cannot contact snackbot, please try again later.", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func Configure(w http.ResponseWriter, r *http.Request, c context.Context) {
	u := user.Current(c)
	if !u.Admin {
		http.NotFound(w, r)
		return
	}
	cfg, err := getConfig(c)
	if err != nil && err != datastore.ErrNoSuchEntity {
		err = fmt.Errorf("Cannot load configuration: %v", err)
		log.Criticalf(c, "%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ConfigPageParams struct {
		Message string
		Config  Configuration
	}

	var renderParams ConfigPageParams
	if r.Method == "POST" {
		cfg.AgentURL = r.FormValue("agent-url")
		cfg.SecretAuthToken = r.FormValue("secret-token")
		cfg.DispenseTime, err = time.ParseDuration(r.FormValue("dispense-time"))
		if err == nil && (cfg.DispenseTime <= 0 || cfg.DispenseTime >= 30*time.Second) {
			err = fmt.Errorf("Dispense time is unreasonable: %v", cfg.DispenseTime)
		}
		usernames := r.Form["username"]
		useremails := r.Form["useremail"]
		if len(usernames) != len(useremails) {
			log.Errorf(c, "Username form doesn't match useremail form:\nusername: %q\nuseremail: %q",
				usernames, useremails)
			http.Error(w, "username list should match useremail list", http.StatusBadRequest)
			return
		}
		cfg.GithubUsers = nil
		for idx := range usernames {
			username := usernames[idx]
			email := useremails[idx]
			if username == "" || email == "" {
				continue
			}
			cfg.GithubUsers = append(cfg.GithubUsers, GithubUserInfo{Username: username, Email: email})
		}

		if err == nil {
			_, err = datastore.Put(c, cfg.Key(c), &cfg)
		}

		if err != nil {
			log.Criticalf(c, "Failed to save configuration: %v", err)
			renderParams.Message = fmt.Sprintf("Failed to save configuration: %v", err)
		} else {
			log.Infof(c, "Configuration updated: %#v", cfg)
			renderParams.Message = "Configuration updated!"
		}
	}
	renderParams.Config = cfg
	if err := configHtmlTpl.Execute(w, renderParams); err != nil {
		log.Criticalf(c, "Failed to render config page: %v", err)
	}
}
