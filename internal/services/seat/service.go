package seat

import (
	"context"
	"time"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

import (
	"services_app/internal/database"
	"services_app/internal/config"
)

import (
	seat "services_app/proto/seat"
)

type Service struct {
	db  *database.PostgresDB
	redis *database.RedisClient
	cfg *config.Config
	seat.UnimplementedSeatServiceServer
}

func NewService(db *database.PostgresDB,redis *database.RedisClient, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		redis: redis,
		cfg: cfg,
	}
}

func (s *Service) CreateSeat(ctx context.Context, req *seat.CreateSeatRequest) (*seat.CreateSeatResponse, error) {
	// Validate request
	if req.EventId == "" || req.Name == "" || req.RowNumber == "" || req.SeatNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id, name, row_number, and seat_number are required")
	}

	// Create seat
	seatID := uuid.New().String()
	now := time.Now().UTC()
	nowUnix := now.Unix()

	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Insert seat
		_, err := tx.Exec(ctx, `
			INSERT INTO public.seats (
				id, event_id, name, row_number, seat_number, coordinates,
				attributes, is_available, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, seatID, req.EventId, req.Name, req.RowNumber, req.SeatNumber,
			req.Coordinates, req.Attributes, true, nowUnix, nowUnix)

		return err
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create seat: "+err.Error())
	}

	// Return response
	return &seat.CreateSeatResponse{
		Seat: &seat.Seat{
			Id:          seatID,
			EventId:     req.EventId,
			Name:        req.Name,
			RowNumber:   req.RowNumber,
			SeatNumber:  req.SeatNumber,
			Coordinates: req.Coordinates,
			Attributes:  req.Attributes,
			IsAvailable: true,
			CreatedAt:   nowUnix,
			UpdatedAt:   nowUnix,
		},
	}, nil
}

func (s *Service) UpdateSeat(ctx context.Context, req *seat.UpdateSeatRequest) (*seat.UpdateSeatResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "seat id is required")
	}

	// Check if seat exists
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM public.seats WHERE id = $1)
	`, req.Id).Scan(&exists)

	if err != nil {
		return nil, status.Error(codes.Internal, "database query failed: "+err.Error())
	}

	if !exists {
		return nil, status.Error(codes.NotFound, "seat not found")
	}

	// Update seat
	now := time.Now().UTC()
	nowUnix := now.Unix()

	err = s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE public.seats SET
				name = $1,
				row_number = $2,
				seat_number = $3,
				coordinates = $4,
				attributes = $5,
				is_available = $6,
				updated_at = $7
			WHERE id = $8
		`, req.Name, req.RowNumber, req.SeatNumber, req.Coordinates,
			req.Attributes, req.IsAvailable, nowUnix, req.Id)

		return err
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update seat: "+err.Error())
	}

	// Get updated seat
	var seatData seat.Seat
	err = s.db.QueryRow(ctx, `
		SELECT id, event_id, name, row_number, seat_number, coordinates,
			   is_available, created_at, updated_at
		FROM public.seats
		WHERE id = $1
	`, req.Id).Scan(
		&seatData.Id, &seatData.EventId, &seatData.Name, &seatData.RowNumber, &seatData.SeatNumber,
		&seatData.Coordinates, &seatData.IsAvailable, &seatData.CreatedAt, &seatData.UpdatedAt,
	)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve updated seat: "+err.Error())
	}

	// Retrieve attributes separately
	attributes := make(map[string]string)
	// Logic to fetch attributes from database
	// This would depend on how you store the map in your database
	
	seatData.Attributes = attributes

	return &seat.UpdateSeatResponse{
		Seat: &seatData,
	}, nil
}

func (s *Service) DeleteSeat(ctx context.Context, req *seat.DeleteSeatRequest) (*seat.DeleteSeatResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "seat id is required")
	}

	// Delete seat
	result, err := s.db.Exec(ctx, `
		DELETE FROM public.seats WHERE id = $1
	`, req.Id)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete seat: "+err.Error())
	}

	if result.RowsAffected() == 0 {
		return nil, status.Error(codes.NotFound, "seat not found")
	}

	return &seat.DeleteSeatResponse{
		Success: true,
	}, nil
}

func (s *Service) GetSeat(ctx context.Context, req *seat.GetSeatRequest) (*seat.GetSeatResponse, error) {
	// Validate request
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "seat id is required")
	}

	// Get seat
	var seatData seat.Seat
	err := s.db.QueryRow(ctx, `
		SELECT id, event_id, name, row_number, seat_number, coordinates,
			   is_available, created_at, updated_at
		FROM public.seats
		WHERE id = $1
	`, req.Id).Scan(
		&seatData.Id, &seatData.EventId, &seatData.Name, &seatData.RowNumber, &seatData.SeatNumber,
		&seatData.Coordinates, &seatData.IsAvailable, &seatData.CreatedAt, &seatData.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Error(codes.NotFound, "seat not found")
		}
		return nil, status.Error(codes.Internal, "database query failed: "+err.Error())
	}

	// Retrieve attributes separately
	attributes := make(map[string]string)
	// Logic to fetch attributes from database
	// This would depend on how you store the map in your database
	
	seatData.Attributes = attributes

	return &seat.GetSeatResponse{
		Seat: &seatData,
	}, nil
}

func (s *Service) ListSeatsByEvent(ctx context.Context, req *seat.ListSeatsByEventRequest) (*seat.ListSeatsByEventResponse, error) {
	// Validate request
	if req.EventId == "" {
		return nil, status.Error(codes.InvalidArgument, "event_id is required")
	}

	// Query seats
	rows, err := s.db.Query(ctx, `
		SELECT id, event_id, name, row_number, seat_number, coordinates,
			   is_available, created_at, updated_at
		FROM public.seats
		WHERE event_id = $1
		ORDER BY row_number, seat_number
	`, req.EventId)

	if err != nil {
		return nil, status.Error(codes.Internal, "database query failed: "+err.Error())
	}
	defer rows.Close()

	// Build response
	var seats []*seat.Seat
	for rows.Next() {
		var s seat.Seat
		err := rows.Scan(
			&s.Id, &s.EventId, &s.Name, &s.RowNumber, &s.SeatNumber,
			&s.Coordinates, &s.IsAvailable, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to scan seat row: "+err.Error())
		}

		// Retrieve attributes separately for each seat
		attributes := make(map[string]string)
		// Logic to fetch attributes from database
		// This would depend on how you store the map in your database
		
		s.Attributes = attributes
		
		seats = append(seats, &s)
	}

	if err = rows.Err(); err != nil {
		return nil, status.Error(codes.Internal, "error iterating seats: "+err.Error())
	}

	return &seat.ListSeatsByEventResponse{
		Seats: seats,
	}, nil
}