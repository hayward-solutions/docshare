package models

import "github.com/google/uuid"

type GroupMembershipRole string

const (
	GroupRoleOwner  GroupMembershipRole = "owner"
	GroupRoleAdmin  GroupMembershipRole = "admin"
	GroupRoleMember GroupMembershipRole = "member"
)

type GroupMembership struct {
	BaseModel
	UserID  uuid.UUID           `json:"userID" gorm:"type:uuid;not null;index;uniqueIndex:idx_user_group"`
	GroupID uuid.UUID           `json:"groupID" gorm:"type:uuid;not null;index;uniqueIndex:idx_user_group"`
	Role    GroupMembershipRole `json:"role" gorm:"type:varchar(20);not null;default:'member'"`
	User    User                `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Group   Group               `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}
