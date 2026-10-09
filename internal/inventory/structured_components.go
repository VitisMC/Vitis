package inventory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

//go:embed component_schema.json
var componentSchemaJSON []byte

// StructuredComponent stores the validated fields of a compound wire codec.
type StructuredComponent struct{ Data any }

// isComponentValue marks a structured component as an item patch value.
func (StructuredComponent) isComponentValue() {}

// RegistryHolder identifies a registry entry or owns its inline definition.
type RegistryHolder struct {
	ID   int32
	Data any
}

// RegistrySet represents a registry tag or an explicit entry list.
type RegistrySet struct {
	Tag string
	IDs []int32
}

// NetworkTag owns one unnamed NBT value.
type NetworkTag struct {
	Type  byte
	Value any
}

// wireSchema is the compiled, immutable form of a protocol component codec.
type wireSchema struct {
	kind         string
	fields       []wireField
	element      *wireSchema
	alternatives map[string]*wireSchema
	mappings     map[int32]string
	reverse      map[string]int32
	compare      string
	base         string
}

// wireField names one field of a component codec.
type wireField struct {
	name   string
	anon   bool
	schema *wireSchema
}

var componentSchemas = compileComponentSchemas()

// compileComponentSchemas resolves the pinned component schemas once at startup.
func compileComponentSchemas() map[int32]*wireSchema {
	var source struct {
		Components map[string]json.RawMessage
		Types      map[string]json.RawMessage
	}
	if err := json.Unmarshal(componentSchemaJSON, &source); err != nil {
		panic(fmt.Sprintf("invalid embedded component schemas: %v", err))
	}
	cache := map[string]*wireSchema{}
	var compile func(json.RawMessage) *wireSchema
	compile = func(raw json.RawMessage) *wireSchema {
		var name string
		if json.Unmarshal(raw, &name) == nil {
			if name == "string" || name == "varint" || name == "i32" || name == "i64" || name == "f32" || name == "f64" || name == "bool" || name == "void" || name == "Slot" || name == "anonymousNbt" || name == "anonOptionalNbt" || name == "position" || name == "UUID" {
				return &wireSchema{kind: name}
			}
			if cached := cache[name]; cached != nil {
				return cached
			}
			if definition, ok := source.Types[name]; ok {
				result := &wireSchema{}
				cache[name] = result
				*result = *compile(definition)
				return result
			}
			return &wireSchema{kind: name}
		}
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil || len(parts) < 2 {
			panic("invalid embedded component codec")
		}
		if err := json.Unmarshal(parts[0], &name); err != nil {
			panic(err)
		}
		result := &wireSchema{kind: name}
		switch name {
		case "container":
			var fields []struct {
				Name string
				Anon bool
				Type json.RawMessage
			}
			if err := json.Unmarshal(parts[1], &fields); err != nil {
				panic(err)
			}
			for _, field := range fields {
				result.fields = append(result.fields, wireField{field.Name, field.Anon, compile(field.Type)})
			}
		case "array":
			var spec struct{ Type json.RawMessage }
			if err := json.Unmarshal(parts[1], &spec); err != nil {
				panic(err)
			}
			result.element = compile(spec.Type)
		case "option":
			result.element = compile(parts[1])
		case "mapper":
			var spec struct {
				Type     json.RawMessage
				Mappings map[int32]string
			}
			if err := json.Unmarshal(parts[1], &spec); err != nil {
				panic(err)
			}
			result.element = compile(spec.Type)
			result.mappings = spec.Mappings
			result.reverse = map[string]int32{}
			for id, name := range spec.Mappings {
				result.reverse[name] = id
			}
		case "switch":
			var spec struct {
				CompareTo string
				Fields    map[string]json.RawMessage
			}
			if err := json.Unmarshal(parts[1], &spec); err != nil {
				panic(err)
			}
			result.compare = spec.CompareTo
			result.alternatives = map[string]*wireSchema{}
			for key, value := range spec.Fields {
				result.alternatives[key] = compile(value)
			}
		case "registryEntryHolder":
			var spec struct {
				BaseName  string
				Otherwise struct{ Type json.RawMessage }
			}
			if err := json.Unmarshal(parts[1], &spec); err != nil {
				panic(err)
			}
			result.base = spec.BaseName
			result.element = compile(spec.Otherwise.Type)
		case "registryEntryHolderSet":
			result.element = &wireSchema{kind: "varint"}
		default:
			panic("unsupported embedded codec " + name)
		}
		return result
	}
	result := map[int32]*wireSchema{}
	for name, raw := range source.Components {
		result[dc.DataComponentIDByName("minecraft:"+name)] = compile(raw)
	}
	return result
}

