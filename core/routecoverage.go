package core

import (
	"reflect"
	"sync"
	"testing"
)

// answeredRoutes records, under go test only, which routes of each
// responseSchemas table answered a request successfully. It is process-wide
// because a test binary builds a new server for almost every test.
var answeredRoutes = struct {
	enabled bool
	mu      sync.Mutex
	byTable map[uintptr]map[string]bool
}{enabled: testing.Testing(), byTable: map[uintptr]map[string]bool{}}

func markRouteAnswered(schemas map[string]map[int]*BodySchema, key string) {
	if !answeredRoutes.enabled {
		return
	}
	table := reflect.ValueOf(schemas).Pointer()
	answeredRoutes.mu.Lock()
	defer answeredRoutes.mu.Unlock()
	if answeredRoutes.byTable[table] == nil {
		answeredRoutes.byTable[table] = map[string]bool{}
	}
	answeredRoutes.byTable[table][key] = true
}

// AnsweredRoutes returns the keys of schemas ("METHOD /path") whose route
// answered at least one request with a non-error status in this process, so a
// test binary can tell which routes its tests had the response validator
// check. Nothing is recorded outside go test.
func AnsweredRoutes(schemas map[string]map[int]*BodySchema) map[string]bool {
	table := reflect.ValueOf(schemas).Pointer()
	answeredRoutes.mu.Lock()
	defer answeredRoutes.mu.Unlock()
	out := make(map[string]bool, len(answeredRoutes.byTable[table]))
	for key := range answeredRoutes.byTable[table] {
		out[key] = true
	}
	return out
}
