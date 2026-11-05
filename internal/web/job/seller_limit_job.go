package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// SellerLimitJob disables the clients of sellers that are expired or over their traffic cap.
type SellerLimitJob struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewSellerLimitJob() *SellerLimitJob {
	return new(SellerLimitJob)
}

func (j *SellerLimitJob) Run() {
	needRestart, err := j.clientService.EnforceSellerLimits(&j.inboundService, time.Now())
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("seller limit enforcement failed:", err)
	}
}
