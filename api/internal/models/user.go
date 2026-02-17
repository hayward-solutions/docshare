package models

type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

type User struct {
	BaseModel
	Email               string               `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash        string               `json:"-" gorm:"type:text;not null"`
	FirstName           string               `json:"firstName" gorm:"type:varchar(100);not null"`
	LastName            string               `json:"lastName" gorm:"type:varchar(100);not null"`
	Role                UserRole             `json:"role" gorm:"type:varchar(20);not null;default:'user'"`
	AvatarURL           *string              `json:"avatarURL,omitempty" gorm:"type:text"`
	Theme               *string              `json:"theme,omitempty" gorm:"type:varchar(20);default:'system'"`
	IsEmailVerified     bool                 `json:"isEmailVerified" gorm:"default:false"`
	AuthProvider        *string              `json:"authProvider,omitempty" gorm:"type:varchar(20)"`
	ExternalID          *string              `json:"-" gorm:"type:varchar(255)"`
	GroupMemberships    []GroupMembership    `json:"-" gorm:"foreignKey:UserID"`
	Files               []File               `json:"-" gorm:"foreignKey:OwnerID"`
	Shares              []Share              `json:"-" gorm:"foreignKey:SharedByID"`
	LinkedAccounts      []LinkedAccount      `json:"linkedAccounts,omitempty" gorm:"foreignKey:UserID"`
	MFAConfig           *MFAConfig           `json:"-" gorm:"foreignKey:UserID"`
	WebAuthnCredentials []WebAuthnCredential `json:"-" gorm:"foreignKey:UserID"`
}
