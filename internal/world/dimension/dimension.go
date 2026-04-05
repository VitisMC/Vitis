package dimension

// Type represents a Minecraft dimension type with its properties.
type Type struct {
	Name             string
	MinY             int32
	Height           int32
	LogicalHeight    int32
	HasSkylight      bool
	HasCeiling       bool
	Ultrawarm        bool
	Natural          bool
	CoordinateScale  float64
	BedWorks         bool
	RespawnAnchor    bool
	HasRaids         bool
	PiglinSafe       bool
	AmbientLight     float32
	Infiniburn       string
	Effects          string
}

var (
	Overworld = Type{
		Name:            "minecraft:overworld",
		MinY:            -64,
		Height:          384,
		LogicalHeight:   384,
		HasSkylight:     true,
		Natural:         true,
		CoordinateScale: 1.0,
		BedWorks:        true,
		HasRaids:        true,
		AmbientLight:    0.0,
		Infiniburn:      "minecraft:infiniburn_overworld",
		Effects:         "minecraft:overworld",
	}

	Nether = Type{
		Name:            "minecraft:the_nether",
		MinY:            0,
		Height:          256,
		LogicalHeight:   128,
		HasCeiling:      true,
		Ultrawarm:       true,
		CoordinateScale: 8.0,
		RespawnAnchor:   true,
		PiglinSafe:      true,
		AmbientLight:    0.1,
		Infiniburn:      "minecraft:infiniburn_nether",
		Effects:         "minecraft:the_nether",
	}

	End = Type{
		Name:            "minecraft:the_end",
		MinY:            0,
		Height:          256,
		LogicalHeight:   256,
		CoordinateScale: 1.0,
		HasRaids:        true,
		AmbientLight:    0.0,
		Infiniburn:      "minecraft:infiniburn_end",
		Effects:         "minecraft:the_end",
	}
)

// Manager holds the server's dimensions.
type Manager struct {
	dimensions map[string]*Type
	names      []string
}

// NewManager creates a dimension manager with the three default dimensions.
func NewManager() *Manager {
	m := &Manager{
		dimensions: make(map[string]*Type, 3),
	}
	m.Register(&Overworld)
	m.Register(&Nether)
	m.Register(&End)
	return m
}

// Register adds a dimension type.
func (m *Manager) Register(t *Type) {
	m.dimensions[t.Name] = t
	m.names = append(m.names, t.Name)
}

// Get returns a dimension by name.
func (m *Manager) Get(name string) (*Type, bool) {
	t, ok := m.dimensions[name]
	return t, ok
}

// Names returns all registered dimension names.
func (m *Manager) Names() []string {
	return m.names
}

// Count returns the number of registered dimensions.
func (m *Manager) Count() int {
	return len(m.dimensions)
}
