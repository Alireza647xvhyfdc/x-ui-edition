package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// SellerController lets an admin manage seller accounts. Sellers themselves are refused by the route guard.
type SellerController struct {
	sellerService service.SellerService
}

func NewSellerController(g *gin.RouterGroup) *SellerController {
	a := &SellerController{}
	a.initRouter(g)
	return a
}

func (a *SellerController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/me", a.me)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
}

func (a *SellerController) list(c *gin.Context) {
	rows, err := a.sellerService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

// me returns the signed-in seller's own limits and usage. Admins get 403, since they have no seller account.
func (a *SellerController) me(c *gin.Context) {
	u := session.GetLoginUser(c)
	if !u.IsSeller() {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	view, err := a.sellerService.Self(u)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *SellerController) add(c *gin.Context) {
	var in service.SellerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	owner := session.GetLoginUser(c)
	view, err := a.sellerService.Save(owner.Id, 0, in)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	enforceSellersNow()
	jsonObj(c, view, nil)
}

func (a *SellerController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var in service.SellerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	owner := session.GetLoginUser(c)
	view, err := a.sellerService.Save(owner.Id, id, in)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	enforceSellersNow()
	jsonObj(c, view, nil)
}

func (a *SellerController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := a.sellerService.Delete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	enforceSellersNow()
	jsonObj(c, nil, nil)
}

// enforceSellersNow applies a changed seller limit at once rather than at the next limit-job run.
func enforceSellersNow() {
	var clients service.ClientService
	var inbounds service.InboundService
	var xray service.XrayService
	needRestart, err := clients.EnforceSellerLimits(&inbounds, time.Now())
	if needRestart {
		xray.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("seller limit enforcement failed:", err)
	}
}
