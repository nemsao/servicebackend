package auth

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt"
	//"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	
)
import("services_app/internal/database")
import("services_app/proto/auth")
import("services_app/internal/config")
type Service struct {
	db  *database.PostgresDB
	cfg *config.Config
	auth.UnimplementedAuthServiceServer
}

func NewService(db *database.PostgresDB, cfg *config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

// GenerateToken generates a new JWT token
func GenerateToken(userID string, secretKey string, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(expiry).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ValidateToken validates a JWT token
func ValidateToken(tokenString string, secretKey string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token signing method")
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return "", status.Error(codes.Unauthenticated, "invalid token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			return "", status.Error(codes.Unauthenticated, "invalid token claims")
		}
		return userID, nil
	}

	return "", status.Error(codes.Unauthenticated, "invalid token")
}

func (s *Service) ValidateToken(ctx context.Context, req *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	// Validate request
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	// Validate token
	userID, err := ValidateToken(req.Token, s.cfg.JWT.SecretKey)
	if err != nil {
		return nil, err
	}

	// Check if user exists and is active
	var exists bool
	err = s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM public.users
			WHERE id = $1 AND is_active = true
		)
	`, userID).Scan(&exists)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to validate user")
	}

	if !exists {
		return nil, status.Error(codes.Unauthenticated, "user not found or inactive")
	}

	return &auth.ValidateTokenResponse{
		Valid:   true,
		UserId:  userID,
		Message: "token is valid",
	}, nil
}

func (s *Service) RefreshToken(ctx context.Context, req *auth.RefreshTokenRequest) (*auth.RefreshTokenResponse, error) {
	// Validate request
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	// Validate token
	userID, err := ValidateToken(req.Token, s.cfg.JWT.SecretKey)
	if err != nil {
		return nil, err
	}

	// Check if user exists and is active
	var exists bool
	err = s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM public.users
			WHERE id = $1 AND is_active = true
		)
	`, userID).Scan(&exists)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to validate user")
	}

	if !exists {
		return nil, status.Error(codes.Unauthenticated, "user not found or inactive")
	}

	// Generate new token
	newToken, err := GenerateToken(userID, s.cfg.JWT.SecretKey, s.cfg.JWT.AccessExpiry)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate new token")
	}

	return &auth.RefreshTokenResponse{
		Token:   newToken,
		Message: "token refreshed successfully",
	}, nil
} 