// Package disgo provides Discord API adapters using package github.com/disgoorg/disgo
package disgo

import (
	"fmt"

	"github.com/disgoorg/snowflake/v2"
)

// ParseID parses a domain ID (e.g. pomomo.VoiceChannelID, guild/user/message IDs) into a snowflake.
func ParseID[T ~string](id T) (snowflake.ID, error) {
	if id == "" {
		return 0, fmt.Errorf("empty snowflake id")
	}
	sf, err := snowflake.Parse(string(id))
	if err != nil {
		return 0, fmt.Errorf("invalid snowflake id %q: %w", id, err)
	}
	return sf, nil
}

// ParseIDs parses multiple domain IDs, failing on the first invalid one.
func ParseIDs[T ~string](ids ...T) ([]snowflake.ID, error) {
	res := make([]snowflake.ID, 0, len(ids))
	for _, id := range ids {
		sf, err := ParseID(id)
		if err != nil {
			return nil, err
		}
		res = append(res, sf)
	}
	return res, nil
}

// FormatID converts a snowflake into a domain ID, e.g. FormatID[pomomo.TextChannelID](id).
func FormatID[T ~string](id snowflake.ID) T {
	return T(id.String())
}
