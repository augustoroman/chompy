package chompy

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
)

type Reward struct {
	Ip string

	Email        string
	EmailAddress string
	Type         string
	Description  string

	// Donation tracking
	PreviousOwners  []string // this should be the parsed EmailAddress field
	DonationDates   []time.Time
	DonationMessage []string

	Granted   time.Time
	Dispensed time.Time
}

type Uid string

func (k Uid) Key(c context.Context) *datastore.Key {
	return datastore.NewKey(c, "rewards", string(k), 0, nil)
}

func (r Reward) Uid() Uid {
	hash_bytes := sha1.Sum([]byte(fmt.Sprintf("%s•%s•%s", r.Email, r.Type, r.Description)))
	return Uid(hex.EncodeToString(hash_bytes[:]))
}

func (r Reward) Available() bool { return r.Dispensed.IsZero() && !r.Granted.IsZero() }
func (r Reward) Status() string {
	if r.Available() {
		return "available"
	} else {
		return "used"
	}
}
func (r Reward) Reason() string {
	switch r.Type {
	case "issue-closed":
		return fmt.Sprintf("You closed an issue: %s", r.Description)
	case "commit-merged":
		return fmt.Sprintf("You committed some code (%s)", r.Description)
	case "thanks":
		return "You did something nice!"
	case "manual":
		return r.Description
	}
	return "Enjoy!"
}

func (r *Reward) DonateTo(email, msg string) {
	r.PreviousOwners = append(r.PreviousOwners, r.EmailAddress)
	r.DonationDates = append(r.DonationDates, time.Now())
	r.DonationMessage = append(r.DonationMessage, msg)
	// Don't change the Email field because it's used for the Uid of the reward! :-/
	r.EmailAddress = email
}
func (r Reward) LastDonationTime() time.Time {
	N := len(r.DonationDates)
	if N == 0 {
		return time.Time{}
	}
	return r.DonationDates[N-1]
}
func (r Reward) LastDonor() string {
	N := len(r.PreviousOwners)
	if N == 0 {
		return ""
	}
	return r.PreviousOwners[N-1]
}
func (r Reward) LastDonorMessage() string {
	N := len(r.DonationMessage)
	if N == 0 {
		return ""
	}
	return r.DonationMessage[N-1]
}

func loadReward(c context.Context, id Uid) (Reward, error) {
	var r Reward
	err := datastore.Get(c, id.Key(c), &r)
	return r, err
}
