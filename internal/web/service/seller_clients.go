package service

import (
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// SellerOwnedEmails returns the emails of every client the seller created.
func (s *ClientService) SellerOwnedEmails(sellerId int) ([]string, error) {
	return sellerEmailsWhere(sellerId, "1 = 1")
}

// ClientOwnedBy reports whether the client with this email belongs to ownerId.
// A missing client is not owned by anyone.
func (s *ClientService) ClientOwnedBy(email string, ownerId int) (bool, error) {
	var count int64
	err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ? AND owner_id = ?", email, ownerId).Count(&count).Error
	return count > 0, err
}

// SellerUsedBytes sums the up and down counters of the given client emails.
func (s *ClientService) SellerUsedBytes(emails []string) (int64, error) {
	var total int64
	db := database.GetDB()
	for start := 0; start < len(emails); start += sqlInChunk {
		end := min(start+sqlInChunk, len(emails))
		var part int64
		err := db.Model(&xray.ClientTraffic{}).
			Select("COALESCE(SUM(up + down), 0)").
			Where("email IN ?", emails[start:end]).
			Scan(&part).Error
		if err != nil {
			return 0, err
		}
		total += part
	}
	return total, nil
}

// SetClientOwner records the seller that owns each client; ownerId 0 clears it.
func (s *ClientService) SetClientOwner(emails []string, ownerId int) error {
	return updateClientRecords(emails, map[string]any{"owner_id": ownerId})
}

// SetSellerDisabled marks clients as disabled (or not) by the seller limit job.
func (s *ClientService) SetSellerDisabled(emails []string, disabled bool) error {
	return updateClientRecords(emails, map[string]any{"seller_disabled": disabled})
}

func updateClientRecords(emails []string, values map[string]any) error {
	db := database.GetDB()
	for start := 0; start < len(emails); start += sqlInChunk {
		end := min(start+sqlInChunk, len(emails))
		if err := db.Model(&model.ClientRecord{}).
			Where("email IN ?", emails[start:end]).
			Updates(values).Error; err != nil {
			return err
		}
	}
	return nil
}

// EnforceSellerLimits disables the clients of expired or over-cap sellers and
// re-enables those it disabled once the seller recovers. A failing seller does
// not stop the others. It reports whether Xray needs a restart.
func (s *ClientService) EnforceSellerLimits(inboundSvc *InboundService, now time.Time) (bool, error) {
	var sellers []model.User
	if err := database.GetDB().Where("role = ?", model.RoleSeller).Find(&sellers).Error; err != nil {
		return false, err
	}
	needRestart := false
	var errs []error
	for i := range sellers {
		restart, err := s.enforceSeller(inboundSvc, &sellers[i], now)
		needRestart = needRestart || restart
		if err != nil {
			errs = append(errs, err)
		}
	}
	return needRestart, errors.Join(errs...)
}

// enforceSeller applies one seller's expiry and traffic cap to its clients.
func (s *ClientService) enforceSeller(inboundSvc *InboundService, seller *model.User, now time.Time) (bool, error) {
	emails, err := s.SellerOwnedEmails(seller.Id)
	if err != nil {
		return false, err
	}
	used, err := s.SellerUsedBytes(emails)
	if err != nil {
		return false, err
	}
	if seller.SellerExpired(now) || seller.SellerTrafficExceeded(used) {
		disable, err := sellerEmailsWhere(seller.Id, "enable = ?", true)
		if err != nil || len(disable) == 0 {
			return false, err
		}
		_, restart, err := s.BulkSetEnable(inboundSvc, disable, false)
		if err != nil {
			return restart, err
		}
		return restart, s.SetSellerDisabled(disable, true)
	}
	restore, err := sellerEmailsWhere(seller.Id, "seller_disabled = ?", true)
	if err != nil || len(restore) == 0 {
		return false, err
	}
	_, restart, err := s.BulkSetEnable(inboundSvc, restore, true)
	if err != nil {
		return restart, err
	}
	return restart, s.SetSellerDisabled(restore, false)
}

func sellerEmailsWhere(sellerId int, cond string, args ...any) ([]string, error) {
	var emails []string
	err := database.GetDB().Model(&model.ClientRecord{}).
		Where("owner_id = ?", sellerId).
		Where(cond, args...).
		Order("id ASC").Pluck("email", &emails).Error
	return emails, err
}
