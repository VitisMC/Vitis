//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOffsetsRetainEmptyParents prevents subclass metadata from overwriting inherited fields.
func TestOffsetsRetainEmptyParents(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Entity.java":  `public class Entity { EntityDataAccessor<Byte> DATA_FLAGS = SynchedEntityData.defineId(Entity.class, EntityDataSerializers.BYTE); }`,
		"Mob.java":     `public class Mob extends Entity { EntityDataAccessor<Byte> DATA_MOB_FLAGS = SynchedEntityData.defineId(Mob.class, EntityDataSerializers.BYTE); }`,
		"Monster.java": `public abstract class Monster extends Mob { }`,
		"Creeper.java": `public class Creeper extends Monster { EntityDataAccessor<Integer> DATA_SWELL = SynchedEntityData.defineId(Creeper.class, EntityDataSerializers.INT); }`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	classes, err := scanEntityClasses(root)
	if err != nil {
		t.Fatal(err)
	}
	computeOffsets(classes)
	if classes["Creeper"].BaseOffset != 2 {
		t.Fatalf("creeper base offset = %d", classes["Creeper"].BaseOffset)
	}
}
