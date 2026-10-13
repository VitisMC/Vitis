package crafting

import "testing"

// TestRecipeBookLifecycle verifies settings, unlock, seen, and removal behavior.
func TestRecipeBookLifecycle(t *testing.T) {
	book := NewRecipeBook()
	if err := book.SetSettings(BookBlastFurnace, true, true); err != nil {
		t.Fatal(err)
	}
	settings, ok := book.Settings(BookBlastFurnace)
	if !ok || !settings.Open || !settings.Filtering {
		t.Fatalf("settings = %+v", settings)
	}
	if added := book.Unlock(8, 3, 8, -1); len(added) != 2 || added[0] != 3 || added[1] != 8 {
		t.Fatalf("added = %v", added)
	}
	if !book.MarkSeen(3) || book.MarkSeen(3) || book.MarkSeen(4) {
		t.Fatal("invalid seen transition")
	}
	if removed := book.Remove(8, 4); len(removed) != 1 || removed[0] != 8 {
		t.Fatalf("removed = %v", removed)
	}
	if known := book.Known(); len(known) != 1 || known[0] != 3 {
		t.Fatalf("known = %v", known)
	}
}

// TestRecipeBookRejectsInvalidType verifies category bounds.
func TestRecipeBookRejectsInvalidType(t *testing.T) {
	if err := NewRecipeBook().SetSettings(BookType(4), false, false); err == nil {
		t.Fatal("accepted invalid recipe book type")
	}
}
