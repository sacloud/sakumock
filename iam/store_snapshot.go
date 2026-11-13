package iam

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir). Every
// table is an array in insertion order, which is the order list endpoints
// return. The IAM and ID roles are built in, not state, so they are not part
// of it.
type storeSnapshot struct {
	Users             []*UserRecord                `json:"users"`
	TrustedDevices    []*UserTrustedDeviceRecord   `json:"trusted_devices"`
	SecurityKeys      []*UserSecurityKeyRecord     `json:"security_keys"`
	Groups            []*GroupRecord               `json:"groups"`
	Projects          []*ProjectRecord             `json:"projects"`
	Folders           []*FolderRecord              `json:"folders"`
	ServicePrincipals []*ServicePrincipalRecord    `json:"service_principals"`
	SPKeys            []*ServicePrincipalKeyRecord `json:"service_principal_keys"`
	APIKeys           []*ProjectAPIKeyRecord       `json:"api_keys"`
	SSOProfiles       []*SSOProfileRecord          `json:"sso_profiles"`
	ScimConfigs       []*ScimConfigurationRecord   `json:"scim_configurations"`

	Organization   organizationState   `json:"organization"`
	PasswordPolicy passwordPolicyState `json:"password_policy"`
	AuthConditions json.RawMessage     `json:"auth_conditions"`
	OrgIAMPolicy   []PolicyBinding     `json:"organization_iam_policy"`
	OrgIDPolicy    []PolicyBinding     `json:"organization_id_policy"`
	// ProjectIAMPolicies and FolderIAMPolicies are keyed by project/folder ID.
	ProjectIAMPolicies   map[string][]PolicyBinding `json:"project_iam_policies"`
	FolderIAMPolicies    map[string][]PolicyBinding `json:"folder_iam_policies"`
	ServicePolicyRules   json.RawMessage            `json:"service_policy_rules"`
	ServicePolicyEnabled bool                       `json:"service_policy_enabled"`
}

// stage validates rows as the new content of t, keyed by key, and returns the
// function that puts them in place. Nothing changes until that function runs,
// so a snapshot with one bad table leaves the whole store untouched.
func stage[T any](t *table[T], name string, rows []*T, key func(*T) string) (func(), error) {
	items := make(map[string]*T, len(rows))
	order := make([]string, 0, len(rows))
	for i, row := range rows {
		if row == nil {
			return nil, fmt.Errorf("%s[%d]: record is missing", name, i)
		}
		k := key(row)
		if k == "" || k == "0" {
			return nil, fmt.Errorf("%s[%d]: record has no id", name, i)
		}
		if _, dup := items[k]; dup {
			return nil, fmt.Errorf("%s[%d]: duplicate id %s", name, i, k)
		}
		items[k] = row
		order = append(order, k)
	}
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.items = items
		t.order = order
	}, nil
}

