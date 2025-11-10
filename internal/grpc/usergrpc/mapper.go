package usergrpc

import (
	"lead_exchange/internal/domain"
	pb "lead_exchange/pkg"
)

// userDomainToProto — преобразует доменную сущность пользователя в protobuf-модель.
func userDomainToProto(u domain.User) *pb.UserProfile {
	return &pb.UserProfile{
		Id:         u.ID.String(),
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Phone:      u.Phone,
		AgencyName: u.AgencyName,
		AvatarUrl:  u.AvatarURL,
		Role:       userTypeDomainToProto(u.Role),
	}
}

func userTypeDomainToProto(userType domain.UserRole) pb.UserRole {
	switch userType {
	case domain.UserRoleUser:
		return pb.UserRole_USER_ROLE_USER
	case domain.UserRoleAdmin:
		return pb.UserRole_USER_ROLE_ADMIN
	default:
		return pb.UserRole_USER_ROLE_UNSPECIFIED
	}
}
