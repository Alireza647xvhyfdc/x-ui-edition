package model

import (
	"testing"
	"time"
)

func TestSellerExpiredBoundary(t *testing.T) {
	now := time.Unix(1_000, 0)
	seller := &User{Role: RoleSeller, SellerExpire: 1_000}
	if seller.SellerExpired(now) {
		t.Fatal("seller expiring exactly now must still be active")
	}
	if !seller.SellerExpired(now.Add(time.Second)) {
		t.Fatal("seller must be expired one second after its expiry")
	}
	if (&User{Role: RoleSeller}).SellerExpired(now) {
		t.Fatal("expiry 0 means never expires")
	}
	if (&User{Role: RoleAdmin, SellerExpire: 1}).SellerExpired(now) {
		t.Fatal("admin accounts never expire as sellers")
	}
}

func TestSellerTrafficExceededUsesGB(t *testing.T) {
	seller := &User{Role: RoleSeller, SellerTotalGB: 2}
	if seller.SellerTrafficExceeded(2*bytesPerGB - 1) {
		t.Fatal("just under the cap must not be exceeded")
	}
	if !seller.SellerTrafficExceeded(2 * bytesPerGB) {
		t.Fatal("reaching the cap must be exceeded")
	}
	if (&User{Role: RoleSeller}).SellerTrafficExceeded(1 << 60) {
		t.Fatal("cap 0 means unlimited")
	}
}

func TestCanUseInbound(t *testing.T) {
	admin := &User{Id: 1, Role: RoleAdmin}
	if !admin.CanUseInbound(999) {
		t.Fatal("admin must use every inbound")
	}
	legacy := &User{Id: 1}
	if !legacy.CanUseInbound(999) {
		t.Fatal("a role-less row is an admin and must use every inbound")
	}
	seller := &User{Id: 2, Role: RoleSeller, SellerInboundIds: []int{3, 5}}
	if !seller.CanUseInbound(5) {
		t.Fatal("seller must use a granted inbound")
	}
	if seller.CanUseInbound(4) {
		t.Fatal("seller must not use an inbound that was not granted")
	}
	if (&User{Role: RoleSeller}).CanUseInbound(1) {
		t.Fatal("seller with no grants must use nothing")
	}
	var none *User
	if none.CanUseInbound(1) {
		t.Fatal("nil user must use nothing")
	}
}

func TestInboundOwnerIdForSellerIsSellingAdmin(t *testing.T) {
	seller := &User{Id: 7, Role: RoleSeller, OwnerId: 1}
	if got := seller.InboundOwnerId(); got != 1 {
		t.Fatalf("seller inbound owner = %d, want 1", got)
	}
	admin := &User{Id: 1, Role: RoleAdmin}
	if got := admin.InboundOwnerId(); got != 1 {
		t.Fatalf("admin inbound owner = %d, want 1", got)
	}
}
