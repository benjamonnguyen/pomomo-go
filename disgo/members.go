package disgo

import (
	"errors"

	"github.com/disgoorg/disgo/rest"
)

// Username returns the member's username; inGuild is false if they're no longer a member.
func (a *Adapter) Username(gid, uid string) (string, bool, error) {
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
	return m.User.Username, true, nil
}
