package store

import (
	"context"
	"log"
	"time"
)

func (s *Store) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			batch, cancel := context.WithTimeout(ctx, time.Minute)
			err := s.cleanup(batch, now)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Print("Metadata cleanup did not complete; check PostgreSQL availability and locks. Remaining expired records will be retried next hour.")
			}
		}
	}
}

func (s *Store) cleanup(ctx context.Context, now time.Time) error {
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM sessions WHERE expires<$1", now.Unix()); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM oauth WHERE expires>0 AND expires<$1", now.Unix()); err != nil {
		return err
	}
	cutoff := now.Add(-30 * 24 * time.Hour).UnixMilli()
	for {
		// Commit each bounded deletion so audit writers can acquire their ordering lock.
		result, err := s.DB.ExecContext(ctx, "DELETE FROM audit WHERE id IN (SELECT id FROM audit WHERE at<$1 ORDER BY at LIMIT 1000)", cutoff)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows < 1000 {
			return nil
		}
	}
}