// decodeStructuredComponent decodes a bounded component field graph.
func decodeStructuredComponent(buf *protocol.Buffer, id int32, depth int) (ComponentValue, error) {
	schema := componentSchemas[id]
	if schema == nil {
		return nil, fmt.Errorf("unknown component codec %d", id)
	}
	budget := 16384
	value, err := readWireValue(buf, schema, nil, depth, 0, &budget)
	if err != nil {
		return nil, err
	}
	if err := validateStructuredFields(id, value, ""); err != nil {
		return nil, err
	}
	validationBudget := 16384
	if _, _, err := wireToNBT(schema, value, id, "", depth, 0, &validationBudget); err != nil {
		return nil, err
	}
	return StructuredComponent{Data: value}, nil
}

// encodeStructuredComponent encodes a component using its exact versioned schema.
func encodeStructuredComponent(buf *protocol.Buffer, id int32, value StructuredComponent, depth int) error {
	schema := componentSchemas[id]
	if schema == nil {
		return fmt.Errorf("unknown component codec %d", id)
	}
	budget := 16384
	return writeWireValue(buf, schema, value.Data, nil, depth, 0, &budget)
}

// readWireValue decodes one value while limiting allocations and recursion.
func readWireValue(buf *protocol.Buffer, s *wireSchema, ctx map[string]any, itemDepth, level int, budget *int) (any, error) {
	*budget--
	if *budget < 0 || level > 64 {
		return nil, fmt.Errorf("component graph exceeds limit")
	}
	read := func(child *wireSchema) (any, error) {
		return readWireValue(buf, child, ctx, itemDepth, level+1, budget)
	}
	switch s.kind {
	case "varint":
		return buf.ReadVarInt()
	case "i32":
		return buf.ReadInt32()
	case "i64", "position":
		return buf.ReadInt64()
	case "f32":
		v, e := buf.ReadFloat32()
		if e == nil && !finiteFloat(v) {
			e = fmt.Errorf("invalid float")
		}
		return v, e
	case "f64":
		v, e := buf.ReadFloat64()
		if e == nil && (math.IsNaN(v) || math.IsInf(v, 0)) {
			e = fmt.Errorf("invalid double")
		}
		return v, e
	case "bool":
		return buf.ReadBool()
	case "string":
		return buf.ReadStringLimit(32767)
	case "UUID":
		data, err := buf.ReadBytes(16)
		return bytes.Clone(data), err
	case "void":
		return nil, nil
	case "Slot":
		return decodeItemStack(buf, itemDepth+1)
	case "anonymousNbt", "anonOptionalNbt":
		d := componentNBTDecoder(buf.RemainingBytes())
		t, v, e := d.ReadRootTag()
		if e != nil {
			return nil, e
		}
		if d.Pos() > maxComponentNBTBytes {
			return nil, fmt.Errorf("NBT component exceeds limit")
		}
		if _, e = buf.ReadBytes(d.Pos()); e != nil {
			return nil, e
		}
		if t == nbt.TagEnd {
			if s.kind == "anonymousNbt" {
				return nil, fmt.Errorf("missing required NBT")
			}
			return nil, nil
		}
		return NetworkTag{t, v}, nil
	case "option":
		has, e := buf.ReadBool()
		if e != nil || !has {
			return nil, e
		}
		return read(s.element)
	case "array":
		n, e := readBoundedListCount(buf)
		if e != nil {
			return nil, e
		}
		result := make([]any, n)
		for i := range result {
			result[i], e = read(s.element)
			if e != nil {
				return nil, e
			}
		}
		return result, nil
	case "mapper":
		v, e := read(s.element)
		if e != nil {
			return nil, e
		}
		id, ok := v.(int32)
		if !ok {
			return nil, fmt.Errorf("invalid mapper value")
		}
		name, ok := s.mappings[id]
		if !ok {
			return nil, fmt.Errorf("invalid enum %d", id)
		}
		return name, nil
	case "container":
		m := map[string]any{}
		for _, field := range s.fields {
			v, e := readWireValue(buf, field.schema, m, itemDepth, level+1, budget)
			if e != nil {
				return nil, fmt.Errorf("field %s: %w", field.name, e)
			}
			if field.anon {
				if values, ok := v.(map[string]any); ok {
					for k, x := range values {
						m[k] = x
					}
				}
			} else {
				m[field.name] = v
			}
		}
		return m, nil
	case "switch":
		key := fmt.Sprint(ctx[s.compare])
		child := s.alternatives[key]
		if child == nil {
			return nil, fmt.Errorf("invalid union tag %s", key)
		}
		return read(child)
	case "registryEntryHolder":
		id, e := buf.ReadVarInt()
		if e != nil || id < 0 {
			return nil, fmt.Errorf("invalid registry holder")
		}
		h := RegistryHolder{ID: id - 1}
		if id == 0 {
			h.Data, e = read(s.element)
		}
		return h, e
	case "registryEntryHolderSet":
		n, e := buf.ReadVarInt()
		if e != nil || n < 0 || n > maxComponentMapEntries+1 {
			return nil, fmt.Errorf("invalid registry set")
		}
		result := RegistrySet{}
		if n == 0 {
			result.Tag, e = buf.ReadStringLimit(32767)
			return result, e
		}
		result.IDs = make([]int32, n-1)
		for i := range result.IDs {
			result.IDs[i], e = buf.ReadVarInt()
			if e != nil || result.IDs[i] < 0 {
				return nil, fmt.Errorf("invalid registry set id")
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported codec %s", s.kind)
	}
}

// writeWireValue encodes a validated value without reflecting over gameplay state.
func writeWireValue(buf *protocol.Buffer, s *wireSchema, value any, ctx map[string]any, itemDepth, level int, budget *int) error {
	*budget--
	if *budget < 0 || level > 64 {
		return fmt.Errorf("component graph exceeds limit")
	}
	write := func(child *wireSchema, v any) error {
		return writeWireValue(buf, child, v, ctx, itemDepth, level+1, budget)
	}
	switch s.kind {
	case "varint", "i32":
		v, ok := value.(int32)
		if !ok {
			return fmt.Errorf("expected integer")
		}
		if s.kind == "varint" {
			buf.WriteVarInt(v)
		} else {
			buf.WriteInt32(v)
		}
	case "i64", "position":
		v, ok := value.(int64)
		if !ok {
			return fmt.Errorf("expected long")
		}
		buf.WriteInt64(v)
	case "f32":
		v, ok := value.(float32)
		if !ok || !finiteFloat(v) {
			return fmt.Errorf("invalid float")
		}
		buf.WriteFloat32(v)
	case "f64":
		v, ok := value.(float64)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("invalid double")
		}
		buf.WriteFloat64(v)
	case "bool":
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected boolean")
		}
		buf.WriteBool(v)
	case "string":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		return buf.WriteString(v)
	case "UUID":
		v, ok := value.([]byte)
		if !ok || len(v) != 16 {
			return fmt.Errorf("invalid UUID")
		}
		buf.WriteBytes(v)
	case "void":
	case "Slot":
		v, ok := value.(ItemStack)
		if !ok {
			return fmt.Errorf("expected item")
		}
		return encodeItemStack(buf, v, itemDepth+1)
	case "anonymousNbt", "anonOptionalNbt":
		if value == nil {
			if s.kind != "anonOptionalNbt" {
				return fmt.Errorf("missing NBT")
			}
			buf.WriteByte(nbt.TagEnd)
			return nil
		}
		v, ok := value.(NetworkTag)
		if !ok {
			return fmt.Errorf("expected NBT")
		}
		e := nbt.NewEncoder(128)
		if err := e.WriteRootTag(v.Type, v.Value); err != nil {
			return err
		}
		if len(e.Bytes()) > maxComponentNBTBytes {
			return fmt.Errorf("NBT exceeds limit")
		}
		buf.WriteBytes(e.Bytes())
	case "option":
		buf.WriteBool(value != nil)
		if value != nil {
			return write(s.element, value)
		}
	case "array":
		v, ok := value.([]any)
		if !ok || len(v) > maxComponentMapEntries {
			return fmt.Errorf("invalid component list")
		}
		buf.WriteVarInt(int32(len(v)))
		for _, entry := range v {
			if err := write(s.element, entry); err != nil {
				return err
			}
		}
	case "mapper":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected enum")
		}
		id, ok := s.reverse[v]
		if !ok {
			return fmt.Errorf("invalid enum %s", v)
		}
		return write(s.element, id)
	case "container":
		v, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("expected component fields")
		}
		for _, field := range s.fields {
			entry := v[field.name]
			if field.anon {
				entry = v
			}
			if err := writeWireValue(buf, field.schema, entry, v, itemDepth, level+1, budget); err != nil {
				return fmt.Errorf("field %s: %w", field.name, err)
			}
		}
	case "switch":
		child := s.alternatives[fmt.Sprint(ctx[s.compare])]
		if child == nil {
			return fmt.Errorf("invalid union tag")
		}
		return write(child, value)
	case "registryEntryHolder":
		v, ok := value.(RegistryHolder)
		if !ok || v.ID < -1 {
			return fmt.Errorf("invalid holder")
		}
		buf.WriteVarInt(v.ID + 1)
		if v.ID == -1 {
			return write(s.element, v.Data)
		}
	case "registryEntryHolderSet":
		v, ok := value.(RegistrySet)
		if !ok || len(v.IDs) > maxComponentMapEntries {
			return fmt.Errorf("invalid set")
		}
		if v.Tag != "" {
			buf.WriteVarInt(0)
			return buf.WriteString(v.Tag)
		}
		buf.WriteVarInt(int32(len(v.IDs) + 1))
		for _, id := range v.IDs {
			if id < 0 {
				return fmt.Errorf("invalid registry id")
			}
			buf.WriteVarInt(id)
		}
	default:
		return fmt.Errorf("unsupported codec %s", s.kind)
	}
	return nil
}

// cloneStructuredValue recursively owns all mutable component data.
func cloneStructuredValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, x := range v {
			m[k] = cloneStructuredValue(x)
		}
		return m
	case []any:
		r := make([]any, len(v))
		for i, x := range v {
			r[i] = cloneStructuredValue(x)
		}
		return r
	case []byte:
		return bytes.Clone(v)
	case ItemStack:
		return v.Clone()
	case RegistryHolder:
		v.Data = cloneStructuredValue(v.Data)
		return v
	case RegistrySet:
		v.IDs = slices.Clone(v.IDs)
		return v
	case NetworkTag:
		c := nbt.NewCompound().PutTag("v", v.Type, v.Value)
		v.Value, _, _ = c.Get("v")
		return v
	default:
		return v
	}
}
