package disgo

import (
	"errors"

	"github.com/disgoorg/disgo/rest"
)

// DisplayName returns the member's server display name (nickname, global name, then username);
// inGuild is false if they're no longer a member.
func (a *Adapter) DisplayName(gid, uid string) (string, bool, error) {
	ids, err := ParseIDs(gid, uid)
	if err != nil {
		return "", false, err
	}
	m, err := a.rest.GetMember(ids[0], ids[1])
	if err != nil {
		var rerr *rest.Error
		if errors.As(err, &rerr) && rerr.Code == rest.JSONErrorCodeUnknownMember {
			return "", false, nil
		}
		return "", false, err
	}
	return m.EffectiveName(), true, nil
}
