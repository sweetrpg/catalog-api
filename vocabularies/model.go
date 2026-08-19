// Package vocabularies stores the shared, growable "pick-or-add" lists used across volume
// editing (contribution types, property names, formats) - one generic mechanism instead of
// three near-duplicate collections, per durable-volume-editing's design.md.
package vocabularies

import "go.mongodb.org/mongo-driver/bson/primitive"

const CollectionName = "vocabularies"

// Vocabulary types. GET is available to any edit-capable role for TypeContributionType and
// TypePropertyName; TypeFormat additionally restricts GET to editor/admin (see
// volume-format-selector's spec). POST is editor/admin-only for every type.
const (
	TypeContributionType = "contribution-type"
	TypePropertyName     = "property-name"
	TypeFormat           = "format"
	TypeTag              = "tag"
)

// ValidTypes lists every recognized :type path value.
var ValidTypes = []string{TypeContributionType, TypePropertyName, TypeFormat, TypeTag}

// IsValidType reports whether t is a recognized vocabulary type.
func IsValidType(t string) bool {
	for _, v := range ValidTypes {
		if v == t {
			return true
		}
	}
	return false
}

// entry is a single (type, value) pair - the storage shape backing List/Add. Unexported since
// callers only ever see the plain value strings List returns.
type entry struct {
	ID    primitive.ObjectID `bson:"_id,omitempty"`
	Type  string             `bson:"type"`
	Value string             `bson:"value"`
}
