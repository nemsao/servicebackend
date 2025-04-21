package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nemsao/servicebackend/internal/config"   // Assuming your config package path
	"github.com/nemsao/servicebackend/internal/database" // Assuming your database package path
	pb "github.com/nemsao/servicebackend/proto/order_service"    // Assuming your proto package path
)
type Service struct {
	db  *database.PostgresDB
	cfg *config.Config
	pb.UnimplementedOrderServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error) {
	// Validate request
	if req.CustomerId == "" || len(req.OrderItems) == 0 {
		return nil, status.Error(codes.InvalidArgument, "customer_id and at least one order item are required")
	}

	orderID := uuid.New().String()
	orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano()) // Simple order number generation
	orderDate := time.Now().UTC()
	var subtotal float64
	var totalAmount float64

	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Calculate subtotal and total amount for order items
		for _, item := range req.OrderItems {
			subtotal += item.UnitPrice * float64(item.Quantity)
			totalAmount += item.UnitPrice * float64(item.Quantity) // Initially, assume no discounts, tax, or fees
		}

		// Insert order
		_, err := tx.Exec(ctx, `
			INSERT INTO public.orders (
				id, customer_id, order_number, order_date, subtotal, total_amount, currency, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, orderID, req.CustomerId, orderNumber, orderDate, subtotal, totalAmount, req.Currency, "pending", orderDate, orderDate)
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}

		// Insert order items
		for _, item := range req.OrderItems {
			orderItemID := nextBigInt() // Assuming you have a function to generate unique big integers
			itemSubtotal := item.UnitPrice * float64(item.Quantity)
			itemTotal := itemSubtotal // Initially, assume no discounts, tax, or fees for item

			_, err := tx.Exec(ctx, `
				INSERT INTO public.order_items (
					id, order_id, ticket_id, quantity, unit_price, subtotal, total_amount, status, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, orderItemID, orderID, item.TicketId, item.Quantity, item.UnitPrice, itemSubtotal, itemTotal, "active", orderDate, orderDate)
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	// Fetch the created order for the response
	order, err := s.getOrderInternal(ctx, orderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve created order: %v", err)
	}

	return &pb.OrderResponse{
		Order: order,
	}, nil
}

func (s *Service) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.OrderResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID is required")
	}

	order, err := s.getOrderInternal(ctx, req.Id)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, err
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	return &pb.OrderResponse{
		Order: order,
	}, nil
}

