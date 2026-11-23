package disgo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/rest"
)

func TestAdapterUsername(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/members/1"):
			_, _ = w.Write([]byte(`{"user":{"id":"1","username":"tomato_fan"}}`))
		case strings.HasSuffix(r.URL.Path, "/members/2"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":10007,"message":"Unknown Member"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":0,"message":"oops"}`))
		}
	}))
	t.Cleanup(srv.Close)
	a := NewAdapter(rest.New(rest.NewClient("token", rest.WithURL(srv.URL))), nil, nil)

	if name, inGuild, err := a.Username("9", "1"); err != nil || !inGuild || name != "tomato_fan" {
		t.Errorf("member: got (%q, %v, %v)", name, inGuild, err)
	}
	if _, inGuild, err := a.Username("9", "2"); err != nil || inGuild {
		t.Errorf("departed member should be (not in guild, nil err), got (%v, %v)", inGuild, err)
	}
	if _, _, err := a.Username("9", "3"); err == nil {
		t.Error("expected error for server failure")
	}
}
