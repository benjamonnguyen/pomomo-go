package disgo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
)

type recordedRequest struct {
	Method, Path string
	Body         map[string]any
}

func newTestMessenger(t *testing.T) (*Messenger, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var reqs []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path}
		if len(b) > 0 {
			_ = json.Unmarshal(b, &rec.Body)
		}
		mu.Lock()
		reqs = append(reqs, rec)
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/callback") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"222","channel_id":"111","type":0,"content":""}`))
	}))
	t.Cleanup(srv.Close)
	r := rest.New(rest.NewClient("token", rest.WithURL(srv.URL)))
	return NewMessenger(r), func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

func testInteraction(t *testing.T) discord.Interaction {
	t.Helper()
	it, err := discord.UnmarshalInteraction([]byte(`{
		"id":"333","application_id":"444","type":3,"token":"tok","version":1,
		"guild_id":"555","channel":{"id":"111","type":0},
		"data":{"component_type":2,"custom_id":"skip:111"},
		"message":{"id":"222","channel_id":"111","type":0,"content":""}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func flagsOf(t *testing.T, body map[string]any) discord.MessageFlags {
	t.Helper()
	f, _ := body["flags"].(float64)
	return discord.MessageFlags(f)
}

func TestMessengerEditChannelMessage(t *testing.T) {
	m, reqs := newTestMessenger(t)
	if _, err := m.EditChannelMessage("111", "222", discord.TextDisplayComponent{Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	got := reqs()
	if len(got) != 1 || got[0].Method != http.MethodPatch || !strings.HasSuffix(got[0].Path, "/channels/111/messages/222") {
		t.Fatalf("unexpected requests: %+v", got)
	}
	if !flagsOf(t, got[0].Body).Has(discord.MessageFlagIsComponentsV2) {
		t.Errorf("missing IsComponentsV2 flag: %v", got[0].Body)
	}
	if _, err := m.EditChannelMessage("111", "not-an-id"); err == nil {
		t.Error("expected error for invalid message id")
	}
}

func TestMessengerRespond(t *testing.T) {
	m, reqs := newTestMessenger(t)
	msg, err := m.Respond(testInteraction(t), true, discord.TextDisplayComponent{Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.ID.String() != "222" {
		t.Fatalf("expected original message, got %+v", msg)
	}
	got := reqs()
	if len(got) != 2 {
		t.Fatalf("expected callback + get original, got %+v", got)
	}
	if !strings.HasSuffix(got[0].Path, "/interactions/333/tok/callback") || got[0].Body["type"] != float64(discord.InteractionResponseTypeCreateMessage) {
		t.Errorf("unexpected callback: %+v", got[0])
	}
	data, _ := got[0].Body["data"].(map[string]any)
	if !flagsOf(t, data).Has(discord.MessageFlagIsComponentsV2) {
		t.Errorf("missing IsComponentsV2 flag: %v", data)
	}
	if got[1].Method != http.MethodGet || !strings.HasSuffix(got[1].Path, "/webhooks/444/tok/messages/@original") {
		t.Errorf("unexpected original fetch: %+v", got[1])
	}
}

func TestMessengerDeferMessageCreate(t *testing.T) {
	m, reqs := newTestMessenger(t)
	followup, err := m.DeferMessageCreate(testInteraction(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := followup(discord.TextDisplayComponent{Content: "Joined session!"}); err != nil {
		t.Fatal(err)
	}
	got := reqs()
	if len(got) != 2 {
		t.Fatalf("unexpected requests: %+v", got)
	}
	data, _ := got[0].Body["data"].(map[string]any)
	if got[0].Body["type"] != float64(discord.InteractionResponseTypeDeferredCreateMessage) || !flagsOf(t, data).Has(discord.MessageFlagEphemeral) {
		t.Errorf("unexpected defer: %+v", got[0])
	}
	if got[1].Method != http.MethodPost || !strings.HasSuffix(got[1].Path, "/webhooks/444/tok") || !flagsOf(t, got[1].Body).Has(discord.MessageFlagIsComponentsV2) {
		t.Errorf("unexpected followup: %+v", got[1])
	}
}

func TestMessengerDeferMessageUpdate(t *testing.T) {
	m, reqs := newTestMessenger(t)
	followup, err := m.DeferMessageUpdate(testInteraction(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := followup(discord.TextDisplayComponent{Content: "x"}); err != nil {
		t.Fatal(err)
	}
	got := reqs()
	if len(got) != 2 {
		t.Fatalf("unexpected requests: %+v", got)
	}
	if got[0].Body["type"] != float64(discord.InteractionResponseTypeDeferredUpdateMessage) {
		t.Errorf("unexpected defer: %+v", got[0])
	}
	if got[1].Method != http.MethodPatch || !strings.HasSuffix(got[1].Path, "/webhooks/444/tok/messages/@original") {
		t.Errorf("unexpected followup: %+v", got[1])
	}
}
