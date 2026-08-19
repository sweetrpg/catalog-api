package server

import (
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

// invalidateCachedPaths deletes catalog-api's Redis-backed GET response cache (see
// docs/service-conventions.md's Caching section) for each given request path - best-effort, same
// "a cache miss just costs one extra backend fetch" tolerance CachePage itself has. No write path
// in this service invalidated this cache before - see the entity-association write paths that
// call this (applyVolumePatch, patchLicenseVolumes, patchStudioVolumes) - so a changed
// association (e.g. a volume newly linked to a publisher) stayed stale on that publisher's
// browse-card volume count and detail page for the rest of CACHE_TTLS' window (tens of minutes),
// not just until the writing request's own response.
func invalidateCachedPaths(store persistence.CacheStore, paths ...string) {
	for _, p := range paths {
		_ = store.Delete(cache.CreateKey(p))
	}
}

// invalidateVolumeAssociationCache invalidates every cached GET response a change to before's/
// after's Publishers/Studios/Systems could have made stale: each affected entity's own
// `/<type>/<id>` and `/<type>/<id>/volumes` (its browse-card volume count and detail page), each
// affected type's top-level list (`/publishers`, `/studios`, `/systems`), and the volume's own
// `/volumes/<id>` and `/volumes` list. Called with both the pre- and post-patch volume so an id
// that was removed gets invalidated too, not just one newly added.
func invalidateVolumeAssociationCache(store persistence.CacheStore, before, after *vo.VolumeVO) {
	paths := []string{"/volumes", "/volumes/" + before.ID, "/publishers", "/studios", "/systems"}
	for _, id := range unionPublisherIDs(before.Publishers, after.Publishers) {
		paths = append(paths, "/publishers/"+id, "/publishers/"+id+"/volumes")
	}
	for _, id := range unionStudioIDs(before.Studios, after.Studios) {
		paths = append(paths, "/studios/"+id, "/studios/"+id+"/volumes")
	}
	for _, id := range unionSystemIDs(before.Systems, after.Systems) {
		paths = append(paths, "/systems/"+id, "/systems/"+id+"/volumes")
	}
	invalidateCachedPaths(store, paths...)
}

func unionPublisherIDs(a, b []*vo.PublisherVO) []string {
	seen := map[string]bool{}
	var ids []string
	for _, p := range append(append([]*vo.PublisherVO{}, a...), b...) {
		if p != nil && !seen[p.ID] {
			seen[p.ID] = true
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func unionStudioIDs(a, b []*vo.StudioVO) []string {
	seen := map[string]bool{}
	var ids []string
	for _, s := range append(append([]*vo.StudioVO{}, a...), b...) {
		if s != nil && !seen[s.ID] {
			seen[s.ID] = true
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// invalidateLicenseVolumesCache invalidates the license's own cached paths, the top-level
// `/licenses` list, and every affected volume's `/volumes/<id>` (a volume's license badge on its
// own detail page would otherwise stay stale too) after a `patchLicenseVolumes` full-replace.
func invalidateLicenseVolumesCache(store persistence.CacheStore, licenseID string, before, after []string) {
	paths := []string{
		"/licenses", "/licenses/" + licenseID, "/licenses/" + licenseID + "/volumes", "/volumes",
	}
	for _, vid := range unionStrings(before, after) {
		paths = append(paths, "/volumes/"+vid)
	}
	invalidateCachedPaths(store, paths...)
}

// invalidateStudioVolumesCache is invalidateLicenseVolumesCache's studio counterpart.
func invalidateStudioVolumesCache(store persistence.CacheStore, studioID string, before, after []string) {
	paths := []string{
		"/studios", "/studios/" + studioID, "/studios/" + studioID + "/volumes", "/volumes",
	}
	for _, vid := range unionStrings(before, after) {
		paths = append(paths, "/volumes/"+vid)
	}
	invalidateCachedPaths(store, paths...)
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func unionSystemIDs(a, b []*vo.SystemVO) []string {
	seen := map[string]bool{}
	var ids []string
	for _, s := range append(append([]*vo.SystemVO{}, a...), b...) {
		if s != nil && !seen[s.ID] {
			seen[s.ID] = true
			ids = append(ids, s.ID)
		}
	}
	return ids
}
