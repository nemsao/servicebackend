package services

import (
	"context"
	"time"
"log"
	"google.golang.org/grpc"

	"services_app/internal/database"
pb_auth	"services_app/proto/auth"
pb_event	"services_app/proto/event"
pb_order	"services_app/proto/order"
pb_ticket	"services_app/proto/ticket"
pb_user	"services_app/proto/user"
pb_seat	"services_app/proto/seat"

   "services_app/internal/services/auth"
	"services_app/internal/services/event"
	"services_app/internal/services/order"
	"services_app/internal/services/ticket"
	"services_app/internal/services/user"
	"services_app/internal/services/seat"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"services_app/internal/config"
)

// RegisterServices registers all gRPC services with the server
func RegisterServices(server *grpc.Server, db *database.PostgresDB,redis *database.RedisClient , cfg *config.Config) {
   // Initialize services
   userService := user.NewService(db, cfg)
   eventService := event.NewService(db, cfg)
   seatService := seat.NewService(db,redis , cfg)
   // Khởi tạo TicketService server implementation
   // Biến ticketService có kiểu là *"services_app/internal/services/ticket".Service
   ticketService := ticket.NewService(db, cfg)

   // Khởi tạo OrderService, truyền implement server của TicketService vào đây.
   // Điều này hợp lệ vì *"services_app/internal/services/ticket".Service implements pb_ticket.TicketServiceServer
   orderService := order.NewService(db, cfg, ticketService) // <== Pass implement server

   authService := auth.NewService(db, cfg)


   // Register services
   pb_user.RegisterUserServiceServer(server, userService)
   pb_event.RegisterEventServiceServer(server, eventService)
   pb_ticket.RegisterTicketServiceServer(server, ticketService) // <== Đăng ký implement server TicketService
   pb_order.RegisterOrderServiceServer(server, orderService)      // <== Đăng ký implement server OrderService
   pb_seat.RegisterSeatServiceServer(server, seatService)

   pb_auth.RegisterAuthServiceServer(server, authService)
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
