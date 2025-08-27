package chat

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/twitchtv/twirp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ pb.ChatService = (*Server)(nil)

type Assistant interface {
	Title(ctx context.Context, conv *model.Conversation) (string, error)
	Reply(ctx context.Context, conv *model.Conversation) (string, error)
}

type Server struct {
	repo   *model.Repository
	assist Assistant
}

func NewServer(repo *model.Repository, assist Assistant) *Server {
	return &Server{repo: repo, assist: assist}
}

var (
	meter  = otel.GetMeterProvider().Meter("chat-server")
	tracer = otel.Tracer("chat-server")

	requestCounter, _    = meter.Int64Counter("chat_requests_total")
	errorCounter, _      = meter.Int64Counter("chat_errors_total")
	responseHistogram, _ = meter.Float64Histogram("chat_response_duration_ms")
)

func instrument(ctx context.Context, method string, fn func(ctx context.Context) (any, error)) (any, error) {
	ctx, span := tracer.Start(ctx, method)
	defer span.End()
	start := time.Now()
	requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", method)))

	resp, err := fn(ctx)
	duration := float64(time.Since(start).Milliseconds())
	responseHistogram.Record(ctx, duration, metric.WithAttributes(attribute.String("method", method)))

	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", method)))
		span.RecordError(err)
	}
	return resp, err
}

func (s *Server) StartConversation(ctx context.Context, req *pb.StartConversationRequest) (*pb.StartConversationResponse, error) {
	result, err := instrument(ctx, "StartConversation", func(ctx context.Context) (any, error) {
		if strings.TrimSpace(req.GetMessage()) == "" {
			return nil, twirp.RequiredArgumentError("message")
		}

		questionTime := time.Now()

		conversation := &model.Conversation{
			ID:        primitive.NewObjectID(),
			Title:     "Untitled conversation",
			CreatedAt: questionTime,
			UpdatedAt: questionTime,
			Messages: []*model.Message{{
				ID:        primitive.NewObjectID(),
				Role:      model.RoleUser,
				Content:   req.GetMessage(),
				CreatedAt: questionTime,
				UpdatedAt: questionTime,
			}},
		}

		// choose a title
		title, err := s.assist.Title(ctx, conversation)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate conversation title", "error", err)
		} else {
			conversation.Title = title
		}

		// generate a reply
		reply, err := s.assist.Reply(ctx, conversation)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate conversation reply", "error", err)
			return nil, err
		}
		replyTime := time.Now()

		conversation.Messages = append(conversation.Messages, &model.Message{
			ID:        primitive.NewObjectID(),
			Role:      model.RoleAssistant,
			Content:   reply,
			CreatedAt: replyTime,
			UpdatedAt: replyTime,
		})

		if err := s.repo.CreateConversation(ctx, conversation); err != nil {
			slog.ErrorContext(ctx, "Failed to create conversation", "error", err)
			return nil, err
		}

		return &pb.StartConversationResponse{
			ConversationId: conversation.ID.Hex(),
			Title:          conversation.Title,
			Reply:          reply,
		}, nil

	})
	if err != nil {
		return nil, err
	}
	return result.(*pb.StartConversationResponse), nil
}

func (s *Server) ContinueConversation(ctx context.Context, req *pb.ContinueConversationRequest) (*pb.ContinueConversationResponse, error) {

	result, err := instrument(ctx, "ContinueConversation", func(ctx context.Context) (any, error) {

		if req.GetConversationId() == "" {
			return nil, twirp.RequiredArgumentError("conversation_id")
		}

		if strings.TrimSpace(req.GetMessage()) == "" {
			return nil, twirp.RequiredArgumentError("message")
		}

		conversation, err := s.repo.DescribeConversation(ctx, req.GetConversationId())
		if err != nil {
			return nil, err
		}

		conversation.UpdatedAt = time.Now()
		conversation.Messages = append(conversation.Messages, &model.Message{
			ID:        primitive.NewObjectID(),
			Role:      model.RoleUser,
			Content:   req.GetMessage(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})

		reply, err := s.assist.Reply(ctx, conversation)
		if err != nil {
			return nil, twirp.InternalErrorWith(err)
		}

		conversation.Messages = append(conversation.Messages, &model.Message{
			ID:        primitive.NewObjectID(),
			Role:      model.RoleAssistant,
			Content:   reply,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})

		if err := s.repo.UpdateConversation(ctx, conversation); err != nil {
			return nil, twirp.InternalErrorWith(err)
		}

		return &pb.ContinueConversationResponse{Reply: reply}, nil

	})
	if err != nil {
		return nil, err
	}
	return result.(*pb.ContinueConversationResponse), nil
}

func (s *Server) ListConversations(ctx context.Context, req *pb.ListConversationsRequest) (*pb.ListConversationsResponse, error) {

	result, err := instrument(ctx, "ListConversations", func(ctx context.Context) (any, error) {

		conversations, err := s.repo.ListConversations(ctx)
		if err != nil {
			return nil, twirp.InternalErrorWith(err)
		}

		resp := &pb.ListConversationsResponse{}
		for _, conv := range conversations {
			conv.Messages = nil // Clear messages to avoid sending large data
			resp.Conversations = append(resp.Conversations, conv.Proto())
		}

		return resp, nil

	})
	if err != nil {
		return nil, err
	}
	return result.(*pb.ListConversationsResponse), nil
}

func (s *Server) DescribeConversation(ctx context.Context, req *pb.DescribeConversationRequest) (*pb.DescribeConversationResponse, error) {
	result, err := instrument(ctx, "DescribeConversation", func(ctx context.Context) (any, error) {

		if req.GetConversationId() == "" {
			return nil, twirp.RequiredArgumentError("conversation_id")
		}

		conversation, err := s.repo.DescribeConversation(ctx, req.GetConversationId())
		if err != nil {
			return nil, err
		}

		if conversation == nil {
			return nil, twirp.NotFoundError("conversation not found")
		}

		return &pb.DescribeConversationResponse{Conversation: conversation.Proto()}, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*pb.DescribeConversationResponse), nil
}
