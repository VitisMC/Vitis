package effect

import (
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.Count() != 0 {
		t.Errorf("expected 0 effects, got %d", m.Count())
	}
}

func TestAddAndHas(t *testing.T) {
	m := NewManager()
	ok := m.Add(Instance{
		ID:        effectdata.EffectSpeed,
		Amplifier: 0,
		Duration:  200,
		Flags:     DefaultFlags(),
	})
	if !ok {
		t.Error("expected Add to return true")
	}
	if !m.Has(effectdata.EffectSpeed) {
		t.Error("expected Has(Speed) to be true")
	}
	if m.Has(effectdata.EffectSlowness) {
		t.Error("expected Has(Slowness) to be false")
	}
	if m.Count() != 1 {
		t.Errorf("expected 1 effect, got %d", m.Count())
	}
}

func TestAddReplacesStronger(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})

	ok := m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 1, Duration: 100, Flags: DefaultFlags()})
	if !ok {
		t.Error("expected stronger effect to replace weaker")
	}

	inst := m.Get(effectdata.EffectSpeed)
	if inst.Amplifier != 1 {
		t.Errorf("expected amplifier 1, got %d", inst.Amplifier)
	}
}

func TestAddRejectsWeaker(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 1, Duration: 200, Flags: DefaultFlags()})

	ok := m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 300, Flags: DefaultFlags()})
	if ok {
		t.Error("expected weaker effect to be rejected")
	}

	inst := m.Get(effectdata.EffectSpeed)
	if inst.Amplifier != 1 {
		t.Errorf("expected amplifier to remain 1, got %d", inst.Amplifier)
	}
}

func TestAddReplacesLongerSameLevel(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 100, Flags: DefaultFlags()})

	ok := m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})
	if !ok {
		t.Error("expected longer duration to replace shorter at same amplifier")
	}

	inst := m.Get(effectdata.EffectSpeed)
	if inst.Duration != 200 {
		t.Errorf("expected duration 200, got %d", inst.Duration)
	}
}

func TestRemove(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})

	ok := m.Remove(effectdata.EffectSpeed)
	if !ok {
		t.Error("expected Remove to return true")
	}
	if m.Has(effectdata.EffectSpeed) {
		t.Error("expected effect to be removed")
	}

	ok = m.Remove(effectdata.EffectSpeed)
	if ok {
		t.Error("expected Remove of non-existent to return false")
	}
}

func TestClear(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})
	m.Add(Instance{ID: effectdata.EffectSlowness, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})

	ids := m.Clear()
	if len(ids) != 2 {
		t.Errorf("expected 2 cleared IDs, got %d", len(ids))
	}
	if m.Count() != 0 {
		t.Errorf("expected 0 effects after clear, got %d", m.Count())
	}
}

func TestTickExpiration(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 3, Flags: DefaultFlags()})

	m.Tick()
	if !m.Has(effectdata.EffectSpeed) {
		t.Error("expected effect still active after 1 tick")
	}

	m.Tick()
	if !m.Has(effectdata.EffectSpeed) {
		t.Error("expected effect still active after 2 ticks")
	}

	diff := m.Tick()
	if m.Has(effectdata.EffectSpeed) {
		t.Error("expected effect to expire after 3 ticks")
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != effectdata.EffectSpeed {
		t.Error("expected Speed in removed list")
	}
}

func TestTickRegeneration(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectRegeneration, Amplifier: 0, Duration: 100, Flags: DefaultFlags()})

	var totalHeal float32
	for i := 0; i < 100; i++ {
		diff := m.Tick()
		for _, a := range diff.Actions {
			totalHeal += a.Heal
		}
	}

	if totalHeal < 1.0 {
		t.Errorf("expected at least 1.0 heal from regeneration, got %f", totalHeal)
	}
}

func TestTickPoison(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectPoison, Amplifier: 0, Duration: 100, Flags: DefaultFlags()})

	var totalDamage float32
	for i := 0; i < 100; i++ {
		diff := m.Tick()
		for _, a := range diff.Actions {
			totalDamage += a.Damage
		}
	}

	if totalDamage < 1.0 {
		t.Errorf("expected at least 1.0 damage from poison, got %f", totalDamage)
	}
}

func TestApplyInstantHealth(t *testing.T) {
	action := ApplyInstant(effectdata.EffectInstantHealth, 0)
	if action == nil {
		t.Fatal("expected non-nil action")
	}
	if action.Heal != 4.0 {
		t.Errorf("expected 4.0 heal, got %f", action.Heal)
	}

	action2 := ApplyInstant(effectdata.EffectInstantHealth, 1)
	if action2.Heal != 8.0 {
		t.Errorf("expected 8.0 heal at amplifier 1, got %f", action2.Heal)
	}
}

func TestApplyInstantDamage(t *testing.T) {
	action := ApplyInstant(effectdata.EffectInstantDamage, 0)
	if action == nil {
		t.Fatal("expected non-nil action")
	}
	if action.Damage != 6.0 {
		t.Errorf("expected 6.0 damage, got %f", action.Damage)
	}
}

func TestIsInstant(t *testing.T) {
	if !IsInstant(effectdata.EffectInstantHealth) {
		t.Error("expected InstantHealth to be instant")
	}
	if !IsInstant(effectdata.EffectInstantDamage) {
		t.Error("expected InstantDamage to be instant")
	}
	if IsInstant(effectdata.EffectSpeed) {
		t.Error("expected Speed to not be instant")
	}
}

func TestActive(t *testing.T) {
	m := NewManager()
	m.Add(Instance{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 200, Flags: DefaultFlags()})
	m.Add(Instance{ID: effectdata.EffectStrength, Amplifier: 1, Duration: 100, Flags: DefaultFlags()})

	active := m.Active()
	if len(active) != 2 {
		t.Errorf("expected 2 active effects, got %d", len(active))
	}
}

func TestGetNonExistent(t *testing.T) {
	m := NewManager()
	inst := m.Get(effectdata.EffectSpeed)
	if inst != nil {
		t.Error("expected nil for non-existent effect")
	}
}
