package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nemsao/servicebackend/internal/database"
	"github.com/nemsao/servicebackend/internal/config" // Assuming your config package path
	pb "github.com/nemsao/servicebackend/proto/event_service"   // Assuming your proto package path
)

type Service struct {
	db  *database.PostgresDB
	cfg *config.Config
	pb.UnimplementedEventServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.EventResponse, error) {
	// Validate request
	if req.Title == "" || req.StartDate == "" || req.EndDate == "" || req.RegistrationStartDate == "" || req.RegistrationEndDate == "" || req.VenueId == "" || req.OrganizerId == "" {
		return nil, status.Error(codes.InvalidArgument, "title, start_date, end_date, registration_start_date, registration_end_date, venue_id, and organizer_id are required")
	}

	// Parse dates
	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid start_date format")
	}

	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid end_date format")
	}

	regStartDate, err := time.Parse(time.RFC3339, req.RegistrationStartDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid registration_start_date format")
	}

	regEndDate, err := time.Parse(time.RFC3339, req.RegistrationEndDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid registration_end_date format")
	}

	// Create event
	eventID := uuid.New().String()
	now := time.Now().UTC()

	_, err = s.db.Exec(ctx, `
		INSERT INTO public.events (
			id, title, description, short_description, category_id, organizer_id, 
			venue_id, start_date, end_date, registration_start_date, registration_end_date, 
			is_featured, is_private, status, max_attendees, thumbnail_url, banner_url, 
			website, contact_email, contact_phone, terms_and_conditions, additional_info, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`, eventID, req.Title, req.Description, req.ShortDescription, req.CategoryId, req.OrganizerId,
		req.VenueId, startDate, endDate, regStartDate, regEndDate,
		req.IsFeatured, req.IsPrivate, "draft", req.MaxAttendees, req.ThumbnailUrl, req.BannerUrl,
		req.Website, req.ContactEmail, req.ContactPhone, req.TermsAndConditions, req.AdditionalInfo,
		now, now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create event: %v", err)
	}

	// Return response
	return &pb.EventResponse{
		Event: &pb.Event{
			Id:                  eventID,
			Title:               req.Title,
			Description:         req.Description,
			ShortDescription:    req.ShortDescription,
			CategoryId:          req.CategoryId,
			OrganizerId:         req.OrganizerId,
			VenueId:             req.VenueId,
			StartDate:           req.StartDate,
			EndDate:             req.EndDate,
			RegistrationStartDate: req.RegistrationStartDate,
			RegistrationEndDate:   req.RegistrationEndDate,
			IsFeatured:          req.IsFeatured,
			IsPrivate:           req.IsPrivate,
			Status:              "draft",
			MaxAttendees:        req.MaxAttendees,
			ThumbnailUrl:        req.ThumbnailUrl,
			BannerUrl:           req.BannerUrl,
			Website:             req.Website,
			ContactEmail:        req.ContactEmail,
			ContactPhone:        req.ContactPhone,
			TermsAndConditions:  req.TermsAndConditions,
			AdditionalInfo:      req.AdditionalInfo,
			CreatedAt:           now.Format(time.RFC3339),
			UpdatedAt:           now.Format(time.RFC3339),
		},
	}, nil
}

func (s *Service) GetEvent(ctx context.Context, req *pb.GetEventRequest) (*pb.EventResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "event ID is required")
	}

	var e pb.Event
	var startDate, endDate, registrationStartDate, registrationEndDate, createdAt, updatedAt time.Time

	err := s.db.Get(ctx, &e.Id, &e.Title, &e.Description, &e.ShortDescription, &e.CategoryId, &e.OrganizerId,
		&e.VenueId, &startDate, &endDate, &registrationStartDate, &registrationEndDate,
		&e.IsFeatured, &e.IsPrivate, &e.Status, &e.MaxAttendees, &e.ThumbnailUrl, &e.BannerUrl,
		&e.Website, &e.ContactEmail, &e.ContactPhone, &e.TermsAndConditions, &e.AdditionalInfo,
		&createdAt, &updatedAt,
		`
			SELECT 
				id, title, description, short_description, category_id, organizer_id, 
				venue_id, start_date, end_date, registration_start_date, registration_end_date, 
				is_featured, is_private, status, max_attendees, thumbnail_url, banner_url, 
				website, contact_email, contact_phone, terms_and_conditions, additional_info, 
				created_at, updated_at
			FROM public.events
			WHERE id = $1
		`, req.Id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "event with ID %s not found", req.Id)
		}
		return nil, status.Errorf(codes.Internal, "failed to get event: %v", err)
	}

	e.StartDate = startDate.Format(time.RFC3339)
	e.EndDate = endDate.Format(time.RFC3339)
	e.RegistrationStartDate = registrationStartDate.Format(time.RFC3339)
	e.RegistrationEndDate = registrationEndDate.Format(time.RFC3339)
	e.CreatedAt = createdAt.Format(time.RFC3339)
	e.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &pb.EventResponse{
		Event: &e,
	}, nil
}

