package api

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prmsrswt/gonotify/pkg/api/models"
)

// Group represents a group of nodes
type Group struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	UserID        int            `json:"userID"`
	WhatsAppNodes []WhatsAppNode `json:"whatsappNodes"`
}

func (api *API) queryGroups(c *gin.Context) {
	logger := log.With(api.logger, "route", "groups")

	uID := int(c.MustGet("id").(float64))

	wNodes := map[int][]WhatsAppNode{}

	wRows, err := api.DB.Query(
		`SELECT
			whatsappNodes.id,
			whatsappNodes.groupID,
			whatsappNodes.numberID,
			numbers.phone,
			whatsappNodes.lastMsgReceived
		FROM whatsappNodes
		LEFT JOIN numbers ON whatsappNodes.numberID = numbers.id
		WHERE numbers.userID = ?`,
		uID,
	)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "some error occured"})
		level.Error(logger).Log("err", err)
		return
	}
	defer wRows.Close()

	for wRows.Next() {
		var n WhatsAppNode

		err = wRows.Scan(&n.ID, &n.GroupID, &n.NumberID, &n.Phone, &n.LastMsgReceived)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "some error occured"})
			level.Error(logger).Log("err", err)
			return
		}

		wNodes[n.GroupID] = append(wNodes[n.GroupID], n)
	}

	groups := []Group{}

	rows, err := api.DB.Query(
		`SELECT id, userID, name FROM groups WHERE userID = ?`,
		uID,
	)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "some error occured"})
		level.Error(logger).Log("err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var g Group
		err = rows.Scan(&g.ID, &g.UserID, &g.Name)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "some error occured"})
			level.Error(logger).Log("err", err)
			return
		}

		g.WhatsAppNodes = wNodes[g.ID]
		if g.WhatsAppNodes == nil {
			g.WhatsAppNodes = []WhatsAppNode{}
		}

		groups = append(groups, g)
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
	})
}

func (api *API) handleAddGroup(c *gin.Context) {
	l := log.With(api.logger, "route", "addGroup")

	type input struct {
		Name string `json:"name" binding:"required,alpha"`
	}

	var i input
	uID := int(c.MustGet("id").(float64))

	if err := c.ShouldBind(&i); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "input validation failed",
		})
		return
	}

	i.Name = strings.ToLower(i.Name)

	gp := models.Group{Name: i.Name, UserID: uID}

	err := gp.GetByNameUserID(api.DB)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "group already exists",
		})
		return
	}
	if err != sql.ErrNoRows {
		throwInternalError(c, l, err)
		return
	}

	_, err = models.WithDB(api.DB, gp.New)
	if err != nil {
		throwInternalError(c, l, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "group created successfully",
	})
}

func (api *API) handleRemoveGroup(c *gin.Context) {
	l := log.With(api.logger, "route", "removeGroup")

	type input struct {
		ID int `json:"id" binding:"required"`
	}

	var i input
	uID := int(c.MustGet("id").(float64))

	if err := c.ShouldBind(&i); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "all fields are required",
		})
		return
	}

	group := models.Group{ID: i.ID}
	err := group.GetByID(api.DB)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "given group doesn't exist",
			})
			return
		}
		throwInternalError(c, l, err)
		return
	}

	if group.UserID != uID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "not authorized",
		})
		return
	}

	if group.Name == "default" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cannot delete default group",
		})
		return
	}

	tx, err := api.DB.Begin()
	if err != nil {
		throwInternalError(c, l, err)
		return
	}
	defer tx.Rollback()

	_, err = group.DeleteByID(tx)
	if err != nil {
		throwInternalError(c, l, err)
		return
	}

	_, err = tx.Exec(
		`DELETE FROM whatsappNodes WHERE groupID = ?`,
		i.ID,
	)
	if err != nil {
		throwInternalError(c, l, err)
		return
	}

	_, err = tx.Exec(
		`DELETE FROM notifications WHERE groupID = ?`,
		i.ID,
	)
	if err != nil {
		throwInternalError(c, l, err)
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": "group successfully deleted",
	})
}
