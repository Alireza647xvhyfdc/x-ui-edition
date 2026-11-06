package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type sellerRoute struct {
	method string
	suffix string
}

// sellerAllowedRoutes is the whole surface a seller may call. Anything not
// listed is refused, so new admin routes are closed to sellers by default.
var sellerAllowedRoutes = []sellerRoute{
	{http.MethodGet, "/panel/"},
	{http.MethodGet, "/panel/inbounds"},
	{http.MethodGet, "/panel/csrf-token"},
	{http.MethodPost, "/logout"},
	{http.MethodGet, "/api/sellers/me"},
	{http.MethodPost, "/api/setting/updateUser"},
	{http.MethodGet, "/api/setting/apiTokens"},
	{http.MethodPost, "/api/setting/apiTokens/create"},
	{http.MethodPost, "/api/setting/apiTokens/delete/:id"},
	{http.MethodPost, "/api/setting/apiTokens/setEnabled/:id"},
	{http.MethodGet, "/api/inbounds/list"},
	{http.MethodGet, "/api/inbounds/list/slim"},
	{http.MethodGet, "/api/inbounds/get/:id"},
	{http.MethodGet, "/api/clients/list"},
	{http.MethodGet, "/api/clients/get/:email"},
	{http.MethodGet, "/api/clients/traffic/:email"},
	{http.MethodGet, "/api/clients/links/:email"},
	{http.MethodPost, "/api/clients/add"},
	{http.MethodPost, "/api/clients/update/:email"},
	{http.MethodPost, "/api/clients/del/:email"},
	{http.MethodPost, "/api/clients/bulkEnable"},
	{http.MethodPost, "/api/clients/bulkDisable"},
}

// sellerRouteAllowed matches on the registered route pattern (c.FullPath),
// so ids in the URL do not affect the decision.
func sellerRouteAllowed(method, fullPath string) bool {
	for _, r := range sellerAllowedRoutes {
		if r.method == method && strings.HasSuffix(fullPath, r.suffix) {
			return true
		}
	}
	return false
}

// enforceSellerAccess refuses a seller whose account has expired or whose
// request is outside sellerAllowedRoutes. Admins and anonymous callers pass through.
func enforceSellerAccess(c *gin.Context) {
	user := session.GetLoginUser(c)
	if !user.IsSeller() {
		c.Next()
		return
	}
	if user.SellerExpired(time.Now()) || !sellerRouteAllowed(c.Request.Method, c.FullPath()) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}

// filterInboundsForUser keeps the inbounds u may use. Admins get the full list.
func filterInboundsForUser(u *model.User, inbounds []*model.Inbound) []*model.Inbound {
	if !u.IsSeller() {
		return inbounds
	}
	out := make([]*model.Inbound, 0, len(inbounds))
	for _, ib := range inbounds {
		if u.CanUseInbound(ib.Id) {
			out = append(out, ib)
		}
	}
	return out
}

var (
	errSellerInboundNotGranted = errors.New("this inbound is not assigned to your seller account")
	errSellerEmailTaken        = errors.New("this client name belongs to another account")
	errSellerMaxClients        = errors.New("your seller account reached its client limit")
	errSellerLimited           = errors.New("your seller account has expired or reached its traffic limit")
)

// sellerBlocked reports whether a seller's expiry has passed or its clients
// have used the traffic cap.
func sellerBlocked(u *model.User, cs *service.ClientService) (bool, error) {
	if u.SellerExpired(time.Now()) {
		return true, nil
	}
	owned, err := cs.SellerOwnedEmails(u.Id)
	if err != nil {
		return false, err
	}
	used, err := cs.SellerUsedBytes(owned)
	if err != nil {
		return false, err
	}
	return u.SellerTrafficExceeded(used), nil
}

// sellerCanAccess reports whether the request's user may act on every given
// client: it must own the client and hold every inbound the client is on.
// Admins always may; a refusal is written to c and reported as false.
func sellerCanAccess(c *gin.Context, cs *service.ClientService, emails ...string) bool {
	u := session.GetLoginUser(c)
	if !u.IsSeller() {
		return true
	}
	for _, email := range emails {
		owned, err := cs.ClientOwnedBy(email, u.Id)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return false
		}
		if !owned {
			c.AbortWithStatus(http.StatusForbidden)
			return false
		}
		granted, err := sellerHoldsClientInbounds(u, cs, email)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return false
		}
		if !granted {
			c.AbortWithStatus(http.StatusForbidden)
			return false
		}
	}
	return true
}

// sellerHoldsClientInbounds reports whether every inbound the client is on is
// still granted to the seller. An admin who revokes an inbound thereby also
// revokes access to the links and credentials of clients on it.
func sellerHoldsClientInbounds(u *model.User, cs *service.ClientService, email string) (bool, error) {
	rec, err := cs.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	ids, err := cs.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if !u.CanUseInbound(id) {
			return false, nil
		}
	}
	return true, nil
}

