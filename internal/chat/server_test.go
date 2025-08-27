package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	. "github.com/acai-travel/tech-challenge/internal/chat/testing"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestServer_StartConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), assistant.New())

	t.Run("start conversation creates new conversation and populates title/response", WithFixture(func(t *testing.T, f *Fixture) {
		req := pb.StartConversationRequest{
			Message: "What is the weather in Paris?",
		}
		out, err := srv.StartConversation(ctx, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		conv, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: out.GetConversationId()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conv == nil {
			t.Fatal("expected conversation, got nil")
		}
		if conv.Conversation.Title == "" {
			t.Error("expected conversation title to be populated")
		}
		if len(conv.Conversation.Messages) == 0 || conv.Conversation.Messages[len(conv.Conversation.Messages)-1].Role != 2 {
			t.Error("expected assistant response message")
		}
		if !strings.Contains(conv.Conversation.Title, "weather in Paris") {
			t.Error("expected conversation title to be summarized")
		}
	}))
}
func TestServer_DescribeConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), nil)

	t.Run("describe existing conversation", WithFixture(func(t *testing.T, f *Fixture) {
		c := f.CreateConversation()

		out, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: c.ID.Hex()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, want := out.GetConversation(), c.Proto()
		if !cmp.Equal(got, want, protocmp.Transform()) {
			t.Errorf("DescribeConversation() mismatch (-got +want):\n%s", cmp.Diff(got, want, protocmp.Transform()))
		}
	}))

	t.Run("describe non existing conversation should return 404", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: "08a59244257c872c5943e2a2"})
		if err == nil {
			t.Fatal("expected error for non-existing conversation, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.NotFound {
			t.Fatalf("expected twirp.NotFound error, got %v", err)
		}
	}))
}
