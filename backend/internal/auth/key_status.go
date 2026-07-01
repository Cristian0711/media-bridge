package auth

func inviteKeyStatus(isActive bool) string {
	if isActive {
		return "available"
	}
	return "used"
}

func toInviteKeyResponse(key Key) InviteKeyResponse {
	return InviteKeyResponse{
		Value:     key.Value,
		IsActive:  key.IsActive,
		Status:    inviteKeyStatus(key.IsActive),
		UsedAt:    key.UsedAt,
		CreatedAt: key.CreatedAt,
	}
}
