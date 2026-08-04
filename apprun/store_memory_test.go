package apprun

import (
	"encoding/json"
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

	// Verify initial status
	apps, _ := store.ListApplications(ListParams{})
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Status != "Healthy" {
		t.Fatalf("expected Healthy, got %s", apps[0].Status)
	}

	// Set status to Unhealthy (simulates container start failure)
	store.SetApplicationStatus(app.ID, "UnHealthy")

	// Verify status via ReadApplication
	read, ok := store.ReadApplication(app.ID)
	if !ok {
		t.Fatal("app not found")
	}
	if read.Status != "UnHealthy" {
		t.Fatalf("ReadApplication: expected Unhealthy, got %s", read.Status)
	}

	// Verify status via ListApplications
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

	// Verify status via ListApplications
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

	// GET /applications (list)
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

	// GET /applications/{id} (individual)
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
