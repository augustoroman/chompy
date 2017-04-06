package chompy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	netmail "net/mail"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/mail"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/appengine/user"

	"github.com/go-martini/martini"
)

var (
	showRewardTpl        = template.Must(template.ParseFiles("templates/show_reward.html"))
	usedRewardTpl        = template.Must(template.ParseFiles("templates/used_reward.html"))
	homeHtmlTpl          = template.Must(template.ParseFiles("templates/home.html"))
	emailTextTpl         = template.Must(template.ParseFiles("templates/email.txt"))
	emailHtmlTpl         = template.Must(template.ParseFiles("templates/email.html"))
	donationEmailTextTpl = template.Must(template.ParseFiles("templates/donation_email.txt"))
	donationEmailHtmlTpl = template.Must(template.ParseFiles("templates/donation_email.html"))
	configHtmlTpl        = template.Must(template.ParseFiles("templates/config.html"))
)

const home = "/me"

func init() {
	m := martini.Classic()
	// Provide the appengine context to all requests
	m.Use(func(ctx martini.Context, r *http.Request) {
		ctx.MapTo(appengine.NewContext(r), (*context.Context)(nil))
	})

	// Handle one-time initialization, including secrets setup.
	m.Get("/config", Configure)
	m.Post("/config", Configure)

	m.Post("/webhook", HandleWebhook)

	m.Put("/r", AddReward)
	m.Get("/r/:id", ShowReward)
	m.Post("/r/:id", DispenseReward)
	m.Post("/donate", DonateRewards)
	m.Get(home, ShowHome)
	http.Handle("/", m)
}

func grantReward(c context.Context, r *http.Request, email, typ, desc string) (code int, err error) {
	email = strings.Replace(email, "(", "<", -1)
	email = strings.Replace(email, ")", ">", -1)

	addr, _ := netmail.ParseAddress(email)

	reward := Reward{
		Ip: r.RemoteAddr,

		Email:        email,
		EmailAddress: addr.Address,
		Type:         typ,
		Description:  desc,

		Granted: time.Now(),
		// Dispensed is left empty
	}

	uid := reward.Uid()
	key := uid.Key(c)

	if err := datastore.Get(c, key, &reward); err == nil {
		log.Errorf(c, "Duplicate reward attempt %v: %#v", key, reward)
		return http.StatusConflict, fmt.Errorf("Reward already issued")
	}

	if _, err := datastore.Put(c, key, &reward); err != nil {
		log.Errorf(c, "Failed to save reward %v: %v\nReward:%#v", key, err, reward)
		return http.StatusInternalServerError, fmt.Errorf("Failed to save reward")
	}

	retrievalUrl := fmt.Sprintf("http://%s/r/%s", r.Host, uid)

	log.Debugf(c, "Host: [%s] Retrieval url: [%s]", r.RequestURI, retrievalUrl)

	data := map[string]string{
		"credit_url": retrievalUrl,
		"reason":     reward.Reason(),
		"home_url":   fmt.Sprintf("http://%s/me", r.Host),
	}

	msg := &mail.Message{
		Sender:   fmt.Sprintf("Chompy <notify@%s.appspotmail.com>", appengine.AppID(c)),
		To:       []string{email},
		Subject:  "You've got candy!",
		Body:     renderTemplateOrDie(emailTextTpl, data),
		HTMLBody: renderTemplateOrDie(emailHtmlTpl, data),
	}
	if err := mail.Send(c, msg); err != nil {
		log.Errorf(c, "Couldn't send email for reward %v: %v\n%v", key, err, reward)
		return http.StatusInternalServerError, fmt.Errorf("Failed to send notification email")
	}

	log.Infof(c, "Granted reward %v and sent notification email: %#v", key, reward)

	return 200, nil
}

