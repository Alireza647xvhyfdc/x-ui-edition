package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSellerRouteAllowlist(t *testing.T) {
	cases := []struct {
		method, full string
		want         bool
	}{
		{http.MethodGet, "/panel/api/inbounds/list", true},
		{http.MethodGet, "/panel/api/inbounds/list/slim", true},
		{http.MethodGet, "/panel/api/inbounds/get/:id", true},
		{http.MethodPost, "/panel/api/inbounds/add", false},
		{http.MethodPost, "/panel/api/inbounds/del/:id", false},
		{http.MethodGet, "/panel/api/inbounds/allLinks", false},
		{http.MethodGet, "/panel/api/inbounds/options", false},
		{http.MethodGet, "/panel/api/server/status", false},
		{http.MethodPost, "/panel/api/inbounds/list", false},
	}
	for _, c := range cases {
		if got := sellerRouteAllowed(c.method, c.full); got != c.want {
			t.Errorf("%s %s: got %v, want %v", c.method, c.full, got, c.want)
		}
	}
}

func TestSellerClientRouteAllowlist(t *testing.T) {
	cases := []struct {
		method, full string
		want         bool
	}{
		{http.MethodGet, "/panel/api/clients/list", true},
		{http.MethodGet, "/panel/api/clients/get/:email", true},
		{http.MethodPost, "/panel/api/clients/add", true},
		{http.MethodPost, "/panel/api/clients/update/:email", true},
		{http.MethodPost, "/panel/api/clients/bulkEnable", true},
		{http.MethodGet, "/panel/api/clients/list/paged", false},
		{http.MethodPost, "/panel/api/clients/bulkDel", false},
		{http.MethodPost, "/panel/api/clients/bulkCreate", false},
		{http.MethodGet, "/panel/api/clients/subLinks/:subId", false},
		{http.MethodPost, "/panel/api/clients/:email/attach", false},
		{http.MethodPost, "/panel/api/clients/resetAllTraffics", false},
	}
	for _, c := range cases {
		if got := sellerRouteAllowed(c.method, c.full); got != c.want {
			t.Errorf("%s %s: got %v, want %v", c.method, c.full, got, c.want)
		}
	}
}

func TestFilterInboundsForSellerKeepsOnlyGrants(t *testing.T) {
	inbounds := []*model.Inbound{{Id: 1}, {Id: 2}, {Id: 3}}
	seller := &model.User{Role: model.RoleSeller, SellerInboundIds: []int{2}}
	got := filterInboundsForUser(seller, inbounds)
	if len(got) != 1 || got[0].Id != 2 {
		t.Fatalf("seller saw %+v, want only inbound 2", got)
	}
}

func TestFilterInboundsForAdminKeepsEverything(t *testing.T) {
	inbounds := []*model.Inbound{{Id: 1}, {Id: 2}}
	admin := &model.User{Id: 1, Role: model.RoleAdmin}
	if got := filterInboundsForUser(admin, inbounds); len(got) != 2 {
		t.Fatalf("admin saw %d inbounds, want 2", len(got))
	}
}

func TestKeepOwnedClientsInSettingsDropsForeignClients(t *testing.T) {
	settings := `{"clients":[{"email":"mine","id":"1"},{"email":"theirs","id":"2"}],"decryption":"none"}`
	got := keepOwnedClientsInSettings(settings, map[string]bool{"mine": true})
	if strings.Contains(got, "theirs") {
		t.Fatalf("foreign client leaked into seller settings: %s", got)
	}
	if !strings.Contains(got, `"mine"`) {
		t.Fatalf("owned client was dropped: %s", got)
	}
	if !strings.Contains(got, `"decryption":"none"`) {
		t.Fatalf("non-client settings must survive: %s", got)
	}
}

func TestKeepOwnedClientsInSettingsFailsClosedOnBadJSON(t *testing.T) {
	got := keepOwnedClientsInSettings(`{"clients":[`, map[string]bool{"mine": true})
	if got != `{"clients":[]}` {
		t.Fatalf("unparseable settings must become an empty client list, got %s", got)
	}
}

func TestScopeInboundClientsDropsForeignTraffic(t *testing.T) {
	ib := &model.Inbound{
		Settings:    `{"clients":[{"email":"mine"},{"email":"theirs"}]}`,
		ClientStats: []xray.ClientTraffic{{Email: "mine", Up: 1}, {Email: "theirs", Up: 2}},
	}
	scopeInboundClients(ib, map[string]bool{"mine": true})
	if len(ib.ClientStats) != 1 || ib.ClientStats[0].Email != "mine" {
		t.Fatalf("traffic rows = %+v, want only mine", ib.ClientStats)
	}
}

func TestSellerSelfServiceRoutes(t *testing.T) {
	cases := []struct {
		method, full string
		want         bool
	}{
		{http.MethodGet, "/panel/api/sellers/me", true},
		{http.MethodGet, "/panel/api/sellers/list", false},
		{http.MethodPost, "/panel/api/setting/updateUser", true},
		{http.MethodPost, "/panel/api/setting/updateSetting", false},
		{http.MethodGet, "/panel/api/setting/getAllSetting", false},
	}
	for _, c := range cases {
		if got := sellerRouteAllowed(c.method, c.full); got != c.want {
			t.Errorf("%s %s: got %v, want %v", c.method, c.full, got, c.want)
		}
	}
}

func TestWithoutServerSecretsDropsRealityAndTLSPrivateKeys(t *testing.T) {
	in := `{"network":"tcp","security":"reality",` +
		`"realitySettings":{"show":false,"privateKey":"SECRET","target":"x:443"},` +
		`"tlsSettings":{"certificates":[{"certificateFile":"/a.crt","keyFile":"/a.key","key":["PEM"]}]}}`
	want := `{"network":"tcp","realitySettings":{"show":false,"target":"x:443"},` +
		`"security":"reality","tlsSettings":{"certificates":[{"certificateFile":"/a.crt","keyFile":"/a.key"}]}}`
	if got := withoutServerSecrets(in); got != want {
		t.Errorf("withoutServerSecrets:\n got %s\nwant %s", got, want)
	}
	if got := withoutServerSecrets(""); got != "" {
		t.Errorf("empty stream settings changed to %q", got)
	}
}
