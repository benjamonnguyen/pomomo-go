package disgo

import (
	"testing"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/disgoorg/snowflake/v2"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		in      pomomo.VoiceChannelID
		want    snowflake.ID
		wantErr bool
	}{
		{in: "823497369776947260", want: 823497369776947260},
		{in: "", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "-1", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseID(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatalf("ParseID(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Fatalf("ParseID(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseIDs(t *testing.T) {
	got, err := ParseIDs("1", "2")
	if err != nil || len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("ParseIDs = %v, %v", got, err)
	}
	if _, err := ParseIDs("1", ""); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestFormatIDRoundTrip(t *testing.T) {
	const id pomomo.TextChannelID = "1573045435847938058"
	sf, err := ParseID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatID[pomomo.TextChannelID](sf); got != id {
		t.Fatalf("FormatID = %q, want %q", got, id)
	}
}
