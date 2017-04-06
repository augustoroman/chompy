package chompy

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type Status struct {
	Online bool `json:"online"`
}

func GetChompyStatus(c context.Context) Status {
	cfg, err := getConfig(c)
	if err != nil {
		log.Criticalf(c, "Cannot load configuration: %v", err)
		return Status{}
	}

	client := urlfetch.Client(c)
	resp, err := client.Get(cfg.StatusUrl())
	if err != nil {
		log.Criticalf(c, "Could not contact electric imp: %v", err)
		return Status{}
	}
	defer resp.Body.Close()
	respContent, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Criticalf(c, "Could not contact electric imp: %s", respContent)
		return Status{}
	}
	var status Status
	if err := json.Unmarshal(respContent, &status); err != nil {
		log.Criticalf(c, "Cannot decode response: %v\n%s", err, respContent)
		return Status{}
	}

	return status
}
