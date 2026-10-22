package app

import (
	"context"
	"docping/api/internal/domain"
	"log"
	"time"
)

// expiredOrganizations lists the organizations that exist without anyone
// having paid or proved anything, past their time to live: demonstrations,
// and registrations whose e-mail was never confirmed. It also returns the
// object keys those organizations own, because after the purge the state no
// longer says which objects belonged to them.
func expiredOrganizations(s *domain.State, now time.Time, demoTTL, unverifiedTTL time.Duration) (ids []string, keys []string) {
	verified := map[string]bool{}
	accounts := map[string]int{}
	for _, u := range s.Users {
		accounts[u.OrgID]++
		if !u.EmailPending {
			verified[u.OrgID] = true
		}
	}
	doomed := map[string]bool{}
	for id, o := range s.Organizations {
		created, err := time.Parse(time.RFC3339, o.CreatedAt)
		if err != nil {
			continue
		}
		age := now.Sub(created)
		switch {
		case o.Demo:
			if age <= demoTTL {
				continue
			}
		// An organization with no account at all is left alone: it is not a
		// known abandoned registration, and deleting it would hide a bug.
		case accounts[id] > 0 && !verified[id]:
			if age <= unverifiedTTL {
				continue
			}
		default:
			continue
		}
		doomed[id] = true
		ids = append(ids, id)
	}
	for _, r := range s.Requests {
		if !doomed[r.OrgID] {
			continue
		}
		for _, item := range r.Items {
			for _, f := range item.Files {
				// Sample files are synthesized on read and were never stored,
				// so there is no object to remove.
				if !f.Sample && f.ObjectKey != "" {
					keys = append(keys, f.ObjectKey)
				}
			}
		}
	}
	return ids, keys
}

// cleanup removes those organizations and their objects. Objects go first: a
// failure there leaves the state untouched and the next run tries again,
// whereas purging first would lose the only record of which objects to
// delete and leak them in the bucket forever.
func (a *App) cleanup(ctx context.Context, now time.Time) {
	var ids, keys []string
	if err := a.Store.Run(ctx, false, func(s *domain.State) error {
		ids, keys = expiredOrganizations(s, now, a.Config.DemoTTL, a.Config.UnverifiedTTL)
		return nil
	}); err != nil || len(ids) == 0 {
		return
	}
	if a.Objects != nil {
		for _, key := range keys {
			objectCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err := a.Objects.Delete(objectCtx, key)
			cancel()
			if err != nil {
				// Reported without the key, which names an organization and a
				// file. The purge is skipped this round and retried.
				log.Printf("cleanup: could not delete %d stored object(s); retrying next run", len(keys))
				return
			}
		}
	}
	if err := a.Store.PurgeOrganizations(ctx, ids); err != nil {
		log.Printf("cleanup: purge failed: %v", err)
		return
	}
	log.Printf("cleanup: removed %d expired organization(s) and %d object(s)", len(ids), len(keys))
}