func (s *Service) getOrderInternal(ctx context.Context, orderID string) (*pb.Order, error) {
	var o pb.Order
	var orderDate, createdAt, updatedAt time.Time

	err := s.db.Get(ctx, &o.Id, &o.CustomerId, &o.OrderNumber, &orderDate, &o.Subtotal, &o.DiscountAmount, &o.TaxAmount, &o.FeeAmount, &o.TotalAmount, &o.Currency, &o.DiscountId, &o.Status, &o.BillingName, &o.BillingEmail, &o.BillingAddress, &o.BillingCity, &o.BillingState, &o.BillingCountry, &o.BillingPostalCode, &o.BillingPhone, &o.Notes, &o.IpAddress, &o.UserAgent, &o.ExpiryDate, &createdAt, &updatedAt,
		`
			SELECT 
				id, customer_id, order_number, order_date, subtotal, discount_amount, tax_amount, fee_amount, total_amount, currency, discount_id, status, billing_name, billing_email, billing_address, billing_city, billing_state, billing_country, billing_postal_code, billing_phone, notes, ip_address, user_agent, expiry_date, created_at, updated_at
			FROM public.orders
			WHERE id = $1
		`, orderID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "order with ID %s not found", orderID)
		}
		return nil, fmt.Errorf("failed to get order header: %w", err)
	}

	o.OrderDate = orderDate.Format(time.RFC3339)
	if !o.ExpiryDate.IsZero() {
		o.ExpiryDate = o.ExpiryDate.Format(time.RFC3339)
	}
	o.CreatedAt = createdAt.Format(time.RFC3339)
	o.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Fetch order items
	itemRows, err := s.db.Query(ctx, `
		SELECT 
			id, ticket_id, quantity, unit_price, subtotal, discount_amount, tax_amount, fee_amount, total_amount, status, created_at, updated_at
		FROM public.order_items
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer itemRows.Close()

	var orderItems []*pb.OrderItem
	for itemRows.Next() {
		var oi pb.OrderItem
		var createdAt, updatedAt time.Time
		err := itemRows.Scan(&oi.Id, &oi.TicketId, &oi.Quantity, &oi.UnitPrice, &oi.Subtotal, &oi.DiscountAmount, &oi.TaxAmount, &oi.FeeAmount, &oi.TotalAmount, &oi.Status, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		oi.CreatedAt = createdAt.Format(time.RFC3339)
		oi.UpdatedAt = updatedAt.Format(time.RFC3339)
		orderItems = append(orderItems, &oi)
	}

	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order item rows: %w", err)
	}

	o.OrderItems = orderItems
	return &o, nil
}

func (s *Service) UpdateOrder(ctx context.Context, req *pb.UpdateOrderRequest) (*pb.OrderResponse, error) {
	if req.Order == nil || req.Order.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "order and order ID are required")
	}

	o := req.Order
	now := time.Now().UTC()

	_, err := s.db.Exec(ctx, `
		UPDATE public.orders 
		SET 
			customer_id = $2, 
			discount_id = $3, 
			status = $4, 
			billing_name = $5, 
			billing_email = $6, 
			billing_address = $7, 
			billing_city = $8, 
			billing_state = $9, 
			billing_country = $10, 
			billing_postal_code = $11, 
			billing_phone = $12, 
			notes = $13, 
			ip_address = $14, 
			user_agent = $15, 
			expiry_date = CASE WHEN $16::TIMESTAMP IS NULL THEN expiry_date ELSE $16 END,
			updated_at = $17
		WHERE id = $1
	`, o.Id, o.CustomerId, o.DiscountId, o.Status, o.BillingName, o.BillingEmail, o.BillingAddress, o.BillingCity, o.BillingState, o.BillingCountry, o.BillingPostalCode, o.BillingPhone, o.Notes, o.IpAddress, o.UserAgent, nullIfZero(parseTime(o.ExpiryDate)), now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order: %v", err)
	}

	// Fetch the updated order for the response
	updatedOrder, err := s.getOrderInternal(ctx, o.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve updated order: %v", err)
	}

	return &pb.OrderResponse{
		Order: updatedOrder,
	}, nil
}

func (s *Service) DeleteOrder(ctx context.Context, req *pb.DeleteOrderRequest) (*pb.Empty, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID is required")
	}

	_, err := s.db.Exec(ctx, `
		DELETE FROM public.orders 
		WHERE id = $1
	`, req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete order: %v", err)
	}

	return &pb.Empty{}, nil
}

func (s *Service) ListOrdersByUser(ctx context.Context, req *pb.ListOrdersByUserRequest) (*pb.ListOrdersResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer ID is required")
	}

	rows, err := s.db.Query(ctx, `
		SELECT id 
		FROM public.orders
		WHERE customer_id = $1
	`, req.CustomerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list order IDs by user: %v", err)
	}
	defer rows.Close()

	var orderIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan order ID: %w", err)
		}
		orderIDs = append(orderIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order ID rows: %w", err)
	}

	var orders []*pb.Order
	for _, orderID := range orderIDs {
		order, err := s.getOrderInternal(ctx, orderID)
		if err != nil {
			// Log the error but continue fetching other orders
			fmt.Printf("Error fetching order %s: %v\n", orderID, err)
			continue
		}
		orders = append(orders, order)
	}

	return &pb.ListOrdersResponse{
		Orders: orders,
	}, nil
}

// Helper function (replace with your actual unique ID generation)
func nextBigInt() int64 {
	return time.Now().UnixNano()
}

// Helper function to parse time string
func parseTime(timeStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, timeStr)
	return t
}

// Helper function to handle zero time values in updates
func nullIfZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}