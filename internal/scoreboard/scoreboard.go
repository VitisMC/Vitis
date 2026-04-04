package scoreboard

import (
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

const (
	DisplaySlotList      int32 = 0
	DisplaySlotSidebar   int32 = 1
	DisplaySlotBelowName int32 = 2
)

const (
	RenderTypeInteger int32 = 0
	RenderTypeHearts  int32 = 1
)

type PacketBroadcaster interface {
	Send(pkt protocol.Packet) error
}

type Objective struct {
	Name        string
	DisplayName string
	RenderType  int32
}

type Score struct {
	EntityName    string
	ObjectiveName string
	Value         int32
}

type Team struct {
	Name              string
	DisplayName       string
	Prefix            string
	Suffix            string
	FriendlyFire      bool
	SeeInvisible      bool
	NameTagVisibility string
	CollisionRule     string
	Color             int32
	Members           map[string]struct{}
}

type Scoreboard struct {
	mu         sync.RWMutex
	objectives map[string]*Objective
	scores     map[string]map[string]*Score // objectiveName -> entityName -> Score
	displays   map[int32]string             // slot -> objectiveName
	teams      map[string]*Team
	broadcast  func(protocol.Packet)
}

func New(broadcast func(protocol.Packet)) *Scoreboard {
	return &Scoreboard{
		objectives: make(map[string]*Objective),
		scores:     make(map[string]map[string]*Score),
		displays:   make(map[int32]string),
		teams:      make(map[string]*Team),
		broadcast:  broadcast,
	}
}

func (sb *Scoreboard) AddObjective(name, displayName string, renderType int32) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if _, exists := sb.objectives[name]; exists {
		return false
	}
	sb.objectives[name] = &Objective{Name: name, DisplayName: displayName, RenderType: renderType}
	sb.scores[name] = make(map[string]*Score)
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.ScoreboardObjective{
			Name:  name,
			Mode:  0,
			Value: displayName,
			Type:  renderType,
		})
	}
	return true
}

func (sb *Scoreboard) RemoveObjective(name string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if _, exists := sb.objectives[name]; !exists {
		return false
	}
	delete(sb.objectives, name)
	delete(sb.scores, name)
	for slot, obj := range sb.displays {
		if obj == name {
			delete(sb.displays, slot)
		}
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.ScoreboardObjective{
			Name: name,
			Mode: 1,
		})
	}
	return true
}

func (sb *Scoreboard) SetDisplaySlot(slot int32, objectiveName string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if objectiveName == "" {
		delete(sb.displays, slot)
	} else {
		sb.displays[slot] = objectiveName
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.DisplayObjective{
			Position:  slot,
			ScoreName: objectiveName,
		})
	}
}

func (sb *Scoreboard) SetScore(objectiveName, entityName string, value int32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	objScores, ok := sb.scores[objectiveName]
	if !ok {
		return
	}
	objScores[entityName] = &Score{
		EntityName:    entityName,
		ObjectiveName: objectiveName,
		Value:         value,
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateScore{
			EntityName:    entityName,
			ObjectiveName: objectiveName,
			Value:         value,
		})
	}
}

func (sb *Scoreboard) ResetScore(objectiveName, entityName string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if objScores, ok := sb.scores[objectiveName]; ok {
		delete(objScores, entityName)
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.ResetScore{
			EntityName:   entityName,
			HasObjective: true,
			ObjectiveName: objectiveName,
		})
	}
}

func (sb *Scoreboard) ResetAllScores(entityName string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for _, objScores := range sb.scores {
		delete(objScores, entityName)
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.ResetScore{
			EntityName:   entityName,
			HasObjective: false,
		})
	}
}

func (sb *Scoreboard) GetScore(objectiveName, entityName string) (int32, bool) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	objScores, ok := sb.scores[objectiveName]
	if !ok {
		return 0, false
	}
	s, ok := objScores[entityName]
	if !ok {
		return 0, false
	}
	return s.Value, true
}

func (sb *Scoreboard) ListObjectives() []string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	names := make([]string, 0, len(sb.objectives))
	for n := range sb.objectives {
		names = append(names, n)
	}
	return names
}

func (sb *Scoreboard) ListScores(objectiveName string) map[string]int32 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	objScores, ok := sb.scores[objectiveName]
	if !ok {
		return nil
	}
	out := make(map[string]int32, len(objScores))
	for name, s := range objScores {
		out[name] = s.Value
	}
	return out
}

