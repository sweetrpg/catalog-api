package grifts

import (
	"sweetrpg.com/catalog-api/actions"

	"github.com/gobuffalo/buffalo"
)

func init() {
	buffalo.Grifts(actions.App())
}
