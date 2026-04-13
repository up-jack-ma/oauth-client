package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"oauth-client/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db *sql.DB
}

func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "change-me-in-production"
}

func (h *Handler) baseURL() string {
	if u := os.Getenv("BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:8080"
}

func (h *Handler) generateToken(user *models.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(h.jwtSecret()))
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- Public endpoints ----

func (h *Handler) ListProviders(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, name, display_name, icon FROM oauth_providers WHERE enabled = 1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	providers := []models.PublicProvider{}
	for rows.Next() {
		var p models.PublicProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.Icon); err != nil {
			continue
		}
		providers = append(providers, p)
	}
	c.JSON(http.StatusOK, providers)
}

func (h *Handler) StartOAuth(c *gin.Context) {
	providerName := c.Param("provider")

	var p models.OAuthProvider
	err := h.db.QueryRow(
		"SELECT id, name, client_id, auth_url, scopes, extra_params FROM oauth_providers WHERE name = ? AND enabled = 1",
		providerName,
	).Scan(&p.ID, &p.Name, &p.ClientID, &p.AuthURL, &p.Scopes, &p.ExtraParams)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	state := randomState()
	redirectURL := c.DefaultQuery("redirect", "/")

	_, err = h.db.Exec(
		"INSERT INTO oauth_states (state, provider_id, action, redirect_url) VALUES (?, ?, 'login', ?)",
		state, p.ID, redirectURL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create state"})
		return
	}

	callbackURL := fmt.Sprintf("%s/api/auth/%s/callback", h.baseURL(), p.Name)

	authURL, _ := url.Parse(p.AuthURL)
	q := authURL.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", callbackURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	if p.Scopes != "" {
		q.Set("scope", p.Scopes)
	}

	// Add extra params
	if p.ExtraParams != "" && p.ExtraParams != "{}" {
		var extra map[string]string
		if json.Unmarshal([]byte(p.ExtraParams), &extra) == nil {
			for k, v := range extra {
				q.Set(k, v)
			}
		}
	}

	authURL.RawQuery = q.Encode()
	c.Redirect(http.StatusTemporaryRedirect, authURL.String())
}

func (h *Handler) StartOAuthLink(c *gin.Context) {
	providerName := c.Param("provider")
	userID := c.GetInt64("user_id")

	var p models.OAuthProvider
	err := h.db.QueryRow(
		"SELECT id, name, client_id, auth_url, scopes, extra_params FROM oauth_providers WHERE name = ? AND enabled = 1",
		providerName,
	).Scan(&p.ID, &p.Name, &p.ClientID, &p.AuthURL, &p.Scopes, &p.ExtraParams)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	state := randomState()

	_, err = h.db.Exec(
		"INSERT INTO oauth_states (state, provider_id, user_id, action, redirect_url) VALUES (?, ?, ?, 'link', '/accounts')",
		state, p.ID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create state"})
		return
	}

	callbackURL := fmt.Sprintf("%s/api/auth/%s/callback", h.baseURL(), p.Name)

	authURL, _ := url.Parse(p.AuthURL)
	q := authURL.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", callbackURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	if p.Scopes != "" {
		q.Set("scope", p.Scopes)
	}
	if p.ExtraParams != "" && p.ExtraParams != "{}" {
		var extra map[string]string
		if json.Unmarshal([]byte(p.ExtraParams), &extra) == nil {
			for k, v := range extra {
				q.Set(k, v)
			}
		}
	}

	authURL.RawQuery = q.Encode()
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL.String()})
}

func (h *Handler) OAuthCallback(c *gin.Context) {
	code := c.Query("code")
	stateStr := c.Query("state")

	if code == "" || stateStr == "" {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=missing_code")
		return
	}

	// Lookup state
	var st models.OAuthState
	var userIDNull sql.NullInt64
	err := h.db.QueryRow(
		"SELECT state, provider_id, user_id, action, redirect_url FROM oauth_states WHERE state = ?",
		stateStr,
	).Scan(&st.State, &st.ProviderID, &userIDNull, &st.Action, &st.RedirectURL)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=invalid_state")
		return
	}
	if userIDNull.Valid {
		uid := userIDNull.Int64
		st.UserID = &uid
	}

	// Delete used state
	h.db.Exec("DELETE FROM oauth_states WHERE state = ?", stateStr)
	// Cleanup old states
	h.db.Exec("DELETE FROM oauth_states WHERE created_at < datetime('now', '-1 hour')")

	// Get provider details
	var p models.OAuthProvider
	err = h.db.QueryRow(
		"SELECT id, name, client_id, client_secret, token_url, userinfo_url FROM oauth_providers WHERE id = ?",
		st.ProviderID,
	).Scan(&p.ID, &p.Name, &p.ClientID, &p.ClientSecret, &p.TokenURL, &p.UserinfoURL)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=provider_not_found")
		return
	}

	// Exchange code for token
	callbackURL := fmt.Sprintf("%s/api/auth/%s/callback", h.baseURL(), p.Name)
	tokenResp, err := exchangeCode(p, code, callbackURL)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=token_exchange_failed")
		return
	}

	// Fetch userinfo
	userinfo, err := fetchUserinfo(p.UserinfoURL, tokenResp.AccessToken)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=userinfo_failed")
		return
	}

	// Extract common fields
	providerUserID := extractField(userinfo, "id", "sub", "user_id")
	providerEmail := extractField(userinfo, "email")
	providerName := extractField(userinfo, "name", "login", "display_name", "nickname")
	providerAvatar := extractField(userinfo, "avatar_url", "picture", "avatar")

	rawJSON, _ := json.Marshal(userinfo)

	// Compute token expiry times
	var tokenExpiry, refreshTokenExpiry *time.Time
	if tokenResp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		tokenExpiry = &t
	}
	if tokenResp.RefreshExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokenResp.RefreshExpiresIn) * time.Second)
		refreshTokenExpiry = &t
	}
	scopesGranted := tokenResp.Scope
	if scopesGranted == "" {
		scopesGranted = p.Scopes // fallback to requested scopes
	}

	if st.Action == "link" && st.UserID != nil {
		// Link account to existing user
		_, err = h.db.Exec(`
			INSERT INTO linked_accounts (user_id, provider_id, provider_user_id, provider_email, provider_name, provider_avatar,
				access_token, refresh_token, token_expiry, refresh_token_expiry, scopes_granted, raw_token_response, raw_userinfo, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(provider_id, provider_user_id) DO UPDATE SET
				provider_email=excluded.provider_email, provider_name=excluded.provider_name,
				provider_avatar=excluded.provider_avatar, access_token=excluded.access_token,
				refresh_token=excluded.refresh_token, token_expiry=excluded.token_expiry,
				refresh_token_expiry=excluded.refresh_token_expiry, scopes_granted=excluded.scopes_granted,
				raw_token_response=excluded.raw_token_response, raw_userinfo=excluded.raw_userinfo, updated_at=excluded.updated_at`,
			*st.UserID, p.ID, providerUserID, providerEmail, providerName, providerAvatar,
			tokenResp.AccessToken, tokenResp.RefreshToken, tokenExpiry, refreshTokenExpiry,
			scopesGranted, tokenResp.RawJSON, string(rawJSON), time.Now(),
		)
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/accounts?error=link_failed")
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, "/accounts?linked=true")
		return
	}

	// Login or register flow
	// Check if this provider account is already linked
	var existingUserID int64
	err = h.db.QueryRow(
		"SELECT user_id FROM linked_accounts WHERE provider_id = ? AND provider_user_id = ?",
		p.ID, providerUserID,
	).Scan(&existingUserID)

	var user models.User

	if err == nil {
		// Existing linked account — load user
		h.db.QueryRow("SELECT id, email, display_name, avatar_url, role FROM users WHERE id = ?", existingUserID).
			Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.Role)

		// Update token
		h.db.Exec(`UPDATE linked_accounts SET access_token=?, refresh_token=?, token_expiry=?, refresh_token_expiry=?,
			scopes_granted=?, raw_token_response=?, provider_email=?, provider_name=?, provider_avatar=?, raw_userinfo=?, updated_at=?
			WHERE provider_id=? AND provider_user_id=?`,
			tokenResp.AccessToken, tokenResp.RefreshToken, tokenExpiry, refreshTokenExpiry,
			scopesGranted, tokenResp.RawJSON, providerEmail, providerName, providerAvatar, string(rawJSON), time.Now(),
			p.ID, providerUserID,
		)
	} else {
		// Try to find user by email, or create new user
		if providerEmail != "" {
			err = h.db.QueryRow("SELECT id, email, display_name, avatar_url, role FROM users WHERE email = ?", providerEmail).
				Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.Role)
		}

		if user.ID == 0 {
			// Create new user
			displayName := providerName
			if displayName == "" {
				displayName = providerEmail
			}
			result, err := h.db.Exec(
				"INSERT INTO users (email, display_name, avatar_url, role, created_at) VALUES (?, ?, ?, 'user', ?)",
				providerEmail, displayName, providerAvatar, time.Now(),
			)
			if err != nil {
				c.Redirect(http.StatusTemporaryRedirect, "/?error=create_user_failed")
				return
			}
			user.ID, _ = result.LastInsertId()
			user.Email = providerEmail
			user.DisplayName = displayName
			user.AvatarURL = providerAvatar
			user.Role = "user"
		}

		// Create linked account
		h.db.Exec(`INSERT OR REPLACE INTO linked_accounts (user_id, provider_id, provider_user_id, provider_email, provider_name, provider_avatar,
			access_token, refresh_token, token_expiry, refresh_token_expiry, scopes_granted, raw_token_response, raw_userinfo, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, p.ID, providerUserID, providerEmail, providerName, providerAvatar,
			tokenResp.AccessToken, tokenResp.RefreshToken, tokenExpiry, refreshTokenExpiry,
			scopesGranted, tokenResp.RawJSON, string(rawJSON), time.Now(),
		)
	}

	// Generate JWT
	tokenString, err := h.generateToken(&user)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?error=token_generation_failed")
		return
	}

	// Set cookie and redirect
	c.SetCookie("token", tokenString, 3*24*3600, "/", "", false, false)
	redirect := st.RedirectURL
	if redirect == "" {
		redirect = "/accounts"
	}
	c.Redirect(http.StatusTemporaryRedirect, redirect)
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Scope            string `json:"scope"`
	RawJSON          string `json:"-"`
}

func exchangeCode(p models.OAuthProvider, code, redirectURI string) (*tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)

	resp, err := http.PostForm(p.TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tr tokenResponse
	// Try JSON first
	if err := json.Unmarshal(body, &tr); err != nil {
		// Try form-encoded (some providers like GitHub return this)
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, fmt.Errorf("failed to parse token response: %s", string(body))
		}
		tr.AccessToken = vals.Get("access_token")
		tr.TokenType = vals.Get("token_type")
		tr.Scope = vals.Get("scope")
	}

	if tr.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response: %s", string(body))
	}

	tr.RawJSON = string(body)
	return &tr, nil
}

func refreshAccessToken(p models.OAuthProvider, refreshToken string) (*tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)

	resp, err := http.PostForm(p.TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, fmt.Errorf("failed to parse refresh response: %s", string(body))
		}
		tr.AccessToken = vals.Get("access_token")
		tr.RefreshToken = vals.Get("refresh_token")
		tr.TokenType = vals.Get("token_type")
		tr.Scope = vals.Get("scope")
	}

	if tr.AccessToken == "" {
		return nil, fmt.Errorf("refresh failed: %s", string(body))
	}

	tr.RawJSON = string(body)
	return &tr, nil
}

func fetchUserinfo(userinfoURL, accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func extractField(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			switch val := v.(type) {
			case string:
				return val
			case float64:
				return strconv.FormatFloat(val, 'f', -1, 64)
			case json.Number:
				return val.String()
			}
		}
	}
	return ""
}

// ---- Auth endpoints ----

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}

	var user models.User
	err := h.db.QueryRow(
		"SELECT id, email, password_hash, display_name, avatar_url, role FROM users WHERE email = ?",
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AvatarURL, &user.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.SetCookie("token", token, 3*24*3600, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		Name     string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email and password (min 6 chars) required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	name := req.Name
	if name == "" {
		name = req.Email
	}

	result, err := h.db.Exec(
		"INSERT INTO users (email, password_hash, display_name, role, created_at) VALUES (?, ?, ?, 'user', ?)",
		req.Email, string(hash), name, time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	id, _ := result.LastInsertId()
	user := models.User{ID: id, Email: req.Email, DisplayName: name, Role: "user"}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.SetCookie("token", token, 3*24*3600, "/", "", false, false)
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
}

func (h *Handler) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// ---- User endpoints ----

func (h *Handler) GetMe(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var user models.User
	err := h.db.QueryRow(
		"SELECT id, email, display_name, avatar_url, role, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.Role, &user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ListLinkedAccounts(c *gin.Context) {
	userID := c.GetInt64("user_id")
	rows, err := h.db.Query(`
		SELECT la.id, la.provider_id, la.provider_user_id, la.provider_email, la.provider_name,
			la.provider_avatar, la.access_token, la.refresh_token, la.token_expiry, la.refresh_token_expiry,
			la.scopes_granted, la.raw_token_response, la.created_at, la.updated_at,
			op.display_name, op.icon
		FROM linked_accounts la
		JOIN oauth_providers op ON la.provider_id = op.id
		WHERE la.user_id = ?
		ORDER BY la.created_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	accounts := []models.LinkedAccount{}
	for rows.Next() {
		var a models.LinkedAccount
		var tokenExpiry, refreshExpiry sql.NullTime
		if err := rows.Scan(&a.ID, &a.ProviderID, &a.ProviderUserID, &a.ProviderEmail, &a.ProviderName,
			&a.ProviderAvatar, &a.AccessToken, &a.RefreshToken, &tokenExpiry, &refreshExpiry,
			&a.ScopesGranted, &a.RawTokenResponse, &a.CreatedAt, &a.UpdatedAt,
			&a.ProviderDisplayName, &a.ProviderIcon); err != nil {
			continue
		}
		if tokenExpiry.Valid {
			a.TokenExpiry = &tokenExpiry.Time
		}
		if refreshExpiry.Valid {
			a.RefreshTokenExpiry = &refreshExpiry.Time
		}
		// Mask tokens for display — show first 8 and last 4 chars
		a.AccessToken = maskToken(a.AccessToken)
		a.RefreshToken = maskToken(a.RefreshToken)
		accounts = append(accounts, a)
	}
	c.JSON(http.StatusOK, accounts)
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 16 {
		return token[:4] + "****"
	}
	return token[:8] + "..." + token[len(token)-4:]
}

