package koramem

import (
	"testing"
)

func TestOpenAndClose(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()
}

func TestDomainsSeeded(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	domain, err := store.GetDomain(1)
	if err != nil {
		t.Fatalf("failed to get domain: %v", err)
	}

	if domain.Name != "Identity & Self" {
		t.Errorf("expected 'Identity & Self', got '%s'", domain.Name)
	}

	if domain.Layer != "core" {
		t.Errorf("expected 'core', got '%s'", domain.Layer)
	}
}

func TestKoraEntitySeeded(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	kora, err := store.FindEntityByName("Kora")
	if err != nil {
		t.Fatalf("failed to find Kora entity: %v", err)
	}

	if kora.EntityType != "agent" {
		t.Errorf("expected 'agent', got '%s'", kora.EntityType)
	}
}

func TestCreateAndFindEntity(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	entity, err := store.CreateEntity("Kadet", "person", 1, `{"role":"user"}`)
	if err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	found, err := store.FindEntityByName("Kadet")
	if err != nil {
		t.Fatalf("failed to find entity: %v", err)
	}

	if found.ID != entity.ID {
		t.Errorf("expected ID %d, got %d", entity.ID, found.ID)
	}
}

func TestAddFact(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	entity, _ := store.CreateEntity("Kadet", "person", 1, "")
	entityID := entity.ID

	fact, err := store.AddFact(&entityID, 1, "name", "Kadet", 0.9)
	if err != nil {
		t.Fatalf("failed to add fact: %v", err)
	}

	if fact.Value != "Kadet" {
		t.Errorf("expected 'Kadet', got '%s'", fact.Value)
	}
}

func TestFactContradictionSupersedes(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	entity, _ := store.CreateEntity("Kadet", "person", 9, "")
	entityID := entity.ID

	fact1, _ := store.AddFact(&entityID, 9, "city", "Lagos", 0.9)
	fact2, _ := store.AddFact(&entityID, 9, "city", "Berlin", 0.9)

	if fact2.Supersedes == nil || *fact2.Supersedes != fact1.ID {
		t.Errorf("expected fact2 to supersede fact1")
	}

	facts, _ := store.GetFactsByEntity(entityID)
	if len(facts) != 1 {
		t.Errorf("expected 1 active fact, got %d", len(facts))
	}

	if facts[0].Value != "Berlin" {
		t.Errorf("expected 'Berlin', got '%s'", facts[0].Value)
	}
}

func TestAddEdge(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	kora, _ := store.FindEntityByName("Kora")
	kadet, _ := store.CreateEntity("Kadet", "person", 1, "")

	edge, err := store.AddEdge(kora.ID, kadet.ID, "serves", 1.0, "")
	if err != nil {
		t.Fatalf("failed to add edge: %v", err)
	}

	if edge.Relation != "serves" {
		t.Errorf("expected 'serves', got '%s'", edge.Relation)
	}

	edges, _ := store.GetEdgesFrom(kora.ID)
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}

func TestSqliteVecVersion(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	var vecVersion string
	err = store.DB().QueryRow("SELECT vec_version()").Scan(&vecVersion)
	if err != nil {
		t.Fatalf("vec_version() failed: %v", err)
	}

	if vecVersion == "" {
		t.Fatal("vec_version() returned empty string")
	}

	t.Logf("sqlite-vec version: %s", vecVersion)
}
