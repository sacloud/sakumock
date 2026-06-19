package iam

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/sacloud/sakumock/core"
)

type table[T any] struct {
	mu    sync.RWMutex
	items map[string]*T
	order []string
}

func newTable[T any]() *table[T] {
	return &table[T]{items: make(map[string]*T)}
}

func (t *table[T]) get(key string) (*T, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.items[key]
	return v, ok
}

func (t *table[T]) all() []*T {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*T, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, t.items[k])
	}
	return out
}

func (t *table[T]) set(key string, v *T) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.items[key]; !ok {
		t.order = append(t.order, key)
	}
	t.items[key] = v
}

func (t *table[T]) delete(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.items[key]; !ok {
		return false
	}
	delete(t.items, key)
	for i, k := range t.order {
		if k == key {
			t.order = append(t.order[:i], t.order[i+1:]...)
			break
		}
	}
	return true
}

type MemoryStore struct {
	ids    *core.IDGenerator
	logger *slog.Logger

	users             *table[UserRecord]
	groups            *table[GroupRecord]
	projects          *table[ProjectRecord]
	folders           *table[FolderRecord]
	servicePrincipals *table[ServicePrincipalRecord]
	spKeys            *table[ServicePrincipalKeyRecord]
	apiKeys           *table[ProjectAPIKeyRecord]
	ssoProfiles       *table[SSOProfileRecord]
	scimConfigs       *table[ScimConfigurationRecord]

	iamRoles []IAMRoleRecord
	idRoles  []IDRoleRecord

	mu                 sync.RWMutex
	organization       organizationState
	passwordPolicy     passwordPolicyState
	authConditions     json.RawMessage
	orgIAMPolicy       []PolicyBinding
	orgIDPolicy        []PolicyBinding
	projectIAMPolicies map[string][]PolicyBinding
	folderIAMPolicies  map[string][]PolicyBinding
	servicePolicyRules json.RawMessage
}

type organizationState struct {
	ID   int
	Name string
}

type passwordPolicyState struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireSymbols   bool `json:"require_symbols"`
}

func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	s := &MemoryStore{
		ids:    core.NewIDGenerator(core.DefaultIDBase),
		logger: logger,

		users:             newTable[UserRecord](),
		groups:            newTable[GroupRecord](),
		projects:          newTable[ProjectRecord](),
		folders:           newTable[FolderRecord](),
		servicePrincipals: newTable[ServicePrincipalRecord](),
		spKeys:            newTable[ServicePrincipalKeyRecord](),
		apiKeys:           newTable[ProjectAPIKeyRecord](),
		ssoProfiles:       newTable[SSOProfileRecord](),
		scimConfigs:       newTable[ScimConfigurationRecord](),

		organization:       organizationState{ID: 1, Name: "mock-organization"},
		passwordPolicy:     passwordPolicyState{MinLength: 12},
		authConditions:     json.RawMessage(`{"ip_restriction":{"enabled":false,"subnets":[]},"require_two_factor_auth":{"enabled":false},"datetime_restriction":{"after":null,"before":null}}`),
		orgIAMPolicy:       nil,
		orgIDPolicy:        nil,
		projectIAMPolicies: make(map[string][]PolicyBinding),
		folderIAMPolicies:  make(map[string][]PolicyBinding),
		servicePolicyRules: json.RawMessage(`{"rules":[]}`),
	}
	s.seedRoles()
	return s
}

func (s *MemoryStore) seedRoles() {
	s.iamRoles = []IAMRoleRecord{
		{ID: "owner", Name: "オーナー", Description: "全ての操作が可能", Category: "general", LowestGrantableResource: "project"},
		{ID: "editor", Name: "編集者", Description: "リソースの作成・変更・削除が可能", Category: "general", LowestGrantableResource: "project"},
		{ID: "viewer", Name: "閲覧者", Description: "リソースの閲覧のみ可能", Category: "general", LowestGrantableResource: "project"},
		{ID: "resource-creator", Name: "リソース作成者", Description: "リソースの作成が可能", Category: "general", LowestGrantableResource: "project"},
		{ID: "organization-admin", Name: "組織管理者", Description: "組織の管理が可能", Category: "organization", LowestGrantableResource: "organization"},
	}
	s.idRoles = []IDRoleRecord{
		{ID: "admin", Name: "管理者", Description: "ID管理の全操作が可能"},
		{ID: "member", Name: "メンバー", Description: "基本的な操作が可能"},
	}
}

func (s *MemoryStore) nextID() int {
	id, _ := strconv.Atoi(s.ids.Next())
	return id
}

func (s *MemoryStore) getPasswordPolicy() passwordPolicyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.passwordPolicy
}

func (s *MemoryStore) Close() error { return nil }

func newUUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

func idKey(id int) string { return strconv.Itoa(id) }