func (s *Service) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.EventResponse, error) {
	if req.Event == nil || req.Event.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "event and event ID are required")
	}

	e := req.Event
	now := time.Now().UTC()

	// Parse dates if provided
	var startDate time.Time
	if e.StartDate != "" {
		var err error
		startDate, err = time.Parse(time.RFC3339, e.StartDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid start_date format")
		}
	}

	var endDate time.Time
	if e.EndDate != "" {
		var err error
		endDate, err = time.Parse(time.RFC3339, e.EndDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid end_date format")
		}
	}

	var regStartDate time.Time
	if e.RegistrationStartDate != "" {
		var err error
		regStartDate, err = time.Parse(time.RFC3339, e.RegistrationStartDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid registration_start_date format")
		}
	}

	var regEndDate time.Time
	if e.RegistrationEndDate != "" {
		var err error
		regEndDate, err = time.Parse(time.RFC3339, e.RegistrationEndDate)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid registration_end_date format")
		}
	}

	_, err := s.db.Exec(ctx, `
		UPDATE public.events 
		SET 
			title = $2, 
			description = $3, 
			short_description = $4, 
			category_id = $5, 
			organizer_id = $6, 
			venue_id = $7, 
			start_date = CASE WHEN $8::TIMESTAMP IS NULL THEN start_date ELSE $8 END,
			end_date = CASE WHEN $9::TIMESTAMP IS NULL THEN end_date ELSE $9 END,
			registration_start_date = CASE WHEN $10::TIMESTAMP IS NULL THEN registration_start_date ELSE $10 END,
			registration_end_date = CASE WHEN $11::TIMESTAMP IS NULL THEN registration_end_date ELSE $11 END,
			is_featured = $12, 
			is_private = $13, 
			status = $14, 
			max_attendees = $15, 
			thumbnail_url = $16, 
			banner_url = $17, 
			website = $18, 
			contact_email = $19, 
			contact_phone = $20, 
			terms_and_conditions = $21, 
			additional_info = $22,
			updated_at = $23
		WHERE id = $1
	`, e.Id, e.Title, e.Description, e.ShortDescription, e.CategoryId, e.OrganizerId,
		e.VenueId, nullIfZero(startDate), nullIfZero(endDate), nullIfZero(regStartDate), nullIfZero(regEndDate),
		e.IsFeatured, e.IsPrivate, e.Status, e.MaxAttendees, e.ThumbnailUrl, e.BannerUrl,
		e.Website, e.ContactEmail, e.ContactPhone, e.TermsAndConditions, e.AdditionalInfo, now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update event: %v", err)
	}

	e.UpdatedAt = now.Format(time.RFC3339)

	return &pb.EventResponse{
		Event: e,
	}, nil
}

func (s *Service) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.Empty, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "event ID is required")
	}

	_, err := s.db.Exec(ctx, `
		DELETE FROM public.events 
		WHERE id = $1
	`, req.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete event: %v", err)
	}

	return &pb.Empty{}, nil
}

func (s *Service) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	query := `
		SELECT 
			id, title, description, short_description, category_id, organizer_id, 
			venue_id, start_date, end_date, registration_start_date, registration_end_date, 
			is_featured, is_private, status, max_attendees, thumbnail_url, banner_url, 
			website, contact_email, contact_phone, terms_and_conditions, additional_info, 
			created_at, updated_at
		FROM public.events
	`
	var args []interface{}

	if req.OrganizerId != "" {
		query += " WHERE organizer_id = $1"
		args = append(args, req.OrganizerId)
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list events: %v", err)
	}
	defer rows.Close()

	var events []*pb.Event
	for rows.Next() {
		var e pb.Event
		var startDate, endDate, registrationStartDate, registrationEndDate, createdAt, updatedAt time.Time
		err := rows.Scan(&e.Id, &e.Title, &e.Description, &e.ShortDescription, &e.CategoryId, &e.OrganizerId,
			&e.VenueId, &startDate, &endDate, &registrationStartDate, &registrationEndDate,
			&e.IsFeatured, &e.IsPrivate, &e.Status, &e.MaxAttendees, &e.ThumbnailUrl, &e.BannerUrl,
			&e.Website, &e.ContactEmail, &e.ContactPhone, &e.TermsAndConditions, &e.AdditionalInfo,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan event: %v", err)
		}
		e.StartDate = startDate.Format(time.RFC3339)
		e.EndDate = endDate.Format(time.RFC3339)
		e.RegistrationStartDate = registrationStartDate.Format(time.RFC3339)
		e.RegistrationEndDate = registrationEndDate.Format(time.RFC3339)
		e.CreatedAt = createdAt.Format(time.RFC3339)
		e.UpdatedAt = updatedAt.Format(time.RFC3339)
		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "error iterating event rows: %v", err)
	}

	return &pb.ListEventsResponse{
		Events: events,
	}, nil
}

// Helper function to handle zero time values in updates
func nullIfZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}