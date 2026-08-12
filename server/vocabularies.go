package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/vocabularies"
	"github.com/sweetrpg/common.go/logging"
)

func setupVocabularyHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up vocabulary endpoint handlers...")

	readRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.GET("/vocabularies/:type", readRoles, listVocabulary)

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.POST("/vocabularies/:type", writeRoles, addVocabularyValue)
}

type vocabularyResponse struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
}

type addVocabularyRequest struct {
	Value string `json:"value"`
}

// List a vocabulary's values.
//
//	@Summary		List a vocabulary's values
//	@Description	Lists every value in the given shared vocabulary (contribution-type, property-name, or format). Format is editor/admin only.
//	@Tags			vocabularies
//	@Produce		json
//	@Param			type	path		string	true	"Vocabulary type"
//	@Success		200		{object}	vocabularyResponse
//	@Failure		403		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Router			/vocabularies/{type} [get]
func listVocabulary(c *gin.Context) {
	vocabType := c.Param("type")
	if !vocabularies.IsValidType(vocabType) {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "unknown_vocabulary", Message: "Unrecognized vocabulary type"})
		return
	}

	// format goes further than the other vocabulary types: the whole field is editor/admin
	// only, not just growing it (see volume-format-selector's spec).
	if vocabType == vocabularies.TypeFormat {
		roles := authz.Roles(c)
		if !authz.HasRole(roles, authz.RoleAdmin) && !authz.HasRole(roles, authz.RoleEditor) {
			c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "Caller does not have a qualifying role"})
			return
		}
	}

	values, err := vocabularies.List(c.Request.Context(), vocabType)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, vocabularyResponse{Type: vocabType, Values: values})
}

// Add a vocabulary value.
//
//	@Summary		Add a value to a vocabulary
//	@Description	Adds a new value to the given shared vocabulary. Editor/admin only for every type.
//	@Tags			vocabularies
//	@Accept			json
//	@Produce		json
//	@Param			type	path		string					true	"Vocabulary type"
//	@Param			request	body		addVocabularyRequest	true	"Value to add"
//	@Success		201		{object}	vocabularyResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Router			/vocabularies/{type} [post]
func addVocabularyValue(c *gin.Context) {
	vocabType := c.Param("type")
	if !vocabularies.IsValidType(vocabType) {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "unknown_vocabulary", Message: "Unrecognized vocabulary type"})
		return
	}

	var req addVocabularyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Value == "" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "value is required"})
		return
	}

	if _, err := vocabularies.Add(c.Request.Context(), vocabType, req.Value); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "add_failed", Message: err.Error()})
		return
	}

	values, err := vocabularies.List(c.Request.Context(), vocabType)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, vocabularyResponse{Type: vocabType, Values: values})
}
