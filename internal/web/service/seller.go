package service

import (
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"

	"gorm.io/gorm"
)

// maxSellerUsername matches the panel's account name limit.
const maxSellerUsername = 256

// SellerInput is the admin's form for one seller. Limits of 0 mean unlimited.
type SellerInput struct {
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	Expire     int64  `json:"expire"`
	TotalGB    int64  `json:"totalGB"`
	MaxClients int    `json:"maxClients"`
	InboundIds []int  `json:"inboundIds"`
}

// SellerView is one seller as the admin sees it. It never carries the password hash.
type SellerView struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	Expire      int64  `json:"expire"`
	TotalGB     int64  `json:"totalGB"`
	MaxClients  int    `json:"maxClients"`
	InboundIds  []int  `json:"inboundIds"`
	ClientCount int    `json:"clientCount"`
	UsedBytes   int64  `json:"usedBytes"`
	Blocked     bool   `json:"blocked"`
}

var (
	errSellerNameRequired     = errors.New("username is required")
	errSellerNameTaken        = errors.New("username already exists")
	errSellerPasswordRequired = errors.New("password is required for a new seller")
	errSellerLimitsInvalid    = errors.New("expiry, traffic and client limits must not be negative")
	errSellerNotFound         = errors.New("seller not found")
	errSellerInboundForeign   = errors.New("some inbound ids are not inbounds of this admin")
)

// SellerService manages seller accounts on behalf of the admin who created them.
type SellerService struct {
	clients ClientService
}

// List returns every seller with its client count and traffic use.
func (s *SellerService) List() ([]SellerView, error) {
	var sellers []model.User
	if err := database.GetDB().Where("role = ?", model.RoleSeller).Order("id ASC").Find(&sellers).Error; err != nil {
		return nil, err
	}
	out := make([]SellerView, 0, len(sellers))
	for i := range sellers {
		view, err := s.view(&sellers[i])
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

// Save creates the seller when id is 0, otherwise updates that seller. ownerId is the admin.
func (s *SellerService) Save(ownerId, id int, in SellerInput) (SellerView, error) {
	db := database.GetDB()
	name := strings.TrimSpace(in.Username)
	if name == "" || len(name) > maxSellerUsername {
		return SellerView{}, errSellerNameRequired
	}
	if in.Expire < 0 || in.TotalGB < 0 || in.MaxClients < 0 {
		return SellerView{}, errSellerLimitsInvalid
	}
	seller := model.User{Role: model.RoleSeller, OwnerId: ownerId}
	if id != 0 {
		if err := db.Where("id = ? AND role = ?", id, model.RoleSeller).First(&seller).Error; err != nil {
			return SellerView{}, errSellerNotFound
		}
	} else if in.Password == "" {
		return SellerView{}, errSellerPasswordRequired
	}

	var taken int64
	if err := db.Model(&model.User{}).Where("username = ? AND id <> ?", name, seller.Id).Count(&taken).Error; err != nil {
		return SellerView{}, err
	}
	if taken > 0 {
		return SellerView{}, errSellerNameTaken
	}

	ids := uniqueInts(in.InboundIds)
	if len(ids) > 0 {
		var owned int64
		if err := db.Model(&model.Inbound{}).Where("user_id = ? AND id IN ?", ownerId, ids).Count(&owned).Error; err != nil {
			return SellerView{}, err
		}
		if int(owned) != len(ids) {
			return SellerView{}, errSellerInboundForeign
		}
	}

	seller.Username = name
	if in.Password != "" {
		hash, err := crypto.HashPasswordAsBcrypt(in.Password)
		if err != nil {
			return SellerView{}, err
		}
		seller.Password = hash
		seller.LoginEpoch++
	}
	seller.SellerExpire = in.Expire
	seller.SellerTotalGB = in.TotalGB
	seller.SellerMaxClients = in.MaxClients
	seller.SellerInboundIds = ids
	if seller.Id == 0 {
		if err := db.Create(&seller).Error; err != nil {
			return SellerView{}, err
		}
	} else if err := db.Save(&seller).Error; err != nil {
		return SellerView{}, err
	}
	return s.view(&seller)
}

// Find returns the seller account with this id.
func (s *SellerService) Find(id int) (*model.User, error) {
	var seller model.User
	if err := database.GetDB().Where("id = ? AND role = ?", id, model.RoleSeller).First(&seller).Error; err != nil {
		return nil, errSellerNotFound
	}
	return &seller, nil
}

// Self returns the view of the seller account that is signed in.
func (s *SellerService) Self(u *model.User) (SellerView, error) {
	return s.view(u)
}

// Delete removes a seller and cuts off its clients: they are disabled on every
// inbound and stay unowned, so only an admin can turn them back on. If the cut
// fails, nothing is deleted.
func (s *SellerService) Delete(id int) error {
	seller, err := s.Find(id)
	if err != nil {
		return err
	}
	emails, err := s.clients.SellerOwnedEmails(seller.Id)
	if err != nil {
		return err
	}
	if len(emails) > 0 {
		var inbounds InboundService
		var xray XrayService
		_, needRestart, err := s.clients.BulkSetEnable(&inbounds, emails, false)
		if err != nil {
			return err
		}
		if needRestart {
			xray.SetToNeedRestart()
		}
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ClientRecord{}).Where("owner_id = ?", seller.Id).
			Updates(map[string]any{"owner_id": 0, "seller_disabled": false}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ?", seller.Id).Delete(&model.ApiToken{}).Error; err != nil {
			return err
		}
		return tx.Delete(seller).Error
	})
}

func (s *SellerService) view(u *model.User) (SellerView, error) {
	owned, err := s.clients.SellerOwnedEmails(u.Id)
	if err != nil {
		return SellerView{}, err
	}
	used, err := s.clients.SellerUsedBytes(owned)
	if err != nil {
		return SellerView{}, err
	}
	ids := u.SellerInboundIds
	if ids == nil {
		ids = []int{}
	}
	return SellerView{
		Id:          u.Id,
		Username:    u.Username,
		Expire:      u.SellerExpire,
		TotalGB:     u.SellerTotalGB,
		MaxClients:  u.SellerMaxClients,
		InboundIds:  ids,
		ClientCount: len(owned),
		UsedBytes:   used,
		Blocked:     u.SellerExpired(time.Now()) || u.SellerTrafficExceeded(used),
	}, nil
}
