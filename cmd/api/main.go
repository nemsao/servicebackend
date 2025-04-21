package main

import (
    "context"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
    "services_app/internal/config"
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
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create gRPC server
	server := grpc.NewServer(
		grpc.UnaryInterceptor(services.UnaryServerInterceptor),
		grpc.StreamInterceptor(services.StreamServerInterceptor),
	)

	// Register services
	services.RegisterServices(server, db, cfg)

	// Enable reflection for testing
	reflection.Register(server)

	// Start server
	lis, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Graceful shutdown
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Println("Shutting down server...")
	server.GracefulStop()
	log.Println("Server stopped")
} 