package main // Đảm bảo file này là package main

import (
	"context" // Lưu ý: context hiện chưa được sử dụng trực tiếp trong main, có thể gây lỗi "imported and not used" nếu không dùng ở đâu khác.
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Chỉ import một lần
	"services_app/internal/config"
	"services_app/internal/database"
	"services_app/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.NewPostgresDB(cfg.Database) // Giả sử hàm này tồn tại và nhận cfg.Database
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close() // Giả sử db có phương thức Close()

	// Create gRPC server
	server := grpc.NewServer(
	// Giả sử các interceptor này tồn tại trong package services
	// grpc.UnaryInterceptor(services.UnaryServerInterceptor),
	// grpc.StreamInterceptor(services.StreamServerInterceptor),
	)

	// Register services
	// Giả sử hàm này tồn tại và nhận các tham số này
	services.RegisterServices(server, db, cfg)

	// Enable reflection for testing (gRPC server reflection)
	reflection.Register(server)

	// Start server
	lis, err := net.Listen("tcp", cfg.Server.Address) // Giả sử cfg.Server.Address tồn tại
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("gRPC server listening on %s", cfg.Server.Address) // Thêm log cho rõ ràng

	// Chạy server trong một goroutine riêng để không block
	go func() {
		if err := server.Serve(lis); err != nil {
			// Không dùng Fatalf ở đây vì server có thể dừng bình thường
			log.Printf("gRPC server stopped serving: %v", err)
		}
	}()

	// Wait for interrupt signal (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Block cho đến khi nhận được tín hiệu

	// Graceful shutdown
	log.Println("Shutting down gRPC server...")
	server.GracefulStop()
	log.Println("Server stopped gracefully")
}

// Xóa dấu } thừa ở đây