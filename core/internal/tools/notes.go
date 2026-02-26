package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldonmem"
)

type SaveNoteArgs struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type GetNoteArgs struct {
	Key string `json:"key"`
}

type DeleteNoteArgs struct {
	Key string `json:"key"`
}

type ArchiveNoteArgs struct {
	OldKey string `json:"old_key"`
	NewKey string `json:"new_key"`
}

type ListArchivedNotesArgs struct {
	Pattern string `json:"pattern"`
}

type RestoreNoteArgs struct {
	Key string `json:"key"`
}

type GetNotesArgs struct {
	Keys []string `json:"keys"`
}

func RegisterNoteTools(registry *Registry, memory *sheldonmem.Store) {
	saveNoteTool := llm.Tool{
		Name: "save_note",
		Description: `Save or update a working note. Notes are for dynamic, temporary state that changes frequently.

Use notes for:
- Meal plans for the week (gets updated as meals are cooked)
- Shopping lists (items get crossed off)
- Weekly goals (progress tracked)
- Project status (current state of something)
- Any "current state" that doesn't fit long-term memory

Notes are key-value: the key identifies the note, content can be text, markdown, or JSON.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Identifier for the note (e.g., 'meal_plan', 'shopping_list', 'weekly_goals')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The note content - can be plain text, markdown, or JSON for structured data",
				},
			},
			"required": []string{"key", "content"},
		},
	}

	registry.Register(saveNoteTool, func(ctx context.Context, args string) (string, error) {
		var params SaveNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := memory.SaveNote(params.Key, params.Content); err != nil {
			return "", fmt.Errorf("failed to save note: %w", err)
		}

		return fmt.Sprintf("Saved note: %s", params.Key), nil
	})

	getNoteTool := llm.Tool{
		Name:        "get_note",
		Description: "Retrieve a note by its key. Searches both working notes and archived notes. Use this to check current state or retrieve historical data.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "The note key to retrieve",
				},
			},
			"required": []string{"key"},
		},
	}

	registry.Register(getNoteTool, func(ctx context.Context, args string) (string, error) {
		var params GetNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		note, err := memory.GetNote(params.Key)
		if err != nil {
			return "", fmt.Errorf("failed to get note: %w", err)
		}

		if note == nil {
			return fmt.Sprintf("No note found with key '%s'", params.Key), nil
		}

		return note.Content, nil
	})

	getNotesTool := llm.Tool{
		Name: "get_notes",
		Description: `Retrieve multiple notes at once. Use this instead of multiple get_note calls when you need several notes.

Examples:
- Comparing budgets: get_notes(["budget_2025_01", "budget_2025_02", "budget_2025_03"])
- Building a report: get_notes(["meal_plan", "shopping_list", "weekly_goals"])
- Exporting history: first list_archived_notes("budget"), then get_notes with those keys`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keys": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of note keys to retrieve",
				},
			},
			"required": []string{"keys"},
		},
	}

	registry.Register(getNotesTool, func(ctx context.Context, args string) (string, error) {
		var params GetNotesArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if len(params.Keys) == 0 {
			return "No keys provided", nil
		}

		notes, err := memory.GetNotes(params.Keys)
		if err != nil {
			return "", fmt.Errorf("failed to get notes: %w", err)
		}

		if len(notes) == 0 {
			return "No notes found for the provided keys", nil
		}

		// Format as key: content pairs
		var result strings.Builder
		for i, note := range notes {
			if i > 0 {
				result.WriteString("\n---\n")
			}
			result.WriteString(fmt.Sprintf("## %s\n%s", note.Key, note.Content))
		}
		return result.String(), nil
	})

	deleteNoteTool := llm.Tool{
		Name:        "delete_note",
		Description: "Delete a working note when it's no longer needed (e.g., week is over, list is complete).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "The note key to delete",
				},
			},
			"required": []string{"key"},
		},
	}

	registry.Register(deleteNoteTool, func(ctx context.Context, args string) (string, error) {
		var params DeleteNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := memory.DeleteNote(params.Key); err != nil {
			return "", fmt.Errorf("failed to delete note: %w", err)
		}

		return fmt.Sprintf("Deleted note: %s", params.Key), nil
	})

	archiveNoteTool := llm.Tool{
		Name: "archive_note",
		Description: `Archive a working note for long-term storage. Use this at natural endpoints like end of week/month.

The archived note is removed from Active Notes (system prompt) but remains retrievable via get_note.
Provide a descriptive new_key that includes context (e.g., 'budget_2025_01' for January budget).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"old_key": map[string]any{
					"type":        "string",
					"description": "The current working note key to archive",
				},
				"new_key": map[string]any{
					"type":        "string",
					"description": "The archive key (e.g., 'budget_2025_01', 'meal_plan_week_08')",
				},
			},
			"required": []string{"old_key", "new_key"},
		},
	}

	registry.Register(archiveNoteTool, func(ctx context.Context, args string) (string, error) {
		var params ArchiveNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := memory.ArchiveNote(params.OldKey, params.NewKey); err != nil {
			return "", fmt.Errorf("failed to archive note: %w", err)
		}

		return fmt.Sprintf("Archived '%s' as '%s'", params.OldKey, params.NewKey), nil
	})

	listArchivedNotesTool := llm.Tool{
		Name:        "list_archived_notes",
		Description: "List archived notes, optionally filtered by a search pattern. Use this to find historical data like past budgets or old meal plans.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Optional search pattern (e.g., 'budget' to find all budget archives, '2025_01' for January 2025)",
				},
			},
		},
	}

	registry.Register(listArchivedNotesTool, func(ctx context.Context, args string) (string, error) {
		var params ListArchivedNotesArgs
		if args != "" && args != "{}" {
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}

		notes, err := memory.ListArchivedNotes(params.Pattern)
		if err != nil {
			return "", fmt.Errorf("failed to list archived notes: %w", err)
		}

		if len(notes) == 0 {
			if params.Pattern != "" {
				return fmt.Sprintf("No archived notes matching '%s'", params.Pattern), nil
			}
			return "No archived notes", nil
		}

		result := "Archived notes:\n"
		for _, n := range notes {
			result += fmt.Sprintf("- %s (archived %s)\n", n.Key, n.UpdatedAt.Format("2006-01-02"))
		}
		return result, nil
	})

	restoreNoteTool := llm.Tool{
		Name:        "restore_note",
		Description: "Restore an archived note back to working notes. Use this to bring back historical data for active use.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "The archived note key to restore",
				},
			},
			"required": []string{"key"},
		},
	}

	registry.Register(restoreNoteTool, func(ctx context.Context, args string) (string, error) {
		var params RestoreNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := memory.RestoreNote(params.Key); err != nil {
			return "", fmt.Errorf("failed to restore note: %w", err)
		}

		return fmt.Sprintf("Restored '%s' to working notes", params.Key), nil
	})
}

// GetNoteKeys returns all note keys for inclusion in system context
func GetNoteKeys(memory *sheldonmem.Store) ([]string, error) {
	return memory.ListNotes()
}