func (sb *Scoreboard) CreateTeam(name, displayName string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if _, exists := sb.teams[name]; exists {
		return false
	}
	t := &Team{
		Name:              name,
		DisplayName:       displayName,
		NameTagVisibility: "always",
		CollisionRule:     "always",
		Color:             -1,
		Members:           make(map[string]struct{}),
	}
	sb.teams[name] = t
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateTeams{
			TeamName:          name,
			Action:            playpacket.TeamActionCreate,
			DisplayName:       displayName,
			FriendlyFlags:     teamFlags(t),
			NameTagVisibility: t.NameTagVisibility,
			CollisionRule:     t.CollisionRule,
			Color:             teamColor(t),
		})
	}
	return true
}

func (sb *Scoreboard) RemoveTeam(name string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if _, exists := sb.teams[name]; !exists {
		return false
	}
	delete(sb.teams, name)
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateTeams{
			TeamName: name,
			Action:   playpacket.TeamActionRemove,
		})
	}
	return true
}

func (sb *Scoreboard) TeamAddMembers(teamName string, members []string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	t, ok := sb.teams[teamName]
	if !ok {
		return false
	}
	for _, m := range members {
		t.Members[m] = struct{}{}
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateTeams{
			TeamName: teamName,
			Action:   playpacket.TeamActionAddEntities,
			Entities: members,
		})
	}
	return true
}

func (sb *Scoreboard) TeamRemoveMembers(teamName string, members []string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	t, ok := sb.teams[teamName]
	if !ok {
		return false
	}
	for _, m := range members {
		delete(t.Members, m)
	}
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateTeams{
			TeamName: teamName,
			Action:   playpacket.TeamActionRemEntities,
			Entities: members,
		})
	}
	return true
}

func (sb *Scoreboard) UpdateTeam(name string, fn func(t *Team)) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	t, ok := sb.teams[name]
	if !ok {
		return false
	}
	fn(t)
	if sb.broadcast != nil {
		sb.broadcast(&playpacket.UpdateTeams{
			TeamName:          name,
			Action:            playpacket.TeamActionUpdateInfo,
			DisplayName:       t.DisplayName,
			FriendlyFlags:     teamFlags(t),
			NameTagVisibility: t.NameTagVisibility,
			CollisionRule:     t.CollisionRule,
			Color:             teamColor(t),
			Prefix:            t.Prefix,
			Suffix:            t.Suffix,
		})
	}
	return true
}

func (sb *Scoreboard) GetTeam(name string) *Team {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.teams[name]
}

func (sb *Scoreboard) ListTeams() []string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	names := make([]string, 0, len(sb.teams))
	for n := range sb.teams {
		names = append(names, n)
	}
	return names
}

func (sb *Scoreboard) SendInitTo(sender PacketBroadcaster) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	for _, obj := range sb.objectives {
		_ = sender.Send(&playpacket.ScoreboardObjective{
			Name:  obj.Name,
			Mode:  0,
			Value: obj.DisplayName,
			Type:  obj.RenderType,
		})
	}
	for slot, objName := range sb.displays {
		_ = sender.Send(&playpacket.DisplayObjective{
			Position:  slot,
			ScoreName: objName,
		})
	}
	for _, objScores := range sb.scores {
		for _, s := range objScores {
			_ = sender.Send(&playpacket.UpdateScore{
				EntityName:    s.EntityName,
				ObjectiveName: s.ObjectiveName,
				Value:         s.Value,
			})
		}
	}
	for _, t := range sb.teams {
		members := make([]string, 0, len(t.Members))
		for m := range t.Members {
			members = append(members, m)
		}
		_ = sender.Send(&playpacket.UpdateTeams{
			TeamName:          t.Name,
			Action:            playpacket.TeamActionCreate,
			DisplayName:       t.DisplayName,
			FriendlyFlags:     teamFlags(t),
			NameTagVisibility: t.NameTagVisibility,
			CollisionRule:     t.CollisionRule,
			Color:             teamColor(t),
			Prefix:            t.Prefix,
			Suffix:            t.Suffix,
			Entities:          members,
		})
	}
}

func teamFlags(t *Team) int8 {
	var f int8
	if t.FriendlyFire {
		f |= 0x01
	}
	if t.SeeInvisible {
		f |= 0x02
	}
	return f
}

func teamColor(t *Team) int32 {
	if t.Color < 0 {
		return 21 // reset
	}
	return t.Color
}