func AddReward(w http.ResponseWriter, r *http.Request, c context.Context) {
	cfg, err := getConfig(c)
	if err != nil {
		log.Criticalf(c, "Cannot load configuration: %v", err)
		http.Error(w, "Cannot load configuration", http.StatusInternalServerError)
		return
	}

	if r.FormValue("auth") != cfg.SecretAuthToken {
		log.Errorf(c, "Unauthorized request, form values: %v", r.Form)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	email, typ, desc := r.FormValue("email"), r.FormValue("type"), r.FormValue("desc")
	if email == "" || typ == "" || desc == "" {
		log.Errorf(c, "Malformatted request, from values: %v", r.Form)
		http.Error(w,
			fmt.Sprintf("Missing field: email:%q type:%q desc:%q", email, typ, desc),
			http.StatusBadRequest)
		return
	}

	if code, err := grantReward(c, r, email, typ, desc); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
}

func renderTemplateOrDie(t *template.Template, data interface{}) string {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

func ShowReward(w http.ResponseWriter, r *http.Request, c context.Context, p martini.Params) {
	reward, err := loadReward(c, Uid(p["id"]))
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, "No such reward", http.StatusNotFound)
		return
	} else if err != nil {
		log.Criticalf(c, "Failed to read %s: %v", p["id"], err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !reward.Available() {
		if err := usedRewardTpl.Execute(w, nil); err != nil {
			log.Criticalf(c, "Failed to render used template: %v", err)
		}
		return
	}
	if err := showRewardTpl.Execute(w, nil); err != nil {
		log.Criticalf(c, "Failed to render show template: %v", err)
	}
}
func DispenseReward(w http.ResponseWriter, r *http.Request, c context.Context, p martini.Params) {
	cfg, err := getConfig(c)
	if err != nil {
		log.Criticalf(c, "Cannot load configuration: %v", err)
		http.Error(w, "Cannot load configuration", http.StatusInternalServerError)
		return
	}

	reward, err := loadReward(c, Uid(p["id"]))
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, "No such reward", http.StatusNotFound)
		return
	} else if err != nil {
		log.Criticalf(c, "Failed to read %s: %v", p["id"], err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !reward.Available() {
		log.Errorf(c, "Reward %s cannot be dispensed: %#v", p["id"], reward)
		http.Error(w, "Not available", http.StatusGone)
		return
	}
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
	reward.Dispensed = time.Now()
	if _, err := datastore.Put(c, reward.Uid().Key(c), &reward); err != nil {
		log.Criticalf(c, "Cannot update reward %s: %v\n%s", reward.Uid(), err, reward)
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func DonateRewards(w http.ResponseWriter, r *http.Request, c context.Context) {
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Not logged in", http.StatusUnauthorized)
		return
	}
	// Allow admins to donate rewards on behalf of other users.
	// Use with great care.
	if u.Admin && r.FormValue("user") != "" {
		u.Email = r.FormValue("user")
	}

	rawemail, numstr, msg := r.FormValue("email"), r.FormValue("num"), r.FormValue("msg")
	rawemail = strings.Replace(rawemail, "(", "<", -1)
	rawemail = strings.Replace(rawemail, ")", ">", -1)
	addr, err2 := netmail.ParseAddress(rawemail)
	num, err := strconv.Atoi(numstr)
	if err2 != nil || err != nil || num == 0 || addr == nil {
		log.Errorf(c, "Bad inputs: email=%q numstr=%q err=%v err2=%v", rawemail, numstr, err, err2)
		http.Error(w, "Bad inputs", http.StatusBadRequest)
		return
	}
	email := addr.Address

	if email == u.Email {
		log.Warningf(c, "%q may be a narcissist: n=%d", u.Email, num)
		http.Error(w, "Donating to yourself?  Really?", http.StatusBadRequest)
		return
	}

	var rewards []Reward
	keys, err := datastore.NewQuery("rewards").
		Filter("EmailAddress =", u.Email).
		Order("-Granted").
		GetAll(c, &rewards)
	if err != nil {
		log.Criticalf(c, "Failed to load rewards for %v", u)
		http.Error(w, "Internal error, no rewards have been donated",
			http.StatusInternalServerError)
		return
	}

	available := 0
	for _, r := range rewards {
		if r.Available() {
			available++
		}
	}

	if num > available {
		num = available
	}

	var donatedKeys []*datastore.Key
	var donatedRewards []Reward
	for i := len(rewards) - 1; i >= 0 && len(donatedRewards) < num; i-- {
		reward, key := &rewards[i], keys[i]
		if !reward.Available() {
			continue
		}
		reward.DonateTo(email, msg)
		donatedKeys = append(donatedKeys, key)
		donatedRewards = append(donatedRewards, *reward)
	}

	log.Infof(c, "%q donated %d credits to %q  -- Msg: %q",
		u.Email, len(donatedKeys), email, msg)

	_, err = datastore.PutMulti(c, donatedKeys, donatedRewards)
	if err != nil {
		log.Criticalf(c, "Failed to save donations: %v", err)
		http.Error(w, "Internal error: some rewards may have been donated.  Sorry.",
			http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"message":  msg,
		"from":     u.Email,
		"N":        len(donatedRewards),
		"home_url": fmt.Sprintf("http://%s/me", r.Host),
	}
	emailMessage := &mail.Message{
		Sender:   fmt.Sprintf("Chompy <notify@%s.appspotmail.com>", appengine.AppID(c)),
		To:       []string{email},
		Subject:  "You've got candy!",
		Body:     renderTemplateOrDie(donationEmailTextTpl, data),
		HTMLBody: renderTemplateOrDie(donationEmailHtmlTpl, data),
	}
	if err := mail.Send(c, emailMessage); err != nil {
		log.Errorf(c, "Couldn't send email for donation: %v", err)
		http.Error(w, "Donations sent, but notification email failed.  "+
			"Tell them about the credits", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"donated": len(donatedRewards),
		"to":      email,
	})
}

func ShowHome(w http.ResponseWriter, r *http.Request, c context.Context) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	u := user.Current(c)
	if u == nil {
		url, _ := user.LoginURL(c, home)
		fmt.Fprintf(w, `<a href="%s">sign in</a>`, url)
		return
	}
	logoutUrl, _ := user.LogoutURL(c, home)

	if u.Admin && r.FormValue("user") != "" {
		u.Email = r.FormValue("user")
	}

	var rewards []Reward
	_, err := datastore.NewQuery("rewards").
		Filter("EmailAddress =", u.Email).
		Order("-Granted").
		GetAll(c, &rewards)
	if err != nil {
		log.Criticalf(c, "Failed to load rewards for %v", u)
	}

	numAvailable := 0
	for _, rw := range rewards {
		if rw.Available() {
			numAvailable++
		}
	}

	params := struct {
		User           *user.User
		LogoutUrl      string
		Rewards        []Reward
		TotalCount     int
		AvailableCount int
		Status         Status
	}{u, logoutUrl, rewards, len(rewards), numAvailable, GetChompyStatus(c)}
	if err := homeHtmlTpl.Execute(w, params); err != nil {
		log.Criticalf(c, "Failed to render home template: %v", err)
	}
}

func must(data []byte, err error) string {
	if err != nil {
		panic(err)
	}
	return string(data)
}
