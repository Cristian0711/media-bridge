package hardlink

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProgressCloneKeepsEmptySlicesNonNil guards a regression where clone()
// turned empty Done/Remaining slices into nil → JSON null, crashing the
// frontend (which reads done.length / remaining.length). They must stay arrays.
func TestProgressCloneKeepsEmptySlicesNonNil(t *testing.T) {
	p := &Progress{
		Total:     1,
		Linked:    1,
		Complete:  true,
		Done:      []Artifact{{Name: "a.mkv", Size: 1, Linked: true}},
		Remaining: []Artifact{}, // empty, but non-nil
	}

	c := p.clone()
	if c.Remaining == nil {
		t.Fatal("clone() must keep Remaining a non-nil empty slice, got nil")
	}
	if c.Done == nil {
		t.Fatal("clone() must keep Done non-nil")
	}

	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"remaining":[]`) {
		t.Fatalf("remaining should serialize as [], got: %s", out)
	}
	if strings.Contains(string(out), `"remaining":null`) {
		t.Fatalf("remaining must not be null: %s", out)
	}

	// Independent copies — mutating the clone must not touch the original.
	c.Done = append(c.Done, Artifact{Name: "b.mkv"})
	if len(p.Done) != 1 {
		t.Fatalf("clone shares backing array with original: original Done len = %d", len(p.Done))
	}
}
