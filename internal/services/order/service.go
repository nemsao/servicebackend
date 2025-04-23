package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	//"github.com/jackc/pgx/v5/pgxpool" // Import pgxpool
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"services_app/internal/config"
	"services_app/internal/database"
	pb "services_app/proto/order"
	ticket_pb "services_app/proto/ticket" // Import the ticket proto
	//ticket_sv "services_app/internal/services/ticket"
)

type Service struct {
	db           *database.PostgresDB
	cfg          *config.Config
	ticketClient ticket_pb.TicketServiceServer // Add ticket service client
	pb.UnimplementedOrderServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config, ticketClient ticket_pb.TicketServiceServer) *Service {
	return &Service{
		db:           db,
		cfg:          cfg,
		ticketClient: ticketClient,
	}
}

func (s *Service) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error) {
	// Validate request
	if req.CustomerId == "" || len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "customer_id and at least one order item are required")
	}

	orderID := uuid.New().String()
	orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano()) // Simple order number generation
	orderDate := time.Now().UTC()
	var subtotal float64
	var totalAmount float64

	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Calculate subtotal and total amount for order items
		for _, item := range req.Items {
			// Call ticket service to get ticket details (including price and currency)
			ticketResp, err := s.ticketClient.GetTicket(ctx, &ticket_pb.GetTicketRequest{TicketId: item.TicketId})
			if err != nil {
				return fmt.Errorf("failed to get ticket %s: %w", item.TicketId, err)
			}
			ticket := ticketResp.GetTicket()
			if ticket == nil {
				return fmt.Errorf("ticket %s not found", item.TicketId)
			}

			// Assign unit price from ticket service.  Use ticket.Price
			unitPrice := ticket.Price
			itemSubtotal := float64(item.Quantity) * unitPrice
			itemTotal := itemSubtotal

			subtotal += itemSubtotal
			totalAmount += itemTotal
		}

		billingInfo := req.BillingInfo
		var billingName, billingEmail, billingAddress, billingCity, billingState, billingCountry, billingPostalCode, billingPhone *string
		if billingInfo != nil {
			billingNamePtr := &billingInfo.Name
			if billingInfo.Name == "" {
				billingNamePtr = nil
			}
			billingEmailPtr := &billingInfo.Email
			if billingInfo.Email == "" {
				billingEmailPtr = nil
			}
			billingAddressPtr := &billingInfo.Address
			if billingInfo.Address == "" {
				billingAddressPtr = nil
			}
			billingCityPtr := &billingInfo.City
			if billingInfo.City == "" {
				billingCityPtr = nil
			}
			billingStatePtr := &billingInfo.State
			if billingInfo.State == "" {
				billingStatePtr = nil
			}
			billingCountryPtr := &billingInfo.Country
			if billingInfo.Country == "" {
				billingCountryPtr = nil
			}
			billingPostalCodePtr := &billingInfo.PostalCode
			if billingInfo.PostalCode == "" {
				billingPostalCodePtr = nil
			}
			billingPhonePtr := &billingInfo.Phone
			if billingInfo.Phone == "" {
				billingPhonePtr = nil
			}

			billingName = billingNamePtr
			billingEmail = billingEmailPtr
			billingAddress = billingAddressPtr
			billingCity = billingCityPtr
			billingState = billingStatePtr
			billingCountry = billingCountryPtr
			billingPostalCode = billingPostalCodePtr
			billingPhone = billingPhonePtr
		}

		// Insert order
		_, err := tx.Exec(ctx, `
			INSERT INTO public.orders (
				id, customer_id, order_number, order_date, subtotal, total_amount, currency, status, billing_name, billing_email, billing_address, billing_city, billing_state, billing_country, billing_postal_code, billing_phone, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		`, orderID, req.CustomerId, orderNumber, orderDate, subtotal, totalAmount, "USD", "pending", billingName, billingEmail, billingAddress, billingCity, billingState, billingCountry, billingPostalCode, billingPhone, orderDate, orderDate)
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}

		// Insert order items
		for _, item := range req.Items {
			orderItemID := uuid.New().String() //  use uuid.New().String()
			//itemSubtotal := float64(item.Quantity) * unitPrice   // use the unitPrice from Ticket service.  PROBLEM LINE
			itemSubtotal := float64(item.Quantity) * item.UnitPrice // Use item.UnitPrice
			itemTotal := itemSubtotal                         // Initially, assume no discounts, tax, or fees for item

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
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID is required")
	}

	order, err := s.getOrderInternal(ctx, req.OrderId)
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
	var billingName, billingEmail, billingAddress, billingCity, billingState, billingCountry, billingPostalCode, billingPhone *string
	//var discountId pgx.NullString
	//var expiryDate pgx.NullTime
	var discountId *string
	var expiryDate *time.Time

	err := s.db.QueryRow(ctx, `
			SELECT
				id, customer_id, order_number, order_date, subtotal, discount_amount, tax_amount, fee_amount, total_amount, currency, discount_id, status, billing_name, billing_email, billing_address, billing_city, billing_state, billing_country, billing_postal_code, billing_phone, notes, ip_address, user_agent, expiry_date, created_at, updated_at
			FROM public.orders
			WHERE id = $1
		`, orderID).Scan(
		&o.Id, &o.CustomerId, &o.OrderNumber, &orderDate, &o.Subtotal, &o.DiscountAmount, &o.TaxAmount, &o.FeeAmount, &o.TotalAmount, &o.Currency, &discountId, &o.Status,
		&billingName, &billingEmail, &billingAddress, &billingCity, &billingState, &billingCountry, &billingPostalCode, &billingPhone,
		&o.Notes, &o.IpAddress, &o.UserAgent, &expiryDate, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "order with ID %s not found", orderID)
		}
		return nil, fmt.Errorf("failed to get order header: %w", err)
	}

	o.Id = orderID // Ensure ID is set correctly
	o.OrderDate = orderDate.Format(time.RFC3339)
	if expiryDate != nil {
		o.ExpiryDate = expiryDate.Format(time.RFC3339)
	}
	if discountId != nil {
		o.DiscountId = *discountId
	}
	o.CreatedAt = createdAt.Format(time.RFC3339)
	o.UpdatedAt = updatedAt.Format(time.RFC3339)

	o.BillingInfo = &pb.BillingInfo{}
	if billingName != nil {
		o.BillingInfo.Name = *billingName
	}
	if billingEmail != nil {
		o.BillingInfo.Email = *billingEmail
	}
	if billingAddress != nil {
		o.BillingInfo.Address = *billingAddress
	}
	if billingCity != nil {
		o.BillingInfo.City = *billingCity
	}
	if billingState != nil {
		o.BillingInfo.State = *billingState
	}
	if billingCountry != nil {
		o.BillingInfo.Country = *billingCountry
	}
	if billingPostalCode != nil {
		o.BillingInfo.PostalCode = *billingPostalCode
	}
	if billingPhone != nil {
		o.BillingInfo.Phone = *billingPhone
	}

	// Fetch order items
	itemRows, err := s.db.Query(ctx, `
		SELECT
			id, ticket_id, quantity, unit_price, subtotal, discount_amount, tax_amount, fee_amount, total_amount, status
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
		err := itemRows.Scan(&oi.Id, &oi.TicketId, &oi.Quantity, &oi.UnitPrice, &oi.Subtotal, &oi.DiscountAmount, &oi.TaxAmount, &oi.FeeAmount, &oi.TotalAmount, &oi.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		orderItems = append(orderItems, &oi)
	}

	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order item rows: %w", err)
	}

	o.Items = orderItems
	return &o, nil
}

func (s *Service) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" || req.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID and status are required")
	}

	now := time.Now().UTC()

	_, err := s.db.Exec(ctx, `
		UPDATE public.orders
		SET
			status = $2,
			updated_at = $3
		WHERE id = $1
	`, req.OrderId, req.Status, now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order status: %v", err)
	}

	// Fetch the updated order for the response
	updatedOrder, err := s.getOrderInternal(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve updated order: %v", err)
	}

	return &pb.OrderResponse{
		Order: updatedOrder,
	}, nil
}

func (s *Service) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID is required")
	}

	now := time.Now().UTC()

	_, err := s.db.Exec(ctx, `
		UPDATE public.orders
		SET
			status = 'cancelled',
			notes = COALESCE(notes || '\n', '') || $2,
			updated_at = $3
		WHERE id = $1
	`, req.OrderId, fmt.Sprintf("Order cancelled: %s", req.Reason), now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel order: %v", err)
	}

	return &pb.CancelOrderResponse{
		Success: true,
		Message: "Order successfully cancelled",
	}, nil
}

func (s *Service) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.ProcessPaymentResponse, error) {
	if req.OrderId == "" || req.PaymentInfo == nil {
		return nil, status.Error(codes.InvalidArgument, "order ID and payment info are required")
	}

	// In a real application, you would integrate with a payment gateway here.
	// This is a simplified example.

	transactionID := uuid.New().String()
	paymentDate := time.Now().UTC()

	_, err := s.db.Exec(ctx, `
		INSERT INTO public.payments (
			order_id, payment_method, payment_provider, transaction_id, amount, currency, status, payment_date, created_at, updated_at
		) VALUES ($1, $2, $3, $4, (SELECT total_amount FROM public.orders WHERE id = $1), (SELECT currency FROM public.orders WHERE id = $1), 'completed', $5, $6, $7)
	`, req.OrderId, req.PaymentInfo.PaymentMethod, "mock_provider", transactionID, paymentDate, paymentDate, paymentDate)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process payment: %v", err)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE public.orders
		SET
			status = 'paid',
			updated_at = $2
		WHERE id = $1
	`, req.OrderId, time.Now().UTC())

	if err != nil {
		// Consider rolling back the payment insertion in a real scenario
		return nil, status.Errorf(codes.Internal, "failed to update order status after payment: %v", err)
	}

	return &pb.ProcessPaymentResponse{
		Success:       true,
		TransactionId: transactionID,
		Message:       "Payment processed successfully",
	}, nil
}

func (s *Service) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer ID is required")
	}

	query := `
		SELECT id
		FROM public.orders
		WHERE customer_id = $1
	`
	var args []interface{}
	args = append(args, req.CustomerId)

	if req.Status != "" {
		query += " AND status = $2"
		args = append(args, req.Status)
	}

	// Add date range filtering if provided
	if req.StartDate != "" && req.EndDate != "" {
		query += " AND order_date BETWEEN $3 AND $4"
		args = append(args, req.StartDate, req.EndDate)
	} else if req.StartDate != "" {
		query += " AND order_date >= $3"
		args = append(args, req.StartDate)
	} else if req.EndDate != "" {
		query += " AND order_date <= $3"
		args = append(args, req.EndDate)
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS order_count"
	var totalCount int32
	err := s.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount) // Use QueryRow and Scan
	if err != nil {
		return nil, fmt.Errorf("failed to get total order count: %w", err)
	}

	if req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query += " ORDER BY order_date DESC LIMIT $5 OFFSET $6"
		args = append(args, req.PageSize, offset)
	} else {
		query += " ORDER BY order_date DESC"
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list order IDs by user: %w", err)
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
		Orders:     orders,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

