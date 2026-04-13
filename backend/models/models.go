package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	AvatarURL    string    `json:"avatar_url"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OAuthProvider struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret,omitempty"`
	AuthURL      string    `json:"auth_url"`
	TokenURL     string    `json:"token_url"`
	UserinfoURL  string    `json:"userinfo_url"`
	Scopes       string    `json:"scopes"`
	Icon         string    `json:"icon"`
	Enabled      bool      `json:"enabled"`
	ExtraParams  string    `json:"extra_params"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PublicProvider struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Icon        string `json:"icon"`
}

type LinkedAccount struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	ProviderID         int64      `json:"provider_id"`
	ProviderUserID     string     `json:"provider_user_id"`
	ProviderEmail      string     `json:"provider_email"`
	ProviderName       string     `json:"provider_name"`
	ProviderAvatar     string     `json:"provider_avatar"`
	AccessToken        string     `json:"access_token"`
	RefreshToken       string     `json:"refresh_token"`
	TokenExpiry        *time.Time `json:"token_expiry"`
	RefreshTokenExpiry *time.Time `json:"refresh_token_expiry"`
	ScopesGranted      string     `json:"scopes_granted"`
	RawTokenResponse   string     `json:"raw_token_response"`
	RawUserinfo        string     `json:"raw_userinfo"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Joined fields
	ProviderDisplayName string `json:"provider_display_name,omitempty"`
	ProviderIcon        string `json:"provider_icon,omitempty"`
	ProviderTokenURL    string `json:"-"`
}

type OAuthState struct {
	State       string `json:"state"`
	ProviderID  int64  `json:"provider_id"`
	UserID      *int64 `json:"user_id,omitempty"`
	Action      string `json:"action"`
	RedirectURL string `json:"redirect_url"`
}

type Stats struct {
	TotalUsers     int `json:"total_users"`
	TotalProviders int `json:"total_providers"`
	TotalAccounts  int `json:"total_linked_accounts"`
}
