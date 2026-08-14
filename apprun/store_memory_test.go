package apprun

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetApplicationStatusReflectedInList(t *testing.T) {
	store := NewStore(func(appID string) string {
		return "http://" + appID + ".localhost:28088"
	})

	app := &Application{
		Name:           "test-app",
		TimeoutSeconds: 60,
		Port:           8080,
		MinScale:       0,
		MaxScale:       1,
		Components: []Component{{
			Name: "web",
			DeploySource: DeploySource{
				ContainerRegistry: &ContainerRegistry{
					Image: "nginx:latest",
				},
			},
		}},
	}

	if err := store.CreateApplication(app); err != nil {
		t.Fatal(err)
	}

	apps, _ := store.ListApplications(ListParams{})
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Status != "Healthy" {
		t.Fatalf("expected Healthy, got %s", apps[0].Status)
	}

	// Set status to Unhealthy (simulates container start failure)
	store.SetApplicationStatus(app.ID, "UnHealthy")

	read, ok := store.ReadApplication(app.ID)
	if !ok {
		t.Fatal("app not found")
	}
	if read.Status != "UnHealthy" {
		t.Fatalf("ReadApplication: expected Unhealthy, got %s", read.Status)
	}

	apps, _ = store.ListApplications(ListParams{})
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Status != "UnHealthy" {
		t.Fatalf("ListApplications: expected Unhealthy, got %s", apps[0].Status)
	}
}

func TestSetApplicationStatusAfterUpdate(t *testing.T) {
	store := NewStore(func(appID string) string {
		return "http://" + appID + ".localhost:28088"
	})

	app := &Application{
		Name:           "test-app",
		TimeoutSeconds: 60,
		Port:           8080,
		MinScale:       0,
		MaxScale:       1,
		Components: []Component{{
			Name: "web",
			DeploySource: DeploySource{
				ContainerRegistry: &ContainerRegistry{
					Image: "nginx:latest",
				},
			},
		}},
	}

	if err := store.CreateApplication(app); err != nil {
		t.Fatal(err)
	}

	// Update creates a new version entry
	if err := store.UpdateApplication(app.ID, &Application{
		TimeoutSeconds: 120,
		MinScale:       -1,
	}); err != nil {
		t.Fatal(err)
	}

	// Set status to Unhealthy on the latest version
	store.SetApplicationStatus(app.ID, "UnHealthy")

	apps, _ := store.ListApplications(ListParams{})
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Status != "UnHealthy" {
		t.Fatalf("ListApplications after update: expected Unhealthy, got %s", apps[0].Status)
	}
}

func TestListEndpointReflectsStatusChange(t *testing.T) {
	srv, err := NewHandler(Config{})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{
		"name": "test-app",
		"timeout_seconds": 60,
		"port": 8080,
		"min_scale": 0,
		"max_scale": 1,
		"components": [{
			"name": "web",
			"max_cpu": "0.5",
			"max_memory": "1Gi",
			"deploy_source": {
				"container_registry": {"image": "nginx:latest"}
			}
		}]
	}`

	resp, err := http.Post(ts.URL+"/applications", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST status %d: %s", resp.StatusCode, b)
	}

	var created applicationJSON
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	// Simulate container start failure
	srv.store.SetApplicationStatus(created.ID, "UnHealthy")

	resp, err = http.Get(ts.URL + "/applications")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var listResp listApplicationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 app in list, got %d", len(listResp.Data))
	}
	if listResp.Data[0].Status != "UnHealthy" {
		t.Fatalf("list endpoint: expected Unhealthy, got %s", listResp.Data[0].Status)
	}

	resp, err = http.Get(ts.URL + "/applications/" + created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var got applicationJSON
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "UnHealthy" {
		t.Fatalf("get endpoint: expected Unhealthy, got %s", got.Status)
	}
}

func newTestApp(name string) *Application {
	return &Application{
		Name:           name,
		TimeoutSeconds: 60,
		Port:           8080,
		MinScale:       0,
		MaxScale:       1,
		Components: []Component{{
			Name: "web",
			DeploySource: DeploySource{
				ContainerRegistry: &ContainerRegistry{
					Image: "nginx:latest",
				},
			},
		}},
	}
}

// CreatedAt is truncated to the second, so versions created by consecutive
// updates usually share a timestamp. The listing must still return them in
// creation order — handleDeleteVersion treats the first entry as the latest
// version and refuses to delete it.
func TestListVersionsOrderWithinSameSecond(t *testing.T) {
	store := NewStore(func(appID string) string {
		return "http://" + appID + ".localhost:28088"
	})

	app := newTestApp("test-app")
	if err := store.CreateApplication(app); err != nil {
		t.Fatal(err)
	}
	for i := range maxVersionsPerApp - 1 {
		if err := store.UpdateApplication(app.ID, &Application{
			TimeoutSeconds: 61 + i,
			MinScale:       -1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	created := make([]string, 0, maxVersionsPerApp)
	for i := 1; i <= maxVersionsPerApp; i++ {
		created = append(created, fmt.Sprintf("%s-%s-%d", app.Name, app.ID, i))
	}

	for _, tc := range []struct {
		sortOrder string
		want      []string
	}{
		{sortOrder: "", want: reversed(created)},
		{sortOrder: "desc", want: reversed(created)},
		{sortOrder: "asc", want: created},
	} {
		versions, total := store.ListVersions(app.ID, ListParams{SortOrder: tc.sortOrder})
		if total != len(created) {
			t.Fatalf("sort_order=%q: total = %d, want %d", tc.sortOrder, total, len(created))
		}
		for i, v := range versions {
			if v.Name != tc.want[i] {
				t.Errorf("sort_order=%q: version[%d] = %s, want %s", tc.sortOrder, i, v.Name, tc.want[i])
			}
		}
	}
}

// Applications are held in a map, so without the creation-order tiebreak the
// listing order of same-second applications varies between calls.
func TestListApplicationsOrderWithinSameSecond(t *testing.T) {
	store := NewStore(func(appID string) string {
		return "http://" + appID + ".localhost:28088"
	})

	var created []string
	for i := range 5 {
		app := newTestApp(fmt.Sprintf("app-%d", i))
		if err := store.CreateApplication(app); err != nil {
			t.Fatal(err)
		}
		created = append(created, app.Name)
	}

	for range 20 {
		for _, tc := range []struct {
			sortOrder string
			want      []string
		}{
			{sortOrder: "", want: reversed(created)},
			{sortOrder: "desc", want: reversed(created)},
			{sortOrder: "asc", want: created},
		} {
			apps, total := store.ListApplications(ListParams{SortOrder: tc.sortOrder})
			if total != len(created) {
				t.Fatalf("sort_order=%q: total = %d, want %d", tc.sortOrder, total, len(created))
			}
			for i, a := range apps {
				if a.Name != tc.want[i] {
					t.Fatalf("sort_order=%q: app[%d] = %s, want %s", tc.sortOrder, i, a.Name, tc.want[i])
				}
			}
		}
	}
}

func reversed(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[len(names)-1-i] = n
	}
	return out
}
