package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func plannedStageFixture(t *testing.T) (*Service, ImportOperation) {
	t.Helper()
	service, _, download, wantedID := operationFixture(t)
	ctx := context.Background()
	payload, err := service.inspectDownloadPayload(ctx, download)
	if err != nil {
		t.Fatal(err)
	}
	op, _, err := service.planPayload(ctx, payload, ImportRequest{WantedID: wantedID, ImportMode: "copy"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	op, err = service.store.planOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	return service, op
}

func TestImportReclaimsOnlyJournaledStageAfterRestart(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial-copy", true: "after-publication"}[published], func(t *testing.T) {
			service, op := plannedStageFixture(t)
			ctx := context.Background()
			token, err := service.store.claimOperation(ctx, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			op.LeaseToken = token
			file := op.Files[0]
			if err := os.MkdirAll(filepath.Dir(file.DestinationPath), 0755); err != nil {
				t.Fatal(err)
			}
			stage := manifestStagePath(file, token)
			if err := service.store.journalImportStage(ctx, op, file, stage); err != nil {
				t.Fatal(err)
			}
			if published {
				if err := copyManifestStage(ctx, file, stage); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(stage, file.DestinationPath); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(stage, []byte("interrupted"), 0644); err != nil {
				t.Fatal(err)
			}
			unrelated := filepath.Join(filepath.Dir(stage), ".librarry-stage-unowned")
			if err := os.WriteFile(unrelated, []byte("retain"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := service.store.db.Exec(`update import_operations set lease_expires_at=now()-interval '1 second' where id=$1`, op.ID); err != nil {
				t.Fatal(err)
			}
			// Use a new service with an old caller snapshot: the executor must reload
			// the journal after acquiring its new lease.
			restarted := NewService(service.store, service.Config(), service.wanted, service.downloads).WithDownloadInspector(service.inspector)
			outcome, err := restarted.runImportOperation(ctx, op)
			if err != nil || !outcome.Imported {
				t.Fatalf("resume: %+v %v", outcome, err)
			}
			if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("orphan stage remains: %v", err)
			}
			if data, err := os.ReadFile(unrelated); err != nil || string(data) != "retain" {
				t.Fatal("unowned temporary file was touched")
			}
			if err := verifyManifestPath(file.SourcePath, file); err != nil {
				t.Fatal(err)
			}
			if err := verifyManifestPath(file.DestinationPath, file); err != nil {
				t.Fatal(err)
			}
			saved, err := service.store.getOperation(ctx, op.ID)
			if err != nil || saved.Files[0].StagePath != "" {
				t.Fatalf("journal not cleared: %+v %v", saved, err)
			}
		})
	}
}

func TestStagingLeaseFenceAndChangedSource(t *testing.T) {
	service, op := plannedStageFixture(t)
	ctx := context.Background()
	token, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	file := op.Files[0]
	if err := os.MkdirAll(filepath.Dir(file.DestinationPath), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.db.Exec(`update import_operations set lease_expires_at=now()-interval '1 second' where id=$1`, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.transferOperationFile(ctx, op, file); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("stale worker began staging: %v", err)
	}
	if _, err := os.Stat(manifestStagePath(file, token)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale worker wrote bytes")
	}
	token, err = service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	if err := os.WriteFile(file.SourcePath, []byte("changed source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.transferOperationFile(ctx, op, file); err == nil {
		t.Fatal("changed source published")
	}
	if _, err := os.Stat(file.DestinationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unverified file became visible")
	}
	saved, err := service.store.getOperation(ctx, op.ID)
	if err != nil || saved.Files[0].StagePath == "" {
		t.Fatalf("failed copy has no journal: %+v %v", saved, err)
	}
}

func TestStagingRejectsForgedPathAndSymlink(t *testing.T) {
	service, op := plannedStageFixture(t)
	ctx := context.Background()
	token, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	file := op.Files[0]
	file.StageLeaseToken = token
	file.StagePath = file.SourcePath
	if err := service.reclaimImportStage(ctx, op, file); err == nil {
		t.Fatal("source accepted as reclaimable stage")
	}
	file.StagePath = manifestStagePath(file, token)
	if err := os.MkdirAll(filepath.Dir(file.StagePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file.SourcePath, file.StagePath); err != nil {
		t.Fatal(err)
	}
	if err := service.reclaimImportStage(ctx, op, file); err == nil {
		t.Fatal("symlink stage accepted")
	}
	if err := verifyManifestPath(file.SourcePath, file); err != nil {
		t.Fatal(err)
	}
}

func TestStaleWorkerCannotPublishAfterStaging(t *testing.T) {
	service, op := plannedStageFixture(t)
	ctx := context.Background()
	token, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	file := op.Files[0]
	if err := os.MkdirAll(filepath.Dir(file.DestinationPath), 0755); err != nil {
		t.Fatal(err)
	}
	// Lose ownership immediately after journaling, before the slow file copy.
	// A successful copy must not be sufficient authority to publish.
	if _, err := service.store.db.Exec(`create function expire_staging_lease() returns trigger language plpgsql as $$
 begin update import_operations set lease_expires_at=now()-interval '1 second' where id=new.operation_id; return new; end $$;
 create trigger expire_staging_lease after update of stage_path on import_operation_files
 for each row when (new.stage_path<>'') execute function expire_staging_lease()`); err != nil {
		t.Fatal(err)
	}
	if err := service.transferOperationFile(ctx, op, file); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("stale publication: %v", err)
	}
	if _, err := os.Stat(file.DestinationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale worker published")
	}
	if err := verifyManifestPath(manifestStagePath(file, token), file); err != nil {
		t.Fatal("copy did not reach publication fence", err)
	}
}

func TestUnownedStageCollisionRemainsUntouched(t *testing.T) {
	service, op := plannedStageFixture(t)
	ctx := context.Background()
	token, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	file := op.Files[0]
	path := manifestStagePath(file, token)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unowned bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.transferOperationFile(ctx, op, file); err == nil {
		t.Fatal("unowned stage adopted")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "unowned bytes" {
		t.Fatal("unowned file changed")
	}
	saved, err := service.store.getOperation(ctx, op.ID)
	if err != nil || saved.Files[0].StagePath != "" {
		t.Fatalf("unowned path was journaled: %+v %v", saved, err)
	}
}
