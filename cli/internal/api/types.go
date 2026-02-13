package api

import "time"

// File mirrors the backend File model fields relevant to the CLI.
type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MimeType    string    `json:"mimeType"`
	Size        int64     `json:"size"`
	IsDirectory bool      `json:"isDirectory"`
	ParentID    *string   `json:"parentID,omitempty"`
	OwnerID     string    `json:"ownerID"`
	StoragePath string    `json:"storagePath,omitempty"`
	SharedWith  int64     `json:"sharedWith"`
	ParentName  string    `json:"parentName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Owner       *User     `json:"owner,omitempty"`
}

// User mirrors the backend User model.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Role      string    `json:"role,omitempty"`
	AvatarURL *string   `json:"avatarURL,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Share mirrors the backend Share model.
type Share struct {
	ID                string    `json:"id"`
	FileID            string    `json:"fileID"`
	SharedByID        string    `json:"sharedByID"`
	SharedWithUserID  *string   `json:"sharedWithUserID,omitempty"`
	SharedWithGroupID *string   `json:"sharedWithGroupID,omitempty"`
	Permission        string    `json:"permission"`
	ExpiresAt         *string   `json:"expiresAt,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`

	File           *File `json:"file,omitempty"`
	SharedBy       *User `json:"sharedBy,omitempty"`
	SharedWithUser *User `json:"sharedWithUser,omitempty"`
}

// PathSegment represents a breadcrumb element from the /files/:id/path endpoint.
type PathSegment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeviceCodeResponse is returned by POST /auth/device/code.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceTokenResponse is returned by POST /auth/device/token on success.
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// DownloadURLResponse is returned by GET /files/:id/download-url.
type DownloadURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expiresIn"`
}

// LoginResponse is returned by POST /auth/login.
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
