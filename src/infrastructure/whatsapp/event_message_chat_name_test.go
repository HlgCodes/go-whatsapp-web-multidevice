package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// TestBuildFromFieldsIncludesGroupChatName pins the fork's own contribution: the generic
// webhook payload must expose a group's display name under "chat_name" so external
// consumers can label a group conversation without calling WhatsApp again.
//
// Upstream resolves the group name on the Chatwoot path (see chatwoot_content_test.go)
// but never puts it in the payload, so this assertion guards a real gap.
func TestBuildFromFieldsIncludesGroupChatName(t *testing.T) {
	const groupJID = "120363999000222@g.us"

	// Seed the TTL cache so getGroupName resolves without a live WhatsApp client.
	// A cached value is returned before the client lookup, which keeps the test
	// hermetic and deterministic.
	setCachedGroupName(groupJID, "Familia Guerrero")

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		t.Fatalf("ParseJID: %v", err)
	}

	payload := map[string]any{}
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     jid,
				Sender:   jid,
				IsFromMe: false,
			},
			ID:        "TESTGROUP1",
			Timestamp: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{Conversation: protoString("hola")},
	}

	buildFromFields(context.Background(), nil, evt, payload)

	got, ok := payload["chat_name"]
	if !ok {
		t.Fatal("expected payload to contain chat_name for a group chat")
	}
	if got != "Familia Guerrero" {
		t.Fatalf("chat_name = %v, want %q", got, "Familia Guerrero")
	}
	if payload["chat_id"] != groupJID {
		t.Fatalf("chat_id = %v, want %q", payload["chat_id"], groupJID)
	}
}

// TestBuildFromFieldsOmitsChatNameForDirectChats is the negative half: a 1:1 chat has
// no group name, so the key must be absent rather than present-and-empty. Downstream
// consumers use the key's presence to decide whether they are looking at a group.
func TestBuildFromFieldsOmitsChatNameForDirectChats(t *testing.T) {
	jid := types.NewJID("628111", types.DefaultUserServer)

	payload := map[string]any{}
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     jid,
				Sender:   jid,
				IsFromMe: false,
			},
			ID:        "TESTDIRECT1",
			Timestamp: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{Conversation: protoString("hola")},
	}

	buildFromFields(context.Background(), nil, evt, payload)

	if v, present := payload["chat_name"]; present {
		t.Fatalf("chat_name must be absent for a direct chat, got %v", v)
	}
}