// compactJSON returns raw without the indentation the state file gives it, so
// a restored document is served in the compact form it was stored in.
func compactJSON(name string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// Snapshot returns every resource, policy and organization setting as JSON,
// for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	snap := storeSnapshot{
		Users:             s.users.all(),
		TrustedDevices:    s.trustedDevices.all(),
		SecurityKeys:      s.securityKeys.all(),
		Groups:            s.groups.all(),
		Projects:          s.projects.all(),
		Folders:           s.folders.all(),
		ServicePrincipals: s.servicePrincipals.all(),
		SPKeys:            s.spKeys.all(),
		APIKeys:           s.apiKeys.all(),
		SSOProfiles:       s.ssoProfiles.all(),
		ScimConfigs:       s.scimConfigs.all(),
	}
	// Marshal under the lock: the policy maps are written in place by handlers.
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap.Organization = s.organization
	snap.PasswordPolicy = s.passwordPolicy
	snap.AuthConditions = s.authConditions
	snap.OrgIAMPolicy = s.orgIAMPolicy
	snap.OrgIDPolicy = s.orgIDPolicy
	snap.ProjectIAMPolicies = s.projectIAMPolicies
	snap.FolderIAMPolicies = s.folderIAMPolicies
	snap.ServicePolicyRules = s.servicePolicyRules
	snap.ServicePolicyEnabled = s.servicePolicyEnabled
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's resources, policies and organization settings
// with a snapshot, and advances the ID generator past the restored resource
// IDs so new resources never reuse one.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Organization.ID == 0 {
		return errors.New("organization: record is missing or has no id")
	}
	authConditions, err := compactJSON("auth_conditions", snap.AuthConditions)
	if err != nil {
		return err
	}
	if len(authConditions) == 0 {
		// Served verbatim as the response body, so it has to be a document.
		return errors.New("auth_conditions: document is missing")
	}
	servicePolicyRules, err := compactJSON("service_policy_rules", snap.ServicePolicyRules)
	if err != nil {
		return err
	}
	if snap.ProjectIAMPolicies == nil {
		snap.ProjectIAMPolicies = make(map[string][]PolicyBinding)
	}
	if snap.FolderIAMPolicies == nil {
		snap.FolderIAMPolicies = make(map[string][]PolicyBinding)
	}

	var (
		commits []func()
		errs    []error
	)
	add := func(commit func(), err error) {
		commits = append(commits, commit)
		errs = append(errs, err)
	}
	add(stage(s.users, "users", snap.Users, func(r *UserRecord) string { return idKey(r.ID) }))
	add(stage(s.trustedDevices, "trusted_devices", snap.TrustedDevices, func(r *UserTrustedDeviceRecord) string { return idKey(r.ID) }))
	add(stage(s.securityKeys, "security_keys", snap.SecurityKeys, func(r *UserSecurityKeyRecord) string { return idKey(r.ID) }))
	add(stage(s.groups, "groups", snap.Groups, func(r *GroupRecord) string { return idKey(r.ID) }))
	add(stage(s.projects, "projects", snap.Projects, func(r *ProjectRecord) string { return idKey(r.ID) }))
	add(stage(s.folders, "folders", snap.Folders, func(r *FolderRecord) string { return idKey(r.ID) }))
	add(stage(s.servicePrincipals, "service_principals", snap.ServicePrincipals, func(r *ServicePrincipalRecord) string { return idKey(r.ID) }))
	add(stage(s.spKeys, "service_principal_keys", snap.SPKeys, func(r *ServicePrincipalKeyRecord) string { return r.ID }))
	add(stage(s.apiKeys, "api_keys", snap.APIKeys, func(r *ProjectAPIKeyRecord) string { return idKey(r.ID) }))
	add(stage(s.ssoProfiles, "sso_profiles", snap.SSOProfiles, func(r *SSOProfileRecord) string { return idKey(r.ID) }))
	add(stage(s.scimConfigs, "scim_configurations", snap.ScimConfigs, func(r *ScimConfigurationRecord) string { return r.ID }))
	if err := errors.Join(errs...); err != nil {
		return err
	}
	for _, commit := range commits {
		commit()
	}

	s.mu.Lock()
	s.organization = snap.Organization
	s.passwordPolicy = snap.PasswordPolicy
	s.authConditions = authConditions
	s.orgIAMPolicy = snap.OrgIAMPolicy
	s.orgIDPolicy = snap.OrgIDPolicy
	s.projectIAMPolicies = snap.ProjectIAMPolicies
	s.folderIAMPolicies = snap.FolderIAMPolicies
	s.servicePolicyRules = servicePolicyRules
	s.servicePolicyEnabled = snap.ServicePolicyEnabled
	s.mu.Unlock()

	for _, r := range snap.Users {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.TrustedDevices {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.SecurityKeys {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.Groups {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.Projects {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.Folders {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.ServicePrincipals {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.APIKeys {
		s.ids.Observe(idKey(r.ID))
	}
	for _, r := range snap.SSOProfiles {
		s.ids.Observe(idKey(r.ID))
	}
	return nil
}
