package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func contentHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// VerifyCompletedDownload requires a new import receipt, an exact client file
// inventory, and current byte-for-byte verification outside the deletion tree.
// Legacy imported flags cannot authorize automatic removal.
func (s *Service) VerifyCompletedDownload(ctx context.Context, download acquisition.DownloadStatus, inventory []acquisition.DownloadFile) error {
	if !s.Available() {
		return errors.New("no durable import receipt")
	}
	op, err := s.store.operationForDownload(ctx, download.Client, download.ID)
	if err == nil {
		return s.verifyOperationCleanup(ctx, op, download, inventory)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return s.verifySingleFileReceipt(ctx, download, inventory)
}

func (s *Service) verifySingleFileReceipt(ctx context.Context, download acquisition.DownloadStatus, inventory []acquisition.DownloadFile) error {
	if !s.Available() || download.ImportedFileID == "" {
		return errors.New("no durable import receipt")
	}
	files, err := s.store.FindFiles(ctx, []string{download.ImportedFileID}, nil)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return errors.New("imported file record is missing")
	}
	file := files[0]
	receipt, ok := file.Metadata["verifiedDownload"].(map[string]any)
	if !ok || receipt["client"] != download.Client || receipt["id"] != download.ID {
		return errors.New("download has no verified identity receipt; review required")
	}
	hash, _ := receipt["sha256"].(string)
	if len(hash) != 64 {
		return errors.New("import checksum is missing")
	}
	mapped := remapDownloadSavePath(download, s.remotePathMappings(ctx))
	source, _, err := locateDownloadSource(mapped)
	if err != nil {
		return err
	}
	destination, err := filepath.EvalSymlinks(file.Path)
	if err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(mapped.SavePath)
	if err != nil {
		return err
	}
	if pathWithinRoot(destination, root) {
		return errors.New("library destination is inside the download deletion tree")
	}
	count := 0
	for _, entry := range inventory {
		if _, supported := classifyFile(entry.Name); !supported {
			continue
		}
		count++
		relative := filepath.Clean(entry.Name)
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("unsafe client file path")
		}
		inventoryPath, err := filepath.EvalSymlinks(filepath.Join(root, relative))
		if err != nil || inventoryPath != source || entry.Progress < 1 {
			return errors.New("client inventory does not prove a complete imported payload")
		}
		info, err := os.Stat(source)
		if err != nil || entry.SizeBytes <= 0 || info.Size() != entry.SizeBytes {
			return errors.New("client file size does not match source")
		}
	}
	if count != 1 {
		return errors.New("complete client file inventory requires review")
	}
	sourceHash, err := contentHash(source)
	if err != nil {
		return err
	}
	destinationHash, err := contentHash(destination)
	if err != nil {
		return err
	}
	if hash != sourceHash || hash != destinationHash {
		return errors.New("imported content failed checksum verification")
	}
	return nil
}
