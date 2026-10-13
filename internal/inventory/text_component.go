package inventory

import (
	"fmt"

	"github.com/vitismc/vitis/internal/nbt"
)

// flatTextFromNBT translates persistent JSON text into network NBT text.
func flatTextFromNBT(value any) (RawNBTComponent, error) {
	raw, ok := value.(string)
	if !ok || len(raw) > maxComponentNBTBytes {
		return RawNBTComponent{}, fmt.Errorf("invalid persistent text")
	}
	tag, text, err := nbt.FromJSON([]byte(raw))
	if err != nil {
		return RawNBTComponent{}, err
	}
	encoder := nbt.NewEncoder(128)
	if err := encoder.WriteRootTag(tag, text); err != nil {
		return RawNBTComponent{}, err
	}
	if len(encoder.Bytes()) > maxComponentNBTBytes {
		return RawNBTComponent{}, fmt.Errorf("text exceeds limit")
	}
	return RawNBTComponent{Data: encoder.Bytes()}, nil
}

// flatTextToNBT translates network NBT text into the persistent JSON text codec.
func flatTextToNBT(value RawNBTComponent) (byte, any, error) {
	_, text, err := nbt.NewDecoder(value.Data).ReadRootTag()
	if err != nil {
		return 0, nil, err
	}
	json, err := nbt.ToJSON(text)
	if err != nil {
		return 0, nil, err
	}
	return nbt.TagString, string(json), nil
}