// sellerCreateCheck applies a seller's inbound grants, limits and name ownership before a client is created.
func sellerCreateCheck(u *model.User, cs *service.ClientService, payload *service.ClientCreatePayload) error {
	for _, id := range payload.InboundIds {
		if !u.CanUseInbound(id) {
			return errSellerInboundNotGranted
		}
	}
	blocked, err := sellerBlocked(u, cs)
	if err != nil {
		return err
	}
	if blocked {
		return errSellerLimited
	}
	rec, err := cs.GetRecordByEmail(nil, payload.Client.Email)
	switch {
	case err == nil:
		if rec.OwnerId != u.Id {
			return errSellerEmailTaken
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		if u.SellerMaxClients > 0 {
			owned, err := cs.SellerOwnedEmails(u.Id)
			if err != nil {
				return err
			}
			if len(owned) >= u.SellerMaxClients {
				return errSellerMaxClients
			}
		}
	default:
		return err
	}
	return nil
}

// clientsMayEnable refuses to switch on a client whose seller is expired or over
// its traffic cap, whoever asks; the limit job would switch it off again.
func clientsMayEnable(c *gin.Context, cs *service.ClientService, emails []string, enable bool) bool {
	if !enable {
		return true
	}
	for _, email := range emails {
		rec, err := cs.GetRecordByEmail(nil, email)
		if err != nil || rec.OwnerId == 0 {
			continue
		}
		seller, err := (&service.SellerService{}).Find(rec.OwnerId)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return false
		}
		blocked, err := sellerBlocked(seller, cs)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return false
		}
		if blocked {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), errSellerLimited)
			return false
		}
	}
	return true
}

// sellerOwnedRows keeps the client rows that the given seller created.
func sellerOwnedRows(rows []service.ClientWithAttachments, sellerId int) []service.ClientWithAttachments {
	out := make([]service.ClientWithAttachments, 0, len(rows))
	for _, r := range rows {
		if r.OwnerId == sellerId {
			out = append(out, r)
		}
	}
	return out
}

// scopeInboundsForSeller hides other sellers' clients from a seller's view of
// inbounds. Admins get the inbounds unchanged.
func scopeInboundsForSeller(u *model.User, cs *service.ClientService, inbounds []*model.Inbound) error {
	if !u.IsSeller() {
		return nil
	}
	emails, err := cs.SellerOwnedEmails(u.Id)
	if err != nil {
		return err
	}
	owned := make(map[string]bool, len(emails))
	for _, e := range emails {
		owned[e] = true
	}
	for _, ib := range inbounds {
		scopeInboundClients(ib, owned)
	}
	return nil
}

func scopeInboundClients(ib *model.Inbound, owned map[string]bool) {
	ib.Settings = keepOwnedClientsInSettings(ib.Settings, owned)
	ib.StreamSettings = withoutServerSecrets(ib.StreamSettings)
	kept := ib.ClientStats[:0]
	for _, st := range ib.ClientStats {
		if owned[st.Email] {
			kept = append(kept, st)
		}
	}
	ib.ClientStats = kept
}

// withoutServerSecrets removes the Reality private key and inline TLS private
// keys from stream settings. No share link carries them, and a seller needs none.
func withoutServerSecrets(stream string) string {
	if stream == "" {
		return stream
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stream), &doc); err != nil {
		return "{}"
	}
	if reality, ok := doc["realitySettings"].(map[string]any); ok {
		delete(reality, "privateKey")
	}
	if tls, ok := doc["tlsSettings"].(map[string]any); ok {
		if certs, ok := tls["certificates"].([]any); ok {
			for _, cert := range certs {
				if m, ok := cert.(map[string]any); ok {
					delete(m, "key")
				}
			}
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return "{}"
	}
	return string(out)
}

// keepOwnedClientsInSettings drops settings.clients entries that the seller does
// not own. Unparseable settings become an empty client list, never the original.
func keepOwnedClientsInSettings(settings string, owned map[string]bool) string {
	const empty = `{"clients":[]}`
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(settings), &doc); err != nil {
		return empty
	}
	var clients []map[string]json.RawMessage
	if raw, ok := doc["clients"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &clients); err != nil {
			return empty
		}
	}
	kept := make([]map[string]json.RawMessage, 0, len(clients))
	for _, cl := range clients {
		var email string
		if json.Unmarshal(cl["email"], &email) == nil && owned[email] {
			kept = append(kept, cl)
		}
	}
	raw, err := json.Marshal(kept)
	if err != nil {
		return empty
	}
	doc["clients"] = raw
	out, err := json.Marshal(doc)
	if err != nil {
		return empty
	}
	return string(out)
}

// capSellerTokenForm pins a seller's new token to the seller scope and never past the seller's expiry.
func capSellerTokenForm(u *model.User, form *apiTokenCreateForm) {
	form.Scope = model.ApiScopeAdmin
	if u.SellerExpire > 0 {
		limit := u.SellerExpire * 1000
		if form.ExpiresAt == 0 || form.ExpiresAt > limit {
			form.ExpiresAt = limit
		}
	}
}

// sellerOwnsToken lets a seller act only on its own tokens. Admins pass; a refusal is written to c.
func sellerOwnsToken(c *gin.Context, tokens panel.ApiTokenService, id int) bool {
	u := session.GetLoginUser(c)
	if !u.IsSeller() {
		return true
	}
	owned, err := tokens.IsOwnedBy(id, u.Id)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return false
	}
	if !owned {
		c.AbortWithStatus(http.StatusForbidden)
		return false
	}
	return true
}
