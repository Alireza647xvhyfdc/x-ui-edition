package model

import "time"

// Account roles. An empty role (rows that predate sellers) counts as admin.
const (
	RoleAdmin  = "admin"
	RoleSeller = "seller"
)

const bytesPerGB = int64(1) << 30

// IsSeller reports whether u is a seller account. A nil user is not a seller.
func (u *User) IsSeller() bool {
	return u != nil && u.Role == RoleSeller
}

// InboundOwnerId is the account whose inbounds u works on: its own id for an
// admin, the selling admin's id for a seller.
func (u *User) InboundOwnerId() int {
	if u.IsSeller() {
		return u.OwnerId
	}
	return u.Id
}

// SellerExpired reports whether a seller's expiry has passed at now.
func (u *User) SellerExpired(now time.Time) bool {
	return u.IsSeller() && u.SellerExpire > 0 && now.Unix() > u.SellerExpire
}

// SellerTrafficExceeded reports whether usedBytes has reached the seller's GB cap.
func (u *User) SellerTrafficExceeded(usedBytes int64) bool {
	return u.IsSeller() && u.SellerTotalGB > 0 && usedBytes >= u.SellerTotalGB*bytesPerGB
}

// CanUseInbound reports whether u may read or sell the inbound. Admins may use
// every inbound; a seller only those listed in SellerInboundIds.
func (u *User) CanUseInbound(inboundId int) bool {
	if u == nil {
		return false
	}
	if !u.IsSeller() {
		return true
	}
	for _, id := range u.SellerInboundIds {
		if id == inboundId {
			return true
		}
	}
	return false
}
