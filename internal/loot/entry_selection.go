package loot

import "math"

// weightedEntry is one eligible singleton, including an expanded item-tag member.
type weightedEntry struct {
	entry    *Entry
	itemName string
	weight   int64
}

// entryWeight applies luck before rounding and keeps explicit zero weights disabled.
func entryWeight(entry *Entry, luck float64) int64 {
	weight := entry.Weight
	if weight == 0 && !entry.weightSpecified {
		weight = 1
	}
	value := math.Floor(float64(weight) + float64(entry.Quality)*luck)
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	return int64(min(value, float64(math.MaxInt32)))
}

// expandEntry collects eligible leaves without evaluating their conditions twice.
func expandEntry(entry *Entry, ctx *Context, choices *[]weightedEntry, depth int) bool {
	if depth >= 64 || !evaluateConditions(entry.Conditions, ctx) {
		return false
	}
	switch EntryType(entry.Type) {
	case EntryTypeAlternatives:
		for i := range entry.Children {
			if expandEntry(&entry.Children[i], ctx, choices, depth+1) {
				return true
			}
		}
		return false
	case EntryTypeSequence:
		for i := range entry.Children {
			if !expandEntry(&entry.Children[i], ctx, choices, depth+1) {
				return false
			}
		}
		return true
	case EntryTypeGroup:
		if len(entry.Children) == 1 {
			return expandEntry(&entry.Children[0], ctx, choices, depth+1)
		}
		for i := range entry.Children {
			expandEntry(&entry.Children[i], ctx, choices, depth+1)
		}
		return true
	}
	weight := entryWeight(entry, ctx.Luck)
	if EntryType(entry.Type) == EntryTypeTag && entry.Expand {
		if ctx.tagResolver != nil {
			for _, name := range ctx.tagResolver(entry.Name) {
				if weight > 0 {
					*choices = append(*choices, weightedEntry{entry: entry, itemName: name, weight: weight})
				}
			}
		}
		return true
	}
	if weight > 0 {
		*choices = append(*choices, weightedEntry{entry: entry, weight: weight})
	}
	return true
}

// evaluateEntries selects one weighted leaf from the expanded entry trees.
func evaluateEntries(entries []Entry, ctx *Context) []Drop {
	if ctx == nil || ctx.Rng == nil {
		return nil
	}
	var choices []weightedEntry
	for i := range entries {
		expandEntry(&entries[i], ctx, &choices, 0)
	}
	if len(choices) == 0 {
		return nil
	}
	selected := choices[0]
	if len(choices) > 1 {
		total := int64(0)
		for _, choice := range choices {
			total += choice.weight
		}
		index := ctx.Rng.Int63n(total)
		for _, choice := range choices {
			index -= choice.weight
			if index < 0 {
				selected = choice
				break
			}
		}
	}
	if selected.itemName != "" {
		entry := *selected.entry
		entry.Type, entry.Name = string(EntryTypeItem), selected.itemName
		return evaluateEntryContent(&entry, ctx)
	}
	return evaluateEntryContent(selected.entry, ctx)
}
