package tools

import (
	"context"
	"encoding/json"
	"fmt"

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
		Description: "Retrieve a working note by its key. Use this to check current state of meal plans, shopping lists, goals, etc.",
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
}

// GetNoteKeys returns all note keys for inclusion in system context
func GetNoteKeys(memory *sheldonmem.Store) ([]string, error) {
	return memory.ListNotes()
}
