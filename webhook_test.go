package chompy

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"testing"
)

const payload = `payload=%7B%22zen%22%3A%22Anything+added+dilutes+everything+else.%22%2C%22hook_id%22%3A13062021%2C%22hook%22%3A%7B%22type%22%3A%22Organization%22%2C%22id%22%3A13062021%2C%22name%22%3A%22web%22%2C%22active%22%3Atrue%2C%22events%22%3A%5B%22issues%22%2C%22push%22%5D%2C%22config%22%3A%7B%22content_type%22%3A%22form%22%2C%22insecure_ssl%22%3A%220%22%2C%22secret%22%3A%22%2A%2A%2A%2A%2A%2A%2A%2A%22%2C%22url%22%3A%22https%3A%2F%2Fchompy-ws.appspot.com%2Fwebhook%22%7D%2C%22updated_at%22%3A%222017-04-06T04%3A37%3A16Z%22%2C%22created_at%22%3A%222017-04-06T04%3A37%3A16Z%22%2C%22url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fhooks%2F13062021%22%2C%22ping_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fhooks%2F13062021%2Fpings%22%7D%2C%22organization%22%3A%7B%22login%22%3A%22webscale-networks%22%2C%22id%22%3A4175214%2C%22url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%22%2C%22repos_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Frepos%22%2C%22events_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fevents%22%2C%22hooks_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fhooks%22%2C%22issues_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fissues%22%2C%22members_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fmembers%7B%2Fmember%7D%22%2C%22public_members_url%22%3A%22https%3A%2F%2Fapi.github.com%2Forgs%2Fwebscale-networks%2Fpublic_members%7B%2Fmember%7D%22%2C%22avatar_url%22%3A%22https%3A%2F%2Favatars1.githubusercontent.com%2Fu%2F4175214%3Fv%3D3%22%2C%22description%22%3A%22%22%7D%2C%22sender%22%3A%7B%22login%22%3A%22hallerm%22%2C%22id%22%3A4911396%2C%22avatar_url%22%3A%22https%3A%2F%2Favatars0.githubusercontent.com%2Fu%2F4911396%3Fv%3D3%22%2C%22gravatar_id%22%3A%22%22%2C%22url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%22%2C%22html_url%22%3A%22https%3A%2F%2Fgithub.com%2Fhallerm%22%2C%22followers_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Ffollowers%22%2C%22following_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Ffollowing%7B%2Fother_user%7D%22%2C%22gists_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Fgists%7B%2Fgist_id%7D%22%2C%22starred_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Fstarred%7B%2Fowner%7D%7B%2Frepo%7D%22%2C%22subscriptions_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Fsubscriptions%22%2C%22organizations_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Forgs%22%2C%22repos_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Frepos%22%2C%22events_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Fevents%7B%2Fprivacy%7D%22%2C%22received_events_url%22%3A%22https%3A%2F%2Fapi.github.com%2Fusers%2Fhallerm%2Freceived_events%22%2C%22type%22%3A%22User%22%2C%22site_admin%22%3Afalse%7D%7D`
const sig = `sha1=22b924e429d04e362c50723fbccd0e5c6c758373`
const key = "Rt9U2OekTf69g2UQ"

func TestSig(t *testing.T) {
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(payload))
	psig := "sha1=" + hex.EncodeToString(h.Sum(nil))
	if psig != sig {
		t.Fatal("Bad sig: ", psig)
	}
}
