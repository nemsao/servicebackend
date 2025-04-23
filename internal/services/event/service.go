package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	//pgxpool "github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	//"google.golang.org/protobuf/types/known/emptypb"

	"services_app/internal/config" // Assuming your config package path
	"services_app/internal/database"
	pb "services_app/proto/event" // Assuming your proto package path
)

type Service struct {
	db  *database.PostgresDB // Use pgxpool.Pool instead of custom struct
	cfg *config.Config
	pb.UnimplementedEventServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service { // Use pgxpool.Pool
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.EventResponse, error) {
	// Validate request
	if req.Title == "" || req.StartDate == "" || req.EndDate == "" || req.RegistrationStartDate == "" || req.RegistrationEndDate == "" || req.VenueId == 0 || req.OrganizerId == "" { // Changed VenueId to 0
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
			StartDate:           startDate.Format(time.RFC3339), // Use parsed time
			EndDate:             endDate.Format(time.RFC3339),     // Use parsed time
			RegistrationStartDate: regStartDate.Format(time.RFC3339),
			RegistrationEndDate:   regEndDate.Format(time.RFC3339),
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
	if req.EventId == "" { // Changed to EventId
		return nil, status.Error(codes.InvalidArgument, "event ID is required")
	}

	var e pb.Event
	var startDate, endDate, registrationStartDate, registrationEndDate, createdAt, updatedAt time.Time

	err := s.db.QueryRow(ctx, ` // Changed to QueryRow
            SELECT 
                id, title, description, short_description, category_id, organizer_id, 
                venue_id, start_date, end_date, registration_start_date, registration_end_date, 
                is_featured, is_private, status, max_attendees, thumbnail_url, banner_url, 
                website, contact_email, contact_phone, terms_and_conditions, additional_info, 
                created_at, updated_at
            FROM public.events
            WHERE id = $1
        `, req.EventId).Scan( // Changed to req.EventId
		&e.Id, &e.Title, &e.Description, &e.ShortDescription, &e.CategoryId, &e.OrganizerId,
		&e.VenueId, &startDate, &endDate, &registrationStartDate, &registrationEndDate,
		&e.IsFeatured, &e.IsPrivate, &e.Status, &e.MaxAttendees, &e.ThumbnailUrl, &e.BannerUrl,
		&e.Website, &e.ContactEmail, &e.ContactPhone, &e.TermsAndConditions, &e.AdditionalInfo,
		&createdAt, &updatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "event with ID %s not found", req.EventId) // Changed to req.EventId
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
	if req.EventId == "" { // Changed to EventId
		return nil, status.Error(codes.InvalidArgument, "event ID is required")
	}

	e := &pb.Event{}

	err := s.db.QueryRow(ctx, `
        SELECT 
            id, title, description, short_description, category_id, organizer_id, 
            venue_id, start_date, end_date, registration_start_date, registration_end_date, 
            is_featured, is_private, status, max_attendees, thumbnail_url, banner_url, 
            website, contact_email, contact_phone, terms_and_conditions, additional_info, 
            created_at, updated_at
        FROM public.events
        WHERE id = $1
    `, req.EventId).Scan( // Use EventId from request
		&e.Id, &e.Title, &e.Description, &e.ShortDescription, &e.CategoryId, &e.OrganizerId,
		&e.VenueId, &e.StartDate, &e.EndDate, &e.RegistrationStartDate, &e.RegistrationEndDate,
		&e.IsFeatured, &e.IsPrivate, &e.Status, &e.MaxAttendees, &e.ThumbnailUrl, &e.BannerUrl,
		&e.Website, &e.ContactEmail, &e.ContactPhone, &e.TermsAndConditions, &e.AdditionalInfo,
		&e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "event with ID %s not found", req.EventId) // Use EventId
		}
		return nil, status.Errorf(codes.Internal, "failed to get event for update: %v", err)
	}

	now := time.Now().UTC()

	// Use the fields from the request, but default to the existing event data if they are not provided.
	updatedTitle := req.Title
	if updatedTitle == "" {
		updatedTitle = e.Title
	}
	updatedDescription := req.Description
	if updatedDescription == "" {
		updatedDescription = e.Description
	}
	updatedShortDescription := req.ShortDescription
	if updatedShortDescription == "" {
		updatedShortDescription = e.ShortDescription
	}
	updatedCategoryId := req.CategoryId
	if updatedCategoryId == 0 {
		updatedCategoryId = e.CategoryId
	}
	updatedVenueId := req.VenueId
	if updatedVenueId == 0 {
		updatedVenueId = e.VenueId
	}
	updatedStartDate := req.StartDate
	if updatedStartDate == "" {
		updatedStartDate = e.StartDate
	}
	updatedEndDate := req.EndDate
	if updatedEndDate == "" {
		updatedEndDate = e.EndDate
	}
	updatedRegistrationStartDate := req.RegistrationStartDate
	if updatedRegistrationStartDate == "" {
		updatedRegistrationStartDate = e.RegistrationStartDate
	}
	updatedRegistrationEndDate := req.RegistrationEndDate
	if updatedRegistrationEndDate == "" {
		updatedRegistrationEndDate = e.RegistrationEndDate
	}
	updatedIsFeatured := req.IsFeatured
	if !updatedIsFeatured {
		updatedIsFeatured = e.IsFeatured
	}
	updatedIsPrivate := req.IsPrivate
	if !updatedIsPrivate {
		updatedIsPrivate = e.IsPrivate
	}
	updatedStatus := req.Status
	if updatedStatus == "" {
		updatedStatus = e.Status
	}
	updatedMaxAttendees := req.MaxAttendees
	if updatedMaxAttendees == 0 {
		updatedMaxAttendees = e.MaxAttendees
	}
	updatedThumbnailUrl := req.ThumbnailUrl
	if updatedThumbnailUrl == "" {
		updatedThumbnailUrl = e.ThumbnailUrl
	}
	updatedBannerUrl := req.BannerUrl
	if updatedBannerUrl == "" {
		updatedBannerUrl = e.BannerUrl
	}
	updatedWebsite := req.Website
	if updatedWebsite == "" {
		updatedWebsite = e.Website
	}
	updatedContactEmail := req.ContactEmail
	if updatedContactEmail == "" {
		updatedContactEmail = e.ContactEmail
	}
	updatedContactPhone := req.ContactPhone
	if updatedContactPhone == "" {
		updatedContactPhone = e.ContactPhone
	}
	updatedTermsAndConditions := req.TermsAndConditions
	if updatedTermsAndConditions == "" {
		updatedTermsAndConditions = e.TermsAndConditions
	}
	updatedAdditionalInfo := req.AdditionalInfo
	if updatedAdditionalInfo == "" {
		updatedAdditionalInfo = e.AdditionalInfo
	}

	// Parse dates
	parsedStartDate, err := time.Parse(time.RFC3339, updatedStartDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid start_date format")
	}

	parsedEndDate, err := time.Parse(time.RFC3339, updatedEndDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid end_date format")
	}

	parsedRegStartDate, err := time.Parse(time.RFC3339, updatedRegistrationStartDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid registration_start_date format")
	}

	parsedRegEndDate, err := time.Parse(time.RFC3339, updatedRegistrationEndDate)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid registration_end_date format")
	}

	_, err = s.db.Exec(ctx, `
        UPDATE public.events 
        SET 
            title = $2, 
            description = $3, 
            short_description = $4, 
            category_id = $5, 
            venue_id = $6, 
            start_date = $7,
            end_date = $8,
            registration_start_date = $9,
            registration_end_date = $10,
            is_featured = $11, 
            is_private = $12, 
            status = $13, 
            max_attendees = $14, 
            thumbnail_url = $15, 
            banner_url = $16, 
            website = $17, 
            contact_email = $18, 
            contact_phone = $19, 
            terms_and_conditions = $20, 
            additional_info = $21,
            updated_at = $22
        WHERE id = $1
    `, req.EventId, updatedTitle, updatedDescription, updatedShortDescription, updatedCategoryId, updatedVenueId,
		parsedStartDate, parsedEndDate, parsedRegStartDate, parsedRegEndDate,
		updatedIsFeatured, updatedIsPrivate, updatedStatus, updatedMaxAttendees, updatedThumbnailUrl, updatedBannerUrl,
		updatedWebsite, updatedContactEmail, updatedContactPhone, updatedTermsAndConditions, updatedAdditionalInfo, now)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update event: %v", err)
	}

	// Fetch the updated event to return
	updatedEvent, err := s.GetEvent(ctx, &pb.GetEventRequest{EventId: req.EventId})
	if err != nil {
		return nil, err // Return the error from GetEvent
	}

	return updatedEvent, nil
}

func (s *Service) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) { // Changed to emptypb.Empty
	if req.EventId == "" { // Changed to EventId
		return nil, status.Error(codes.InvalidArgument, "event ID is required")
	}

	_, err := s.db.Exec(ctx, `
        DELETE FROM public.events 
        WHERE id = $1
    `, req.EventId) // Changed to EventId

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete event: %v", err)
	}

	return &pb.DeleteEventResponse{Success:true}, nil 
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
		Events:    events,
		TotalCount: int32(len(events)), // Basic total count, adjust as needed with SQL COUNT
		Page:      req.Page,
		PageSize:  req.PageSize,
	}, nil
}

