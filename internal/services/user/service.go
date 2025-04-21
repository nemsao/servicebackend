package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nemsao/servicebackend/internal/database"
	"github.com/nemsao/servicebackend/internal/services/auth"
	"github.com/nemsao/servicebackend/proto/user_service"
)
type Service struct {
	db  *database.PostgresDB
	cfg *config.Config
	user.UnimplementedUserServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) RegisterUser(ctx context.Context, req *user.RegisterUserRequest) (*user.UserResponse, error) {
	// Validate request
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email, and password are required")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	// Create user
	userID := uuid.New().String()
	now := time.Now().UTC()

	err = s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Insert user
		_, err := tx.Exec(ctx, `
			INSERT INTO public.users (
				id, username, email, password_hash, first_name, last_name,
				phone_number, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, userID, req.Username, req.Email, hashedPassword, req.FirstName,
			req.LastName, req.PhoneNumber, now, now)

		if err != nil {
			return err
		}

		// Assign customer role
		_, err = tx.Exec(ctx, `
			INSERT INTO public.user_roles (user_id, role_id, assigned_at)
			SELECT $1, id, $2
			FROM public.roles
			WHERE name = 'customer'
		`, userID, now)

		return err
	})

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint" {
			return nil, status.Error(codes.AlreadyExists, "username or email already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	// Generate JWT token
	token, err := auth.GenerateToken(userID, s.cfg.JWT.SecretKey, s.cfg.JWT.AccessExpiry)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// Return response
	return &user.UserResponse{
		User: &user.User{
			Id:        userID,
			Username:  req.Username,
			Email:     req.Email,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			PhoneNumber: req.PhoneNumber,
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
		},
		Token: token,
	}, nil
}

func (s *Service) LoginUser(ctx context.Context, req *user.LoginRequest) (*user.LoginResponse, error) {
	// Validate request
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Get user
	var (
		userID       string
		username     string
		passwordHash string
		firstName    string
		lastName     string
		phoneNumber  string
		createdAt    time.Time
		updatedAt    time.Time
	)

	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, first_name, last_name,
		       phone_number, created_at, updated_at
		FROM public.users
		WHERE email = $1 AND is_active = true
	`, req.Email).Scan(&userID, &username, &passwordHash, &firstName,
		&lastName, &phoneNumber, &createdAt, &updatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid password")
	}

	// Generate JWT token
	token, err := auth.GenerateToken(userID, s.cfg.JWT.SecretKey, s.cfg.JWT.AccessExpiry)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// Return response
	return &user.LoginResponse{
		Token: token,
		User: &user.User{
			Id:          userID,
			Username:    username,
			Email:       req.Email,
			FirstName:   firstName,
			LastName:    lastName,
			PhoneNumber: phoneNumber,
			CreatedAt:   createdAt.Format(time.RFC3339),
			UpdatedAt:   updatedAt.Format(time.RFC3339),
		},
	}, nil
}

func (s *Service) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.UserResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Get user
	var (
		username     string
		email        string
		firstName    string
		lastName     string
		phoneNumber  string
		profileImage string
		dateOfBirth  *time.Time
		isVerified   bool
		isActive     bool
		createdAt    time.Time
		updatedAt    time.Time
	)

	err := s.db.QueryRow(ctx, `
		SELECT username, email, first_name, last_name, phone_number,
		       profile_image_url, date_of_birth, is_verified, is_active,
		       created_at, updated_at
		FROM public.users
		WHERE id = $1
	`, req.UserId).Scan(&username, &email, &firstName, &lastName, &phoneNumber,
		&profileImage, &dateOfBirth, &isVerified, &isActive, &createdAt, &updatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Return response
	user := &user.User{
		Id:             req.UserId,
		Username:       username,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		PhoneNumber:    phoneNumber,
		ProfileImageUrl: profileImage,
		IsVerified:     isVerified,
		IsActive:       isActive,
		CreatedAt:      createdAt.Format(time.RFC3339),
		UpdatedAt:      updatedAt.Format(time.RFC3339),
	}

	if dateOfBirth != nil {
		user.DateOfBirth = dateOfBirth.Format(time.RFC3339)
	}

	return &user.UserResponse{User: user}, nil
}

func (s *Service) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UserResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Update user
	now := time.Now().UTC()

	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE public.users
			SET first_name = COALESCE($1, first_name),
			    last_name = COALESCE($2, last_name),
			    phone_number = COALESCE($3, phone_number),
			    profile_image_url = COALESCE($4, profile_image_url),
			    updated_at = $5
			WHERE id = $6
		`, req.FirstName, req.LastName, req.PhoneNumber,
			req.ProfileImageUrl, now, req.UserId)

		return err
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	// Get updated user
	return s.GetUser(ctx, &user.GetUserRequest{UserId: req.UserId})
}

func (s *Service) DeleteUser(ctx context.Context, req *user.DeleteUserRequest) (*user.DeleteUserResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Delete user
	err := s.db.Transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE public.users
			SET is_active = false,
			    updated_at = $1
			WHERE id = $2
		`, time.Now().UTC(), req.UserId)

		return err
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to delete user")
	}

	return &user.DeleteUserResponse{Success: true}, nil
} 