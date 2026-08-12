package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Control-plane access keys (account keys and permission keys) are mirrored
// into versitygw's internal IAM service so S3 requests signed with an issued
// key authenticate on the data plane. The internal IAM service (enabled by
// --iam-dir) keeps its accounts in a users.json file that it re-reads on every
// credential lookup and rewrites via an atomic temp-file+rename, explicitly
// tolerating uncoordinated external writers (that is how multiple gateway
// instances share one account database). sakumock leans on that contract and
// writes the file directly instead of exec'ing `versitygw admin`, which keeps
// registration synchronous, independent of the gateway's TLS setup, and free
// of per-key subprocesses. sakumock is the file's only writer in practice:
// versitygw itself writes it only through its admin API, which sakumock never
// calls.
//
// The schema mirrors versitygw's auth.iAMConfig/auth.Account (unchanged from
// v1.5.0, the version CI pins, through current main). The data-plane e2e test
// exercises an issued key against the real binary, so a future schema change
// fails tests instead of silently breaking authentication.

// iamUsersFile is the account database versitygw's internal IAM service keeps
// inside its --iam-dir.
const iamUsersFile = "users.json"

// iamConfig mirrors versitygw's auth.iAMConfig.
type iamConfig struct {
	AccessAccounts map[string]iamAccount `json:"accessAccounts"`
}

// iamAccount mirrors versitygw's auth.Account.
type iamAccount struct {
	Access    string `json:"access"`
	Secret    string `json:"secret"`
	Role      string `json:"role"`
	UserID    int    `json:"userID"`
	GroupID   int    `json:"groupID"`
	ProjectID int    `json:"projectID"`
}

// iamRoleAdmin is the role every mirrored key is registered with. The mock
// authenticates issued keys but does not enforce per-bucket permissions:
// buckets are mirrored as plain directories with no owner metadata, so
// versitygw's "user" role (owned-buckets-only) would deny access outright.
const iamRoleAdmin = "admin"

// createUser registers a control-plane access key as a data-plane account. It
// is a no-op on a nil receiver, so callers need not check whether the data
// plane is enabled. A failure is returned rather than just logged so the
// control plane never issues a key the data plane does not authenticate.
func (d *dataPlane) createUser(access, secret string) error {
	if d == nil {
		return nil
	}
	if err := d.updateIAM(func(conf *iamConfig) {
		conf.AccessAccounts[access] = iamAccount{Access: access, Secret: secret, Role: iamRoleAdmin}
	}); err != nil {
		return fmt.Errorf("data plane: register access key: %w", err)
	}
	return nil
}

// deleteUser removes a control-plane access key from the data-plane accounts.
// Deletion takes effect immediately because versitygw runs with its IAM cache
// disabled (see startDataPlane).
func (d *dataPlane) deleteUser(access string) {
	if d == nil {
		return
	}
	if err := d.updateIAM(func(conf *iamConfig) {
		delete(conf.AccessAccounts, access)
	}); err != nil {
		d.logger.Warn("data plane: failed to deregister access key", "access_key", access, "error", err)
	}
}

// updateIAM applies update to users.json under a read-modify-write guarded by
// iamMu and committed with the same temp-file+rename atomic replace versitygw
// itself uses, so the gateway (which re-reads the file per lookup) never
// observes a partial write.
func (d *dataPlane) updateIAM(update func(*iamConfig)) error {
	d.iamMu.Lock()
	defer d.iamMu.Unlock()

	fname := filepath.Join(d.iamDir, iamUsersFile)
	conf := iamConfig{AccessAccounts: map[string]iamAccount{}}
	b, err := os.ReadFile(fname)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &conf); err != nil {
			return fmt.Errorf("parse %s: %w", iamUsersFile, err)
		}
		if conf.AccessAccounts == nil {
			conf.AccessAccounts = map[string]iamAccount{}
		}
	case errors.Is(err, fs.ErrNotExist):
		// versitygw creates an empty users.json at startup, before it starts
		// listening; starting from an empty config covers the file being gone.
	default:
		return fmt.Errorf("read %s: %w", iamUsersFile, err)
	}

	update(&conf)

	b, err = json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("serialize %s: %w", iamUsersFile, err)
	}
	f, err := os.CreateTemp(d.iamDir, iamUsersFile+".tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name())
	_, werr := f.Write(b)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Errorf("write temp file: %w", werr)
	}
	if err := os.Rename(f.Name(), fname); err != nil {
		return fmt.Errorf("replace %s: %w", iamUsersFile, err)
	}
	return nil
}
