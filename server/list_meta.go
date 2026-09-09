package server

import (
	"encoding/json"
	"io"

	"github.com/google/jsonapi"
)

// marshalListWithTotal writes models as a JSON:API many-payload carrying a
// top-level meta.total. total is the count of records matching the request's
// filter, computed at the query layer (CountDocuments) rather than by fetching
// the full collection - so a browse page can rebuild its pager from one page
// without its cost scaling with total catalog size.
func marshalListWithTotal(w io.Writer, models interface{}, total int64) error {
	payload, err := jsonapi.Marshal(models)
	if err != nil {
		return err
	}
	if many, ok := payload.(*jsonapi.ManyPayload); ok {
		meta := jsonapi.Meta{"total": total}
		many.Meta = &meta
	}
	return json.NewEncoder(w).Encode(payload)
}
