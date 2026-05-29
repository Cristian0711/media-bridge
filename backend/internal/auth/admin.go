package auth

import (
	"context"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
)

func (s *service) requireAdmin(ctx context.Context, userID uint) error {
	user, err := s.userSvc.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !users.IsAdmin(user.Role) {
		return ErrForbidden
	}
	return nil
}

func inviteKeyStatus(isActive bool) string {
	if isActive {
		return "available"
	}
	return "used"
}
