package server

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

// entityNames unmarshals a JSON:API list payload of elemType and returns each element's display
// name via getName, in response order.
func entityNames(t *testing.T, body []byte, elemType reflect.Type, getName func(any) string) []string {
	t.Helper()
	raw, err := jsonapi.UnmarshalManyPayload(bytes.NewReader(body), elemType)
	if err != nil {
		t.Fatalf("unmarshal list payload: %v (body: %s)", err, body)
	}
	names := make([]string, 0, len(raw))
	for _, r := range raw {
		names = append(names, getName(r))
	}
	return names
}

func TestListPublishersNameContainsFilter(t *testing.T) {
	hit := seedPublisher(t, "Zorbex Qppub Publishing")
	seedPublisher(t, "Unrelated House")

	r := newRelationshipTestRouter(t, setupPublisherHandlers)
	rec := doGet(t, r, "/publishers?filter[name][contains]=qppub")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	names := entityNames(t, rec.Body.Bytes(), reflect.TypeOf(new(vo.PublisherVO)), func(x any) string {
		return x.(*vo.PublisherVO).Name
	})
	if want := []string{"Zorbex Qppub Publishing"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v (id %s)", names, want, hit)
	}
}

func TestListStudiosNameContainsFilter(t *testing.T) {
	seedStudio(t, "Qpstu Marker Studio")
	seedStudio(t, "Ordinary Studio")

	r := newRelationshipTestRouter(t, setupStudioHandlers)
	rec := doGet(t, r, "/studios?filter[name][contains]=qpstu")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	names := entityNames(t, rec.Body.Bytes(), reflect.TypeOf(new(vo.StudioVO)), func(x any) string {
		return x.(*vo.StudioVO).Name
	})
	if want := []string{"Qpstu Marker Studio"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestListPersonsNameContainsFilter(t *testing.T) {
	seedPerson(t, "Qpper Marker Person")
	seedPerson(t, "Someone Else")

	r := newRelationshipTestRouter(t, setupPersonHandlers)
	rec := doGet(t, r, "/persons?filter[name][contains]=qpper")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	names := entityNames(t, rec.Body.Bytes(), reflect.TypeOf(new(vo.PersonVO)), func(x any) string {
		return x.(*vo.PersonVO).Name
	})
	if want := []string{"Qpper Marker Person"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestListLicensesTitleContainsFilter(t *testing.T) {
	if _, err := data.AddLicense(t.Context(), &vo.LicenseVO{Title: "Qplic Marker License"}); err != nil {
		t.Fatalf("seed license: %v", err)
	}
	if _, err := data.AddLicense(t.Context(), &vo.LicenseVO{Title: "Ordinary License"}); err != nil {
		t.Fatalf("seed license: %v", err)
	}

	r := newRelationshipTestRouter(t, setupLicenseHandlers)
	rec := doGet(t, r, "/licenses?filter[title][contains]=qplic")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	names := entityNames(t, rec.Body.Bytes(), reflect.TypeOf(new(vo.LicenseVO)), func(x any) string {
		return x.(*vo.LicenseVO).Title
	})
	if want := []string{"Qplic Marker License"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestListPublishersPageLimitCapsResults(t *testing.T) {
	for _, n := range []string{"Qppage One", "Qppage Two", "Qppage Three"} {
		seedPublisher(t, n)
	}

	r := newRelationshipTestRouter(t, setupPublisherHandlers)
	rec := doGet(t, r, "/publishers?filter[name][contains]=qppage&page[start]=0&page[limit]=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	names := entityNames(t, rec.Body.Bytes(), reflect.TypeOf(new(vo.PublisherVO)), func(x any) string {
		return x.(*vo.PublisherVO).Name
	})
	if len(names) != 2 {
		t.Fatalf("page[limit]=2 returned %d results: %v", len(names), names)
	}
}

func TestListPublishersRejectsUnknownFilterOperator(t *testing.T) {
	r := newRelationshipTestRouter(t, setupPublisherHandlers)
	rec := doGet(t, r, "/publishers?filter[name][bogus]=x")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown filter operator), body = %s", rec.Code, rec.Body.String())
	}
}
