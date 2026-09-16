package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func operationExclusions(op ImportOperation) (map[string]bool, error) {
	excluded := map[string]bool{}
	raw, err := json.Marshal(op.Metadata["exclusions"])
	if err != nil {
		return nil, err
	}
	var files []PayloadFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, err
	}
	for _, f := range files {
		excluded[f.SourcePath] = true
	}
	return excluded, nil
}

func (s *Service) verifyOperationInventory(ctx context.Context, op ImportOperation, cleanup bool) error {
	payload, err := s.inspectDownloadPayload(ctx, acquisition.DownloadStatus{Client: op.Client, ID: op.DownloadID})
	if err != nil {
		return err
	}
	if payload.SourceRoot != op.SourceRoot {
		return errors.New("client download path changed since import was planned")
	}
	excluded, err := operationExclusions(op)
	if err != nil {
		return err
	}
	planned := map[string]ImportOperationFile{}
	for _, f := range op.Files {
		planned[f.SourcePath] = f
	}
	for _, f := range payload.Files {
		if f.Format == "excluded" {
			continue
		}
		if excluded[f.SourcePath] {
			if cleanup {
				return errors.New("operator-retained payload files block automatic source deletion")
			}
			continue
		}
		expected, ok := planned[f.SourcePath]
		if !ok {
			return fmt.Errorf("client payload has an unmapped required file: %s", f.RelativePath)
		}
		if !f.Included || f.SizeBytes != expected.SizeBytes {
			return fmt.Errorf("client payload no longer proves a complete file: %s", f.RelativePath)
		}
		delete(planned, f.SourcePath)
	}
	if len(planned) > 0 {
		return errors.New("required manifest files are missing from current client inventory")
	}
	return nil
}
