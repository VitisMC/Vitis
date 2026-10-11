package chunk

import "testing"

// TestTicketsSeparateVisibilityFromSimulation covers ticking rings and independent owners.
func TestTicketsSeparateVisibilityFromSimulation(t *testing.T) {
	a := NewActivityTracker()
	if err := a.Set(Ticket{Kind: TicketView, Owner: 1, X: -2, Z: -3, Radius: 8}); err != nil {
		t.Fatal(err)
	}
	if err := a.Set(Ticket{Kind: TicketPlayer, Owner: 1, X: -2, Z: -3, Radius: 2}); err != nil {
		t.Fatal(err)
	}
	a.Update(10)
	for _, test := range []struct {
		x    int32
		want Activity
	}{{-2, ActivityEntityTicking}, {0, ActivityEntityTicking}, {1, ActivityBlockTicking}, {2, ActivityLoaded}, {6, ActivityLoaded}, {7, ActivityUnloaded}} {
		if got := a.At(test.x, -3); got != test.want {
			t.Fatalf("x=%d: got %v, want %v", test.x, got, test.want)
		}
	}
	a.Remove(TicketPlayer, 1)
	a.Update(11)
	if a.At(-2, -3) != ActivityLoaded {
		t.Fatal("removing simulation ticket removed visibility")
	}
	if err := a.Set(Ticket{Kind: TicketPortal, Owner: 2, X: -2, Z: -3, ExpiresAt: 13}); err != nil {
		t.Fatal(err)
	}
	a.Update(12)
	if a.At(-2, -3) != ActivityEntityTicking {
		t.Fatal("portal did not activate chunk")
	}
	a.Update(13)
	if a.At(-2, -3) != ActivityLoaded {
		t.Fatal("expired portal ticket survived")
	}
}

// TestTicketValidation rejects unbounded membership and invalid ticket kinds.
func TestTicketValidation(t *testing.T) {
	a := NewActivityTracker()
	for _, ticket := range []Ticket{{Radius: -1}, {Radius: 97}, {Kind: TicketKind(255)}, {X: 1875000}} {
		if a.Set(ticket) == nil {
			t.Fatalf("accepted invalid ticket %+v", ticket)
		}
	}
}
