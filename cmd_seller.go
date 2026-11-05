package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"

	"gorm.io/gorm"
)

// runSellerCmd handles `x-ui seller set|list|del`. Sellers sell from the first
// admin's inbounds, so grants are checked against that account's inbounds.
func runSellerCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: x-ui seller set|list|del")
	}
	if err := database.InitDB(config.GetDBPath()); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	switch args[0] {
	case "set":
		return sellerSet(args[1:])
	case "list":
		return sellerList()
	case "del":
		return sellerDel(args[1:])
	}
	return fmt.Errorf("unknown seller command %q", args[0])
}

func sellerSet(args []string) error {
	fs := flag.NewFlagSet("seller set", flag.ContinueOnError)
	username := fs.String("username", "", "seller login name")
	password := fs.String("password", "", "seller password")
	expire := fs.String("expire", "", "expiry date YYYY-MM-DD (end of day, local time); empty keeps it, 0 clears it")
	totalGB := fs.Int64("total-gb", -1, "traffic cap in GB for the seller's clients; 0 means unlimited")
	maxClients := fs.Int("max-clients", -1, "client cap; 0 means unlimited")
	inbounds := fs.String("inbounds", "", "comma-separated inbound ids the seller may use")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	if len(*username) > panel.MaxUsernameLength {
		return fmt.Errorf("username can not be longer than %d bytes", panel.MaxUsernameLength)
	}

	db := database.GetDB()
	owner, err := (&panel.UserService{}).GetFirstUser()
	if err != nil {
		return fmt.Errorf("no admin account to sell from: %w", err)
	}
	if owner.IsSeller() {
		return errors.New("the first account must be an admin")
	}

	var seller model.User
	err = db.Where("username = ?", *username).First(&seller).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}
	if isNew {
		if *password == "" {
			return errors.New("-password is required for a new seller")
		}
		seller = model.User{Username: *username, Role: model.RoleSeller, OwnerId: owner.Id}
	} else if !seller.IsSeller() {
		return fmt.Errorf("account %q is an admin, not a seller", *username)
	}

	if *password != "" {
		hash, err := crypto.HashPasswordAsBcrypt(*password)
		if err != nil {
			return err
		}
		seller.Password = hash
		seller.LoginEpoch++
	}
	if *expire != "" {
		if seller.SellerExpire, err = parseSellerExpire(*expire); err != nil {
			return err
		}
	}
	if *totalGB >= 0 {
		seller.SellerTotalGB = *totalGB
	}
	if *maxClients >= 0 {
		seller.SellerMaxClients = *maxClients
	}
	if *inbounds != "" {
		ids, err := parseInboundIds(*inbounds)
		if err != nil {
			return err
		}
		var owned int64
		if err := db.Model(&model.Inbound{}).Where("user_id = ? AND id IN ?", owner.Id, ids).Count(&owned).Error; err != nil {
			return err
		}
		if int(owned) != len(ids) {
			return errors.New("some inbound ids are not inbounds of the selling admin")
		}
		if !isNew {
			if err := warnRevokedAttachments(db, seller.Id, seller.SellerInboundIds, ids); err != nil {
				return err
			}
		}
		seller.SellerInboundIds = ids
	}

	if isNew {
		err = db.Create(&seller).Error
	} else {
		err = db.Save(&seller).Error
	}
	if err != nil {
		return err
	}
	fmt.Printf("seller %q saved (id %d), inbounds %v\n", seller.Username, seller.Id, seller.SellerInboundIds)
	return nil
}

func sellerList() error {
	var sellers []model.User
	if err := database.GetDB().Where("role = ?", model.RoleSeller).Order("id ASC").Find(&sellers).Error; err != nil {
		return err
	}
	for _, s := range sellers {
		expire := "never"
		if s.SellerExpire > 0 {
			expire = time.Unix(s.SellerExpire, 0).Format("2006-01-02")
		}
		fmt.Printf("id=%d user=%s expire=%s total_gb=%d max_clients=%d inbounds=%v\n",
			s.Id, s.Username, expire, s.SellerTotalGB, s.SellerMaxClients, s.SellerInboundIds)
	}
	return nil
}

func sellerDel(args []string) error {
	fs := flag.NewFlagSet("seller del", flag.ContinueOnError)
	username := fs.String("username", "", "seller login name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-username is required")
	}
	db := database.GetDB()
	var seller model.User
	if err := db.Where("username = ? AND role = ?", *username, model.RoleSeller).First(&seller).Error; err != nil {
		return fmt.Errorf("no seller named %q", *username)
	}
	if err := (&service.SellerService{}).Delete(seller.Id); err != nil {
		return err
	}
	fmt.Printf("seller %q deleted; its clients are disabled and no longer belong to any seller\n", *username)
	return nil
}

func parseSellerExpire(s string) (int64, error) {
	if s == "0" {
		return 0, nil
	}
	day, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return 0, fmt.Errorf("expiry must be YYYY-MM-DD or 0: %w", err)
	}
	return day.Add(24*time.Hour - time.Second).Unix(), nil
}

func parseInboundIds(s string) ([]int, error) {
	var ids []int
	for _, part := range strings.Split(s, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid inbound id %q", part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// warnRevokedAttachments reports the seller's clients still attached to inbounds
// it lost. Removing those attachments is left to the admin, who can see the clients.
func warnRevokedAttachments(db *gorm.DB, sellerId int, before, after []int) error {
	keep := make(map[int]bool, len(after))
	for _, id := range after {
		keep[id] = true
	}
	var revoked []int
	for _, id := range before {
		if !keep[id] {
			revoked = append(revoked, id)
		}
	}
	if len(revoked) == 0 {
		return nil
	}
	var count int64
	err := db.Model(&model.ClientInbound{}).
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.owner_id = ? AND client_inbounds.inbound_id IN ?", sellerId, revoked).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		fmt.Printf("warning: %d client attachment(s) of this seller remain on inbounds it lost; detach them in the panel\n", count)
	}
	return nil
}
