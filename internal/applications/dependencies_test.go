package applications

import (
	"errors"
	"testing"
)

func TestDependenciesAcceptADiamond(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	for _, directory := range []string{"api", "worker", "frontend", "database"} {
		makeFolder(t, project, directory)
	}
	api := addApplication(t, database, project.ID, "api")
	worker := addApplication(t, database, project.ID, "worker")
	frontend := addApplication(t, database, project.ID, "frontend")
	storage := addApplication(t, database, project.ID, "database")

	// Two applications share one dependency and one application needs both: a
	// diamond, which is a graph, not a cycle.
	for _, edge := range []struct {
		app       Application
		dependsOn []string
	}{
		{api, []string{storage.ID}},
		{worker, []string{storage.ID}},
		{frontend, []string{api.ID, worker.ID}},
	} {
		if err := SetDependencies(database, edge.app.ID, edge.dependsOn); err != nil {
			t.Fatalf("set dependencies for %q: %v", edge.app.Name, err)
		}
	}

	stored, err := Get(database, frontend.ID)
	if err != nil {
		t.Fatalf("get frontend: %v", err)
	}
	if len(stored.DependsOn) != 2 {
		t.Fatalf("dependencies = %v, want both applications", stored.DependsOn)
	}
}

func TestDependenciesRefuseATwoApplicationCycle(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "api")
	makeFolder(t, project, "worker")
	api := addApplication(t, database, project.ID, "api")
	worker := addApplication(t, database, project.ID, "worker")

	if err := SetDependencies(database, api.ID, []string{worker.ID}); err != nil {
		t.Fatalf("set dependencies: %v", err)
	}
	if err := SetDependencies(database, worker.ID, []string{api.ID}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("err = %v, want the cycle refusal", err)
	}

	// The refused edge is not stored, so the working order survives.
	stored, err := Get(database, worker.ID)
	if err != nil {
		t.Fatalf("get worker: %v", err)
	}
	if len(stored.DependsOn) != 0 {
		t.Fatalf("worker dependencies = %v, want the refused edge left out", stored.DependsOn)
	}
}

func TestDependenciesRefuseALongerCycle(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	names := []string{"api", "worker", "frontend"}
	apps := make(map[string]Application, len(names))
	for _, name := range names {
		makeFolder(t, project, name)
		apps[name] = addApplication(t, database, project.ID, name)
	}

	if err := SetDependencies(database, apps["api"].ID, []string{apps["worker"].ID}); err != nil {
		t.Fatalf("api depends on worker: %v", err)
	}
	if err := SetDependencies(database, apps["worker"].ID, []string{apps["frontend"].ID}); err != nil {
		t.Fatalf("worker depends on frontend: %v", err)
	}
	if err := SetDependencies(database, apps["frontend"].ID, []string{apps["api"].ID}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("err = %v, want the three-application cycle refused", err)
	}
}

func TestDependenciesRefuseThemselves(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "api")
	api := addApplication(t, database, project.ID, "api")

	if err := SetDependencies(database, api.ID, []string{api.ID}); !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("err = %v, want the self-dependency refusal", err)
	}
}

func TestDependenciesRefuseAnotherProject(t *testing.T) {
	database := openDatabase(t)
	first := newProject(t, database)
	second := newProject(t, database)
	makeFolder(t, first, "api")
	makeFolder(t, second, "web")
	api := addApplication(t, database, first.ID, "api")
	web := addApplication(t, database, second.ID, "web")

	if err := SetDependencies(database, api.ID, []string{web.ID}); !errors.Is(err, ErrDependencyOutsideProject) {
		t.Fatalf("err = %v, want the outside-project refusal", err)
	}
}

func TestDependenciesAreReplacedNotAppended(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	for _, directory := range []string{"api", "worker", "frontend"} {
		makeFolder(t, project, directory)
	}
	api := addApplication(t, database, project.ID, "api")
	worker := addApplication(t, database, project.ID, "worker")
	frontend := addApplication(t, database, project.ID, "frontend")

	if err := SetDependencies(database, frontend.ID, []string{api.ID, worker.ID}); err != nil {
		t.Fatalf("set dependencies: %v", err)
	}
	if err := SetDependencies(database, frontend.ID, []string{worker.ID}); err != nil {
		t.Fatalf("replace dependencies: %v", err)
	}

	stored, err := Get(database, frontend.ID)
	if err != nil {
		t.Fatalf("get frontend: %v", err)
	}
	if len(stored.DependsOn) != 1 || stored.DependsOn[0] != worker.ID {
		t.Fatalf("dependencies = %v, want only the replacement", stored.DependsOn)
	}
}

func TestRemovingAnApplicationDropsItsEdges(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "api")
	makeFolder(t, project, "worker")
	api := addApplication(t, database, project.ID, "api")
	worker := addApplication(t, database, project.ID, "worker")

	if err := SetDependencies(database, worker.ID, []string{api.ID}); err != nil {
		t.Fatalf("set dependencies: %v", err)
	}
	if err := Remove(database, api.ID); err != nil {
		t.Fatalf("remove api: %v", err)
	}

	// The dependent application must not keep a dependency on a record that is
	// gone, or the graph would describe something that cannot be started.
	stored, err := Get(database, worker.ID)
	if err != nil {
		t.Fatalf("get worker: %v", err)
	}
	if len(stored.DependsOn) != 0 {
		t.Fatalf("worker dependencies = %v, want none", stored.DependsOn)
	}
}

func TestHasCycleReadsEdgesDirectly(t *testing.T) {
	cases := []struct {
		name  string
		edges map[string][]string
		cycle bool
	}{
		{"empty", map[string][]string{}, false},
		{"chain", map[string][]string{"a": {"b"}, "b": {"c"}}, false},
		{"self", map[string][]string{"a": {"a"}}, true},
		{"pair", map[string][]string{"a": {"b"}, "b": {"a"}}, true},
		{"triangle", map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, true},
		{"diamond", map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d"}}, false},
	}
	for _, testCase := range cases {
		if got := hasCycle(testCase.edges); got != testCase.cycle {
			t.Errorf("%s: hasCycle = %v, want %v", testCase.name, got, testCase.cycle)
		}
	}
}