// RefreshAccountToken refreshes the OAuth token for a linked account
func (h *Handler) RefreshAccountToken(c *gin.Context) {
	userID := c.GetInt64("user_id")
	accountID := c.Param("id")

	// Get linked account with refresh token
	var la struct {
		ID           int64
		ProviderID   int64
		RefreshToken string
	}
	err := h.db.QueryRow(
		"SELECT id, provider_id, refresh_token FROM linked_accounts WHERE id = ? AND user_id = ?",
		accountID, userID,
	).Scan(&la.ID, &la.ProviderID, &la.RefreshToken)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	if la.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no refresh token available for this account"})
		return
	}

	// Get provider
	var p models.OAuthProvider
	err = h.db.QueryRow(
		"SELECT id, name, client_id, client_secret, token_url, scopes FROM oauth_providers WHERE id = ?",
		la.ProviderID,
	).Scan(&p.ID, &p.Name, &p.ClientID, &p.ClientSecret, &p.TokenURL, &p.Scopes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "provider not found"})
		return
	}

	// Call provider to refresh
	tokenResp, err := refreshAccessToken(p, la.RefreshToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("refresh failed: %v", err)})
		return
	}

	// Compute new expiries
	var tokenExpiry, refreshTokenExpiry *time.Time
	if tokenResp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		tokenExpiry = &t
	}
	if tokenResp.RefreshExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokenResp.RefreshExpiresIn) * time.Second)
		refreshTokenExpiry = &t
	}

	// Some providers rotate refresh tokens
	newRefresh := tokenResp.RefreshToken
	if newRefresh == "" {
		newRefresh = la.RefreshToken // keep old one if not rotated
	}

	scopesGranted := tokenResp.Scope
	if scopesGranted == "" {
		scopesGranted = p.Scopes
	}

	_, err = h.db.Exec(`
		UPDATE linked_accounts SET access_token=?, refresh_token=?, token_expiry=?, refresh_token_expiry=?,
			scopes_granted=?, raw_token_response=?, updated_at=?
		WHERE id=? AND user_id=?`,
		tokenResp.AccessToken, newRefresh, tokenExpiry, refreshTokenExpiry,
		scopesGranted, tokenResp.RawJSON, time.Now(),
		accountID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "token refreshed successfully",
		"access_token":         maskToken(tokenResp.AccessToken),
		"refresh_token":        maskToken(newRefresh),
		"token_expiry":         tokenExpiry,
		"refresh_token_expiry": refreshTokenExpiry,
		"scopes_granted":       scopesGranted,
		"raw_token_response":   tokenResp.RawJSON,
	})
}

