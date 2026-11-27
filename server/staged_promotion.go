package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/common.go/logging"
)

const (
	stagedCoverKind  = "cover-staged"
	stagedSampleKind = "sample-staged"
)

// stagedOwner is the owner part of a staged asset id. It mirrors catalog-web's
// sanitizedAssetUserID and assets-web's ownership rule.
func stagedOwner(subject string) string {
	return strings.ReplaceAll(subject, "|", "-")
}

// validateStagedIDs accepts only the forms assets-web lets this subject write:
// cover-staged/<owner> and sample-staged/<owner>-<n>.
func validateStagedIDs(subject, coverID string, sampleIDs []string) error {
	if len(sampleIDs) > maxSampleAssetIds {
		return fmt.Errorf("a volume may have at most %d samples", maxSampleAssetIds)
	}
	owner := stagedOwner(subject)
	if coverID != "" && (owner == "" || coverID != owner) {
		return fmt.Errorf("staged cover is not one of your uploads")
	}
	for _, id := range sampleIDs {
		index, found := strings.CutPrefix(id, owner+"-")
		if _, err := strconv.ParseUint(index, 10, 32); owner == "" || !found || err != nil {
			return fmt.Errorf("staged sample is not one of your uploads")
		}
	}
	return nil
}

// promotion is the set of staged files one publication moves to live ids. Callers copy
// everything, publish, and only then delete the staged files, so a failure at either of the
// first two steps leaves every upload in place for a retry.
type promotion struct {
	files []promotedFile
	// coverID and sampleIDs are the live ids; nil when nothing of that kind is staged.
	coverID   *string
	sampleIDs []string
}

type promotedFile struct{ fromKind, fromID, toKind, toID string }

func newPromotion(volumeID, stagedCoverID string, stagedSampleIDs []string) promotion {
	var p promotion
	if stagedCoverID != "" {
		p.coverID = &volumeID
		p.files = append(p.files, promotedFile{stagedCoverKind, stagedCoverID, "cover", volumeID})
	}
	for i, stagedID := range stagedSampleIDs {
		liveID := fmt.Sprintf("%s-%d", volumeID, i)
		p.sampleIDs = append(p.sampleIDs, liveID)
		p.files = append(p.files, promotedFile{stagedSampleKind, stagedID, "sample", liveID})
	}
	return p
}

// copy copies every staged file to its live id. On failure it also returns the API error code.
func (p promotion) copy(ctx context.Context, client *assets.Client, token string) (string, error) {
	for _, f := range p.files {
		if err := client.Copy(ctx, token, f.fromKind, f.fromID, f.toKind, f.toID); err != nil {
			logging.Logger.Error("promotion: copy failed", "fromKind", f.fromKind, "toId", f.toID, "error", err)
			sentry.CaptureException(err)
			if f.fromKind == stagedCoverKind {
				return "cover_promote_failed", err
			}
			return "sample_promote_failed", err
		}
	}
	return "", nil
}

// deleteStaged removes the staged files after publication. A failure is only logged:
// the publication stands, and assets-web's reclaim job removes what's left.
func (p promotion) deleteStaged(ctx context.Context, client *assets.Client, token string) {
	// A client that hangs up after publication shouldn't leave the files behind.
	ctx = context.WithoutCancel(ctx)
	for _, f := range p.files {
		if err := client.Delete(ctx, token, f.fromKind, f.fromID); err != nil {
			logging.Logger.Warn("promotion: staged delete failed", "fromKind", f.fromKind, "toId", f.toID, "error", err)
			sentry.CaptureException(err)
		}
	}
}
