package app

import (
	"context"
	"os"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// currentPID reports this process id, or 0 when it cannot be determined.
// CaptureOwnerPID is only meaningful when this process actually owns capture;
// it tells the user which instance is recording.
func currentPID() int { return os.Getpid() }

// diagnosticsTimeout bounds the diagnostics query. It is a read, so it uses the
// storage read ceiling from docs/05 §5.6.1.
const diagnosticsTimeout = 5 * time.Second

// GetDiagnostics reports storage and platform health.
//
// The fields whose data sources do not exist yet are reported through
// DTO.Unavailable with a reason rather than as a bare zero. docs/08 §8.4 treats
// a silent zero as indistinguishable from a working zero, and the whole point
// of this method is to make degradation visible.
func (b *Backend) GetDiagnostics() (DiagnosticsDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsTimeout)
	defer cancel()

	dto := DiagnosticsDTO{
		NativeState: NativeStateUnavailable,
		Unavailable: map[string]string{},
	}
	if b.system != nil {
		dto.NativeState = NativeStateOK
	}

	store := b.store()
	if store == nil {
		dto.DBStatus = DBStatusUnavailable
		// Report why, without leaking a path or a driver message. The class of
		// failure is what the user can act on.
		if err := b.storageFailure(); err != nil {
			dto.Unavailable["database"] = storageFailureReason(err)
		} else {
			dto.Unavailable["database"] = "database was not opened"
		}
		b.markPendingDiagnostics(dto.Unavailable)
		return dto, nil
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		// A failing diagnostics query is itself a database_error: the caller
		// asked for health and the health check could not run.
		return DiagnosticsDTO{}, mapStorageError("read diagnostics", err)
	}

	dto.DatabasePath = stats.DatabasePath
	dto.DatabaseBytes = stats.DatabaseBytes + stats.WALBytes
	dto.SkippedCardsToday = int(stats.SkippedCards)

	if store.Mode() == storage.ModeReadOnly {
		dto.DBStatus = DBStatusReadOnly
	} else {
		dto.DBStatus = DBStatusOK
	}
	if store.Instance().CaptureOwner {
		if pid := currentPID(); pid > 0 {
			dto.CaptureOwnerPID = &pid
		}
	}

	if stats.RecordingsAvailable {
		dto.RecordingsBytes = stats.RecordingsBytes
	} else {
		// The screenshots table belongs to the recording module and does not
		// exist yet. Reporting 0 without saying so would look like "no
		// recordings", which is a different claim.
		dto.Unavailable["recordingsBytes"] = "screenshots table not created yet"
	}
	dto.Unavailable["lastCaptureAtTs"] = "capture is not implemented"

	b.markPendingDiagnostics(dto.Unavailable)
	return dto, nil
}

// markPendingDiagnostics records the counts that have no data source yet.
func (b *Backend) markPendingDiagnostics(unavailable map[string]string) {
	// The batch tables belong to the timeline module.
	unavailable["pendingBatches"] = "analysis_batches table not created yet"
	unavailable["failedBatches"] = "analysis_batches table not created yet"
}

// storageFailureReason maps an open failure to a short, non-identifying reason.
// It never returns the underlying message: a driver error can quote a path.
func storageFailureReason(err error) string {
	kind, _ := storage.KindOf(err)
	switch kind {
	case storage.KindCorrupt:
		return "database file is damaged"
	case storage.KindReadOnly:
		return "database is read-only"
	case storage.KindEnvironment:
		return "database could not be opened"
	default:
		return "database is unavailable"
	}
}

// mapStorageError converts a storage failure into the closed application error
// set. This is the single mapping point docs/05 §5.6.1 requires: storage does
// not know about apperr, and the binding layer does not inspect driver errors.
func mapStorageError(op string, err error) error {
	if err == nil {
		return nil
	}
	kind, ok := storage.KindOf(err)
	if !ok {
		return apperr.E(apperr.Internal, op+" failed", err)
	}
	switch kind {
	case storage.KindBusy:
		return apperr.E(apperr.Conflict, op+": database is busy", err)
	case storage.KindCorrupt:
		return apperr.E(apperr.DatabaseError, op+": database is damaged", err)
	case storage.KindReadOnly:
		return apperr.E(apperr.DatabaseError, op+": database is read-only", err)
	case storage.KindNotFound:
		return apperr.E(apperr.NotFound, op+": not found", err)
	case storage.KindConstraint:
		return apperr.E(apperr.InvalidArgument, op+": constraint violation", err)
	default:
		return apperr.E(apperr.DatabaseError, op+" failed", err)
	}
}
