package sheldonmem

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

func TestSheldonEntitySeeded(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	sheldon, err := store.FindEntityByName("Sheldon")
	if err != nil {
		t.Fatalf("failed to find Sheldon entity: %v", err)
	}

	if sheldon.EntityType != "agent" {
		t.Errorf("expected 'agent', got '%s'", sheldon.EntityType)
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

	sheldon, _ := store.FindEntityByName("Sheldon")
	kadet, _ := store.CreateEntity("Kadet", "person", 1, "")

	edge, err := store.AddEdge(sheldon.ID, kadet.ID, "serves", 1.0, "")
	if err != nil {
		t.Fatalf("failed to add edge: %v", err)
	}

	if edge.Relation != "serves" {
		t.Errorf("expected 'serves', got '%s'", edge.Relation)
	}

	edges, _ := store.GetEdgesFrom(sheldon.ID)
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

func TestTraverse(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	// Create entities
	kadet, _ := store.CreateEntity("Kadet", "person", 1, "")
	sarah, _ := store.CreateEntity("Sarah", "person", 6, "")
	mike, _ := store.CreateEntity("Mike", "person", 6, "")

	// Add facts
	store.AddFact(&kadet.ID, 1, "occupation", "engineer", 0.9)
	store.AddFact(&sarah.ID, 6, "relationship", "wife", 0.95)
	store.AddFact(&sarah.ID, 2, "birthday", "March 15", 0.9)
	store.AddFact(&mike.ID, 6, "relationship", "friend", 0.8)

	// Create edges
	store.AddEdge(kadet.ID, sarah.ID, "married_to", 1.0, "")
	store.AddEdge(kadet.ID, mike.ID, "friends_with", 0.8, "")

	// Traverse from Kadet with depth 1
	results, err := store.Traverse(kadet.ID, 1)
	if err != nil {
		t.Fatalf("failed to traverse: %v", err)
	}

	// Should have 3 entities: Kadet, Sarah, Mike
	if len(results) != 3 {
		t.Errorf("expected 3 traversal results, got %d", len(results))
	}

	// First should be Kadet (depth 0)
	if results[0].Entity.Name != "Kadet" {
		t.Errorf("expected first entity to be Kadet, got %s", results[0].Entity.Name)
	}
	if results[0].Depth != 0 {
		t.Errorf("expected depth 0, got %d", results[0].Depth)
	}

	// Check that Sarah has facts
	var sarahResult *TraversalResult
	for _, r := range results {
		if r.Entity.Name == "Sarah" {
			sarahResult = r
			break
		}
	}

	if sarahResult == nil {
		t.Fatal("Sarah not found in traversal")
	}

	if len(sarahResult.Facts) != 2 {
		t.Errorf("expected Sarah to have 2 facts, got %d", len(sarahResult.Facts))
	}
}

func TestSearchEntities(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	store.CreateEntity("Sarah Johnson", "person", 6, "")
	store.CreateEntity("Sarah Smith", "person", 6, "")
	store.CreateEntity("Mike Brown", "person", 6, "")

	results, err := store.SearchEntities("Sarah")
	if err != nil {
		t.Fatalf("failed to search entities: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for 'Sarah', got %d", len(results))
	}
}

func TestDecay(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	entity, _ := store.CreateEntity("Test", "person", 1, "")

	// Add facts with varying confidence
	store.AddFact(&entity.ID, 1, "low_conf", "value1", 0.3)
	store.AddFact(&entity.ID, 1, "high_conf", "value2", 0.9)

	// Manually backdate the low confidence fact
	store.DB().Exec(`UPDATE facts SET created_at = datetime('now', '-1 year') WHERE field = 'low_conf'`)

	// Run decay with default config
	deleted, err := store.Decay(DefaultDecayConfig)
	if err != nil {
		t.Fatalf("decay failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// High confidence fact should remain
	facts, _ := store.GetFactsByEntity(entity.ID)
	if len(facts) != 1 {
		t.Errorf("expected 1 remaining fact, got %d", len(facts))
	}

	if facts[0].Field != "high_conf" {
		t.Errorf("expected high_conf to remain, got %s", facts[0].Field)
	}
}
