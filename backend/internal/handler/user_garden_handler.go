package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/service"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// UserGardenHandler exposes "my garden" endpoints.
type UserGardenHandler struct {
	svc    *service.UserGardenService
	logger *slog.Logger
}

// NewUserGardenHandler creates a UserGardenHandler.
func NewUserGardenHandler(svc *service.UserGardenService, logger *slog.Logger) *UserGardenHandler {
	return &UserGardenHandler{svc: svc, logger: logger}
}

// List handles GET /gardens.
func (h *UserGardenHandler) List(c *gin.Context) {
	items, err := h.svc.List(middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(items))
}

// Add handles POST /gardens.
func (h *UserGardenHandler) Add(c *gin.Context) {
	var req dto.GardenAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	g := &model.UserGarden{
		PlantSpeciesID: req.PlantSpeciesID, Nickname: req.Nickname,
		OwnedSince: req.OwnedSince, Location: req.Location,
	}
	created, err := h.svc.Add(middleware.GetUserID(c), g)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(created))
}

// Update handles PUT /gardens/:id.
func (h *UserGardenHandler) Update(c *gin.Context) {
	id, ok := parseGardenID(c)
	if !ok {
		return
	}
	var req dto.GardenUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	g, err := h.svc.Update(middleware.GetUserID(c), id, req.Nickname, req.Location)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(g))
}

// Repot handles POST /gardens/:id/repot.
func (h *UserGardenHandler) Repot(c *gin.Context) {
	id, ok := parseGardenID(c)
	if !ok {
		return
	}
	var req dto.GardenRepotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	view, err := h.svc.Repot(middleware.GetUserID(c), id, req.OwnedSince, req.Nickname, req.Location)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// BindReminder handles PUT /gardens/:id/reminder.
func (h *UserGardenHandler) BindReminder(c *gin.Context) {
	id, ok := parseGardenID(c)
	if !ok {
		return
	}
	var req dto.GardenBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	view, err := h.svc.BindReminder(middleware.GetUserID(c), id, req.ReminderID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// Remove handles DELETE /gardens/:id.
func (h *UserGardenHandler) Remove(c *gin.Context) {
	id, ok := parseGardenID(c)
	if !ok {
		return
	}
	if err := h.svc.Remove(middleware.GetUserID(c), id); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"removed": true}))
}

func parseGardenID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid garden id"))
		return 0, false
	}
	return uint(id), true
}