func (h *Handler) UnlinkAccount(c *gin.Context) {
	userID := c.GetInt64("user_id")
	accountID := c.Param("id")

	result, err := h.db.Exec("DELETE FROM linked_accounts WHERE id = ? AND user_id = ?", accountID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "account unlinked"})
}

// ---- Admin endpoints ----

func (h *Handler) AdminListProviders(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, name, display_name, client_id, client_secret, auth_url, token_url, userinfo_url, scopes, icon, enabled, extra_params, created_at FROM oauth_providers ORDER BY id")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	providers := []models.OAuthProvider{}
	for rows.Next() {
		var p models.OAuthProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.ClientID, &p.ClientSecret, &p.AuthURL, &p.TokenURL,
			&p.UserinfoURL, &p.Scopes, &p.Icon, &p.Enabled, &p.ExtraParams, &p.CreatedAt); err != nil {
			continue
		}
		// Mask secret for response
		if len(p.ClientSecret) > 8 {
			p.ClientSecret = p.ClientSecret[:4] + "****" + p.ClientSecret[len(p.ClientSecret)-4:]
		}
		providers = append(providers, p)
	}
	c.JSON(http.StatusOK, providers)
}

func (h *Handler) AdminCreateProvider(c *gin.Context) {
	var p models.OAuthProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if p.ExtraParams == "" {
		p.ExtraParams = "{}"
	}

	result, err := h.db.Exec(`
		INSERT INTO oauth_providers (name, display_name, client_id, client_secret, auth_url, token_url, userinfo_url, scopes, icon, enabled, extra_params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.DisplayName, p.ClientID, p.ClientSecret, p.AuthURL, p.TokenURL, p.UserinfoURL,
		p.Scopes, p.Icon, p.Enabled, p.ExtraParams, time.Now(), time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "provider name already exists"})
		return
	}

	p.ID, _ = result.LastInsertId()
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) AdminUpdateProvider(c *gin.Context) {
	id := c.Param("id")
	var p models.OAuthProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If secret contains ****, keep the old one
	if strings.Contains(p.ClientSecret, "****") {
		var oldSecret string
		h.db.QueryRow("SELECT client_secret FROM oauth_providers WHERE id = ?", id).Scan(&oldSecret)
		p.ClientSecret = oldSecret
	}

	if p.ExtraParams == "" {
		p.ExtraParams = "{}"
	}

	_, err := h.db.Exec(`
		UPDATE oauth_providers SET name=?, display_name=?, client_id=?, client_secret=?, auth_url=?, token_url=?,
		userinfo_url=?, scopes=?, icon=?, enabled=?, extra_params=?, updated_at=? WHERE id=?`,
		p.Name, p.DisplayName, p.ClientID, p.ClientSecret, p.AuthURL, p.TokenURL, p.UserinfoURL,
		p.Scopes, p.Icon, p.Enabled, p.ExtraParams, time.Now(), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) AdminDeleteProvider(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.Exec("DELETE FROM oauth_providers WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) AdminListUsers(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT u.id, u.email, u.display_name, u.avatar_url, u.role, u.created_at,
			COUNT(la.id) as linked_count
		FROM users u
		LEFT JOIN linked_accounts la ON u.id = la.user_id
		GROUP BY u.id
		ORDER BY u.created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type UserWithCount struct {
		models.User
		LinkedCount int `json:"linked_count"`
	}

	users := []UserWithCount{}
	for rows.Next() {
		var u UserWithCount
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.LinkedCount); err != nil {
			continue
		}
		users = append(users, u)
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) AdminUpdateUserRole(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is required"})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be 'admin' or 'user'"})
		return
	}

	_, err := h.db.Exec("UPDATE users SET role = ?, updated_at = ? WHERE id = ?", req.Role, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

func (h *Handler) AdminGetStats(c *gin.Context) {
	var stats models.Stats
	h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	h.db.QueryRow("SELECT COUNT(*) FROM oauth_providers WHERE enabled = 1").Scan(&stats.TotalProviders)
	h.db.QueryRow("SELECT COUNT(*) FROM linked_accounts").Scan(&stats.TotalAccounts)
	c.JSON(http.StatusOK, stats)
}
