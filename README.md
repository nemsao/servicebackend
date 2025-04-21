# Ticket Selling API

A high-volume ticket selling API built with Go, gRPC, PostgreSQL, and Kubernetes.

## Architecture

The application is built using a microservices architecture with the following services:

- User Service: Manages user accounts and profiles
- Event Service: Handles event creation and management
- Ticket Service: Manages ticket inventory and pricing
- Order Service: Processes ticket orders
- Payment Service: Handles payment processing
- Notification Service: Sends notifications to users
- Analytics Service: Provides analytics and reporting
- Auth Service: Handles authentication and authorization

## Technology Stack

- Go 1.21
- gRPC
- PostgreSQL
- Kubernetes
- Docker
- JWT for authentication

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker
- Kubernetes cluster
- PostgreSQL database

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/ticketsellingapp.git
   cd ticketsellingapp
   ```

2. Build the Docker image:
   ```bash
   docker build -t ticketsellingapp .
   ```

3. Deploy to Kubernetes:
   ```bash
   kubectl apply -f k8s/deployment.yaml
   ```

### Configuration

The application uses a configuration file located at `configs/config.yaml`. You can customize the following settings:

- Database connection details
- JWT secret key
- Service ports
- Logging configuration

## API Documentation

The API is documented using Protocol Buffers. You can find the service definitions in the `proto` directory:

- `user_service.proto`: User management
- `event_service.proto`: Event management
- `ticket_service.proto`: Ticket management
- `order_service.proto`: Order processing
- `payment_service.proto`: Payment processing
- `notification_service.proto`: Notifications
- `analytics_service.proto`: Analytics
- `auth_service.proto`: Authentication

## Development

### Running Tests

```bash
go test ./...
```

### Generating Protocol Buffers

```bash
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/*.proto
```

## License

This project is licensed under the MIT License - see the LICENSE file for details. 