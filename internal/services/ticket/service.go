package ticket

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nemsao/servicebackend/internal/database"
	"github.com/nemsao/servicebackend/proto/ticket_service"
)
type Service struct {
	db  *database.PostgresDB
	cfg *config.Config
	ticket.UnimplementedTicketServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) CreateTicket(ctx context.Context, req *ticket.CreateTicketRequest) (*ticket.TicketResponse, error) {
	// Validate request
	if req.EventId == "" || req.TicketTypeId == "" || req.Name == "" || req.Price <= 0 {
		return nil, status.Error(codes.InvalidArgument, "event_id, ticket_type_id, name, and price are required")
	}

	// Parse dates
	salesStartDate, err := time.Parse(time.RFC3339, req.SalesStartDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sales_start_date format")
	}

	salesEndDate, err := time.Parse(time.RFC3339, req.SalesEndDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sales_end_date format")
	}

	// Create ticket
	ticketID := uuid.New().String()
	now := time.Now().UTC()

	err = s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Insert ticket
		_, err := tx.Exec(ctx, `
			INSERT INTO public.tickets (
				id, event_id, ticket_type_id, name, description, price,
				currency, max_tickets_per_order, is_transferable, is_refundable,
				refund_policy, sales_start_date, sales_end_date, status,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, ticketID, req.EventId, req.TicketTypeId, req.Name, req.Description,
			req.Price, req.Currency, req.MaxTicketsPerOrder, req.IsTransferable,
			req.IsRefundable, req.RefundPolicy, salesStartDate, salesEndDate,
			"active", now, now)

		if err != nil {
			return err
		}

		// Create inventory
		_, err = tx.Exec(ctx, `
			INSERT INTO public.ticket_inventory (
				ticket_id, total_quantity, available_quantity
			) VALUES ($1, $2, $3)
		`, ticketID, req.TotalQuantity, req.TotalQuantity)

		return err
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create ticket")
	}

	// Return response
	return &ticket.TicketResponse{
		Ticket: &ticket.Ticket{
			Id:                 ticketID,
			EventId:           req.EventId,
			TicketTypeId:      req.TicketTypeId,
			Name:              req.Name,
			Description:       req.Description,
			Price:             req.Price,
			Currency:          req.Currency,
			MaxTicketsPerOrder: req.MaxTicketsPerOrder,
			IsTransferable:    req.IsTransferable,
			IsRefundable:      req.IsRefundable,
			RefundPolicy:      req.RefundPolicy,
			SalesStartDate:    req.SalesStartDate,
			SalesEndDate:      req.SalesEndDate,
			Status:            "active",
			CreatedAt:         now.Format(time.RFC3339),
			UpdatedAt:         now.Format(time.RFC3339),
		},
	}, nil
}

func (s *Service) GetTicket(ctx context.Context, req *ticket.GetTicketRequest) (*ticket.TicketResponse, error) {
	// Validate request
	if req.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id is required")
	}

	// Get ticket
	var (
		ticketID           string
		eventID           string
		ticketTypeID      string
		name              string
		description       string
		price             float64
		currency          string
		maxTicketsPerOrder int32
		isTransferable    bool
		isRefundable      bool
		refundPolicy      string
		salesStartDate    time.Time
		salesEndDate      time.Time
		status            string
		createdAt         time.Time
		updatedAt         time.Time
	)

	err := s.db.QueryRow(ctx, `
		SELECT id, event_id, ticket_type_id, name, description, price,
		       currency, max_tickets_per_order, is_transferable, is_refundable,
		       refund_policy, sales_start_date, sales_end_date, status,
		       created_at, updated_at
		FROM public.tickets
		WHERE id = $1
	`, req.TicketId).Scan(&ticketID, &eventID, &ticketTypeID, &name, &description,
		&price, &currency, &maxTicketsPerOrder, &isTransferable, &isRefundable,
		&refundPolicy, &salesStartDate, &salesEndDate, &status, &createdAt, &updatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Error(codes.NotFound, "ticket not found")
		}
		return nil, status.Error(codes.Internal, "failed to get ticket")
	}

	// Return response
	return &ticket.TicketResponse{
		Ticket: &ticket.Ticket{
			Id:                 ticketID,
			EventId:           eventID,
			TicketTypeId:      ticketTypeID,
			Name:              name,
			Description:       description,
			Price:             price,
			Currency:          currency,
			MaxTicketsPerOrder: maxTicketsPerOrder,
			IsTransferable:    isTransferable,
			IsRefundable:      isRefundable,
			RefundPolicy:      refundPolicy,
			SalesStartDate:    salesStartDate.Format(time.RFC3339),
			SalesEndDate:      salesEndDate.Format(time.RFC3339),
			Status:            status,
			CreatedAt:         createdAt.Format(time.RFC3339),
			UpdatedAt:         updatedAt.Format(time.RFC3339),
		},
	}, nil
}

func (s *Service) ListTickets(ctx context.Context, req *ticket.ListTicketsRequest) (*ticket.ListTicketsResponse, error) {
	// Validate request
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}

	// Set default pagination
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Build query
	query := `
		SELECT id, event_id, ticket_type_id, name, description, price,
		       currency, max_tickets_per_order, is_transferable, is_refundable,
		       refund_policy, sales_start_date, sales_end_date, status,
		       created_at, updated_at
		FROM public.tickets
		WHERE event_id = $1
	`
	args := []interface{}{req.EventId}
	argCount := 2

	if req.Status != "" {
		query += ` AND status = $` + strconv.Itoa(argCount)
		args = append(args, req.Status)
		argCount++
	}

	// Add pagination
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argCount) + ` OFFSET $` + strconv.Itoa(argCount+1)
	args = append(args, req.PageSize, (req.Page-1)*req.PageSize)

	// Execute query
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list tickets")
	}
	defer rows.Close()

	// Get total count
	var totalCount int32
	err = s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.tickets
		WHERE event_id = $1
	`, req.EventId).Scan(&totalCount)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get total count")
	}

	// Process results
	var tickets []*ticket.Ticket
	for rows.Next() {
		var (
			ticketID           string
			eventID           string
			ticketTypeID      string
			name              string
			description       string
			price             float64
			currency          string
			maxTicketsPerOrder int32
			isTransferable    bool
			isRefundable      bool
			refundPolicy      string
			salesStartDate    time.Time
			salesEndDate      time.Time
			status            string
			createdAt         time.Time
			updatedAt         time.Time
		)

		err := rows.Scan(&ticketID, &eventID, &ticketTypeID, &name, &description,
			&price, &currency, &maxTicketsPerOrder, &isTransferable, &isRefundable,
			&refundPolicy, &salesStartDate, &salesEndDate, &status, &createdAt, &updatedAt)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to scan ticket")
		}

		tickets = append(tickets, &ticket.Ticket{
			Id:                 ticketID,
			EventId:           eventID,
			TicketTypeId:      ticketTypeID,
			Name:              name,
			Description:       description,
			Price:             price,
			Currency:          currency,
			MaxTicketsPerOrder: maxTicketsPerOrder,
			IsTransferable:    isTransferable,
			IsRefundable:      isRefundable,
			RefundPolicy:      refundPolicy,
			SalesStartDate:    salesStartDate.Format(time.RFC3339),
			SalesEndDate:      salesEndDate.Format(time.RFC3339),
			Status:            status,
			CreatedAt:         createdAt.Format(time.RFC3339),
			UpdatedAt:         updatedAt.Format(time.RFC3339),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, status.Error(codes.Internal, "failed to process tickets")
	}

	// Return response
	return &ticket.ListTicketsResponse{
		Tickets:    tickets,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (s *Service) UpdateTicket(ctx context.Context, req *ticket.UpdateTicketRequest) (*ticket.TicketResponse, error) {
	// Validate request
	if req.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id is required")
	}

	// Parse dates if provided
	var salesStartDate, salesEndDate time.Time
	var err error

	if req.SalesStartDate != "" {
		salesStartDate, err = time.Parse(time.RFC3339, req.SalesStartDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid sales_start_date format")
		}
	}

	if req.SalesEndDate != "" {
		salesEndDate, err = time.Parse(time.RFC3339, req.SalesEndDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid sales_end_date format")
		}
	}

	// Update ticket
	now := time.Now().UTC()
	query := `
		UPDATE public.tickets
		SET name = COALESCE($2, name),
		    description = COALESCE($3, description),
		    price = COALESCE($4, price),
		    currency = COALESCE($5, currency),
		    max_tickets_per_order = COALESCE($6, max_tickets_per_order),
		    is_transferable = COALESCE($7, is_transferable),
		    is_refundable = COALESCE($8, is_refundable),
		    refund_policy = COALESCE($9, refund_policy),
		    sales_start_date = COALESCE($10, sales_start_date),
		    sales_end_date = COALESCE($11, sales_end_date),
		    status = COALESCE($12, status),
		    updated_at = $13
		WHERE id = $1
		RETURNING id, event_id, ticket_type_id, name, description, price,
		          currency, max_tickets_per_order, is_transferable, is_refundable,
		          refund_policy, sales_start_date, sales_end_date, status,
		          created_at, updated_at
	`

	var (
		ticketID           string
		eventID           string
		ticketTypeID      string
		name              string
		description       string
		price             float64
		currency          string
		maxTicketsPerOrder int32
		isTransferable    bool
		isRefundable      bool
		refundPolicy      string
		updatedSalesStartDate time.Time
		updatedSalesEndDate   time.Time
		status            string
		createdAt         time.Time
		updatedAt         time.Time
	)

	err = s.db.QueryRow(ctx, query,
		req.TicketId, req.Name, req.Description, req.Price, req.Currency,
		req.MaxTicketsPerOrder, req.IsTransferable, req.IsRefundable,
		req.RefundPolicy, salesStartDate, salesEndDate, req.Status, now,
	).Scan(&ticketID, &eventID, &ticketTypeID, &name, &description,
		&price, &currency, &maxTicketsPerOrder, &isTransferable, &isRefundable,
		&refundPolicy, &updatedSalesStartDate, &updatedSalesEndDate, &status,
		&createdAt, &updatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Error(codes.NotFound, "ticket not found")
		}
		return nil, status.Error(codes.Internal, "failed to update ticket")
	}

	// Return response
	return &ticket.TicketResponse{
		Ticket: &ticket.Ticket{
			Id:                 ticketID,
			EventId:           eventID,
			TicketTypeId:      ticketTypeID,
			Name:              name,
			Description:       description,
			Price:             price,
			Currency:          currency,
			MaxTicketsPerOrder: maxTicketsPerOrder,
			IsTransferable:    isTransferable,
			IsRefundable:      isRefundable,
			RefundPolicy:      refundPolicy,
			SalesStartDate:    updatedSalesStartDate.Format(time.RFC3339),
			SalesEndDate:      updatedSalesEndDate.Format(time.RFC3339),
			Status:            status,
			CreatedAt:         createdAt.Format(time.RFC3339),
			UpdatedAt:         updatedAt.Format(time.RFC3339),
		},
	}, nil
}

func (s *Service) DeleteTicket(ctx context.Context, req *ticket.DeleteTicketRequest) (*ticket.DeleteTicketResponse, error) {
	// Validate request
	if req.TicketId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id is required")
	}

	// Delete ticket
	result, err := s.db.Exec(ctx, `
		DELETE FROM public.tickets
		WHERE id = $1
	`, req.TicketId)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete ticket")
	}

	if result.RowsAffected() == 0 {
		return nil, status.Error(codes.NotFound, "ticket not found")
	}

	return &ticket.DeleteTicketResponse{
		Success: true,
	}, nil
}

func (s *Service) CheckAvailability(ctx context.Context, req *ticket.CheckAvailabilityRequest) (*ticket.CheckAvailabilityResponse, error) {
	// Validate request
	if req.TicketId == "" || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "ticket_id and quantity are required")
	}

	// Check availability
	var (
		availableQuantity int32
		totalQuantity    int32
		reservedQuantity int32
		soldQuantity     int32
	)

	err := s.db.QueryRow(ctx, `
		SELECT available_quantity, total_quantity, reserved_quantity, sold_quantity
		FROM public.ticket_inventory
		WHERE ticket_id = $1
	`, req.TicketId).Scan(&availableQuantity, &totalQuantity, &reservedQuantity, &soldQuantity)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Error(codes.NotFound, "ticket not found")
		}
		return nil, status.Error(codes.Internal, "failed to check availability")
	}

	// Check if requested quantity is available
	available := availableQuantity >= req.Quantity

	return &ticket.CheckAvailabilityResponse{
		Available:         available,
		AvailableQuantity: availableQuantity,
		Message:           "ticket availability checked",
	}, nil
}

func (s *Service) ReserveTickets(ctx context.Context, req *ticket.ReserveTicketsRequest) (*ticket.ReserveTicketsResponse, error) {
	// Validate request
	if req.TicketId == "" || req.Quantity <= 0 || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "ticket_id, quantity, and user_id are required")
	}

	// Reserve tickets
	reservationID := uuid.New().String()
	expiresAt := time.Now().Add(15 * time.Minute) // 15 minutes reservation window

	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Check availability
		var availableQuantity int32
		err := tx.QueryRow(ctx, `
			SELECT available_quantity
			FROM public.ticket_inventory
			WHERE ticket_id = $1
			FOR UPDATE
		`, req.TicketId).Scan(&availableQuantity)

		if err != nil {
			if err == pgx.ErrNoRows {
				return status.Error(codes.NotFound, "ticket not found")
			}
			return status.Error(codes.Internal, "failed to check availability")
		}

		if availableQuantity < req.Quantity {
			return status.Error(codes.FailedPrecondition, "not enough tickets available")
		}

		// Update inventory
		_, err = tx.Exec(ctx, `
			UPDATE public.ticket_inventory
			SET available_quantity = available_quantity - $1,
			    reserved_quantity = reserved_quantity + $1,
			    last_updated = CURRENT_TIMESTAMP
			WHERE ticket_id = $2
		`, req.Quantity, req.TicketId)

		if err != nil {
			return status.Error(codes.Internal, "failed to update inventory")
		}

		// Create reservation
		_, err = tx.Exec(ctx, `
			INSERT INTO public.ticket_reservations (
				id, ticket_id, user_id, quantity, expires_at
			) VALUES ($1, $2, $3, $4, $5)
		`, reservationID, req.TicketId, req.UserId, req.Quantity, expiresAt)

		return err
	})

	if err != nil {
		return nil, err
	}

	return &ticket.ReserveTicketsResponse{
		Success:       true,
		ReservationId: reservationID,
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		Message:       "tickets reserved successfully",
	}, nil
} 