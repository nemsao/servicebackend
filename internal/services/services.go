package services

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"services_app/internal/database"
	"services_app/internal/services/auth"
	"services_app/internal/services/event"
	"services_app/internal/services/order"
	"services_app/internal/services/ticket"
	"services_app/internal/services/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterServices registers all gRPC services with the server
func RegisterServices(server *grpc.Server, db *database.PostgresDB, cfg *config.Config) {
	// Initialize services
	userService := user.NewService(db, cfg)
	eventService := event.NewService(db, cfg)
	ticketService := ticket.NewService(db, cfg)
	orderService := order.NewService(db, cfg)
	authService := auth.NewService(db, cfg)

	// Register services
	user.RegisterUserServiceServer(server, userService)
	event.RegisterEventServiceServer(server, eventService)
	ticket.RegisterTicketServiceServer(server, ticketService)
	order.RegisterOrderServiceServer(server, orderService)
	auth.RegisterAuthServiceServer(server, authService)
}

// UnaryServerInterceptor is a gRPC interceptor for unary RPCs
func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Add request ID to context
	ctx = context.WithValue(ctx, "request_id", time.Now().UnixNano())

	// Log request
	log.Printf("Received request: %v", info.FullMethod)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Call handler
	resp, err := handler(ctx, req)
	if err != nil {
		// Log error
		log.Printf("Error handling request: %v", err)

		// Convert error to gRPC status
		if st, ok := status.FromError(err); ok {
			return nil, st.Err()
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

// StreamServerInterceptor is a gRPC interceptor for streaming RPCs
func StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// Add request ID to context
	ctx := context.WithValue(ss.Context(), "request_id", time.Now().UnixNano())

	// Create wrapped stream
	wrapped := &wrappedStream{
		ServerStream: ss,
		ctx:          ctx,
	}

	// Log request
	log.Printf("Received stream request: %v", info.FullMethod)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Call handler
	err := handler(srv, wrapped)
	if err != nil {
		// Log error
		log.Printf("Error handling stream request: %v", err)

		// Convert error to gRPC status
		if st, ok := status.FromError(err); ok {
			return st.Err()
		}
		return status.Error(codes.Internal, err.Error())
	}

	return nil
}

// wrappedStream wraps the original grpc.ServerStream
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
} 
