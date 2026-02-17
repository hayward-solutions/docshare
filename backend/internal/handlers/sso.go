package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SSOHandler struct {
	DB           *gorm.DB
	Cfg          *config.Config
	SSOService   *services.SSOService
	OAuthService *services.OAuthProviderService
	SAMLService  *services.SAMLService
	LDAPService  *services.LDAPService
}

func NewSSOHandler(db *gorm.DB, cfg *config.Config) *SSOHandler {
	return &SSOHandler{
		DB:           db,
		Cfg:          cfg,
		SSOService:   services.NewSSOService(db, cfg),
		OAuthService: services.NewOAuthProviderService(cfg),
		SAMLService:  services.NewSAMLService(cfg),
		LDAPService:  services.NewLDAPService(cfg),
	}
}

type LoginRedirectRequest struct {
	Provider string `json:"provider"`
	Redirect string `json:"redirect"`
}

func (h *SSOHandler) GetLoginRedirect(c *fiber.Ctx) error {
	provider := c.Params("provider")

	authCodeURL, err := h.getAuthorizationURL(c.Context(), provider)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"url": authCodeURL,
	})
}

func (h *SSOHandler) getAuthorizationURL(ctx context.Context, provider string) (string, error) {
	oauthCfg, providerName, err := h.OAuthService.GetOAuthConfig(provider)
	if err != nil {
		return "", err
	}

	state, err := h.OAuthService.GenerateState(providerName)
	if err != nil {
		return "", err
	}

	stateJSON, _ := json.Marshal(state)
	stateEncoded := base64.URLEncoding.EncodeToString(stateJSON)

	return oauthCfg.AuthCodeURL(stateEncoded), nil
}

type OAuthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func (h *SSOHandler) HandleOAuthCallback(c *fiber.Ctx) error {
	provider := c.Params("provider")
	code := c.Query("code")
	state := c.Query("state")

	frontendURL := h.Cfg.Server.FrontendURL

	if code == "" {
		return c.Redirect(frontendURL + "/login?error=" + url.QueryEscape("authorization code is required"))
	}

	profile, err := h.processOAuthCallback(c.Context(), provider, code, state)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=" + url.QueryEscape(err.Error()))
	}

	user, err := h.SSOService.FindOrCreateUser(c.Context(), profile)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=" + url.QueryEscape(err.Error()))
	}

	hasMFA, methods := UserHasMFA(h.DB, user.ID)
	if hasMFA {
		mfaToken, err := utils.GenerateMFAToken(user.ID, user.Email)
		if err != nil {
			return c.Redirect(frontendURL + "/login?error=" + url.QueryEscape("failed to generate MFA token"))
		}
		methodsJSON, _ := json.Marshal(methods)
		return c.Redirect(frontendURL + "/auth/callback?mfa_required=true&mfa_token=" + url.QueryEscape(mfaToken) + "&methods=" + url.QueryEscape(string(methodsJSON)))
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=" + url.QueryEscape("failed to generate token"))
	}

	logger.Info("sso_login_success", map[string]interface{}{
		"user_id":  user.ID.String(),
		"email":    user.Email,
		"provider": provider,
	})

	return c.Redirect(frontendURL + "/auth/callback?token=" + token)
}

func (h *SSOHandler) processOAuthCallback(ctx context.Context, provider, code, state string) (*services.SSOProfile, error) {
	token, err := h.OAuthService.ExchangeCode(ctx, provider, code)
	if err != nil {
		return nil, err
	}

	profile, err := h.OAuthService.GetUserInfo(ctx, provider, token)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (h *SSOHandler) HandleSAMLMetadata(c *fiber.Ctx) error {
	metadata := h.SAMLService.GetMetadata()
	if metadata == nil {
		return utils.Error(c, fiber.StatusNotFound, "SAML not configured")
	}

	c.Set("Content-Type", "application/xml")
	return c.Send(metadata)
}

func (h *SSOHandler) HandleSAMLACS(c *fiber.Ctx) error {
	samlResponse := c.FormValue("SAMLResponse")
	if samlResponse == "" {
		return utils.Error(c, fiber.StatusBadRequest, "SAMLResponse is required")
	}

	decoded, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid SAML response")
	}

	profile, err := h.SAMLService.HandleACS(c.Context(), string(decoded))
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, err.Error())
	}

	user, err := h.SSOService.FindOrCreateUser(c.Context(), profile)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	frontendURL := h.Cfg.Server.FrontendURL

	hasMFA, methods := UserHasMFA(h.DB, user.ID)
	if hasMFA {
		mfaToken, err := utils.GenerateMFAToken(user.ID, user.Email)
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed to generate MFA token")
		}
		methodsJSON, _ := json.Marshal(methods)
		return c.Redirect(frontendURL + "/auth/callback?mfa_required=true&mfa_token=" + url.QueryEscape(mfaToken) + "&methods=" + url.QueryEscape(string(methodsJSON)))
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate token")
	}

	logger.Info("saml_login_success", map[string]interface{}{
		"user_id": user.ID.String(),
		"email":   user.Email,
	})

	return c.Redirect(frontendURL + "/auth/callback?token=" + token)
}

type LDAPLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *SSOHandler) HandleLDAPLogin(c *fiber.Ctx) error {
	var req LDAPLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Username == "" || req.Password == "" {
		return utils.Error(c, fiber.StatusBadRequest, "username and password are required")
	}

	profile, err := h.LDAPService.Authenticate(c.Context(), req.Username, req.Password)
	if err != nil {
		logger.Warn("ldap_login_failed", map[string]interface{}{
			"username": req.Username,
			"error":    err.Error(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	user, err := h.SSOService.FindOrCreateUser(c.Context(), profile)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	hasMFA, methods := UserHasMFA(h.DB, user.ID)
	if hasMFA {
		mfaToken, err := utils.GenerateMFAToken(user.ID, user.Email)
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed to generate MFA token")
		}
		return utils.Success(c, fiber.StatusOK, fiber.Map{
			"mfaRequired": true,
			"mfaToken":    mfaToken,
			"methods":     methods,
		})
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate token")
	}

	logger.Info("ldap_login_success", map[string]interface{}{
		"user_id":  user.ID.String(),
		"email":    user.Email,
		"provider": "ldap",
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"token": token,
		"user":  user,
	})
}

func (h *SSOHandler) ListProviders(c *fiber.Ctx) error {
	providers := []fiber.Map{}

	if h.Cfg.SSO.Google.Enabled {
		providers = append(providers, fiber.Map{
			"name":        "google",
			"displayName": "Google",
			"type":        "oauth",
		})
	}

	if h.Cfg.SSO.GitHub.Enabled {
		providers = append(providers, fiber.Map{
			"name":        "github",
			"displayName": "GitHub",
			"type":        "oauth",
		})
	}

	if h.Cfg.SSO.OIDC.Enabled {
		providers = append(providers, fiber.Map{
			"name":        "oidc",
			"displayName": "OpenID Connect",
			"type":        "oidc",
		})
	}

	if h.SAMLService.IsEnabled() {
		providers = append(providers, fiber.Map{
			"name":        "saml",
			"displayName": "Enterprise SSO (SAML)",
			"type":        "saml",
		})
	}

	if h.LDAPService.IsEnabled() {
		providers = append(providers, fiber.Map{
			"name":        "ldap",
			"displayName": "Corporate Directory (LDAP)",
			"type":        "ldap",
		})
	}

	return utils.Success(c, fiber.StatusOK, providers)
}

func (h *SSOHandler) GetLinkedAccounts(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	accounts, err := h.SSOService.GetLinkedAccounts(c.Context(), user.ID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to get linked accounts")
	}

	return utils.Success(c, fiber.StatusOK, accounts)
}

func (h *SSOHandler) UnlinkAccount(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	accountID := c.Params("id")
	if accountID == "" {
		return utils.Error(c, fiber.StatusBadRequest, "account ID is required")
	}

	err := h.SSOService.UnlinkAccount(c.Context(), user.ID, mustParseUUID(accountID))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to unlink account")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "account unlinked"})
}

func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

type LinkAccountRequest struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	State    string `json:"state"`
}

func (h *SSOHandler) LinkAccount(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req LinkAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	var profile *services.SSOProfile
	var err error

	switch strings.ToLower(req.Provider) {
	case "google", "github", "oidc":
		profile, err = h.processOAuthCallback(c.Context(), req.Provider, req.Code, req.State)
	case "saml":
		return utils.Error(c, fiber.StatusBadRequest, "SAML linking not supported yet")
	case "ldap":
		return utils.Error(c, fiber.StatusBadRequest, "LDAP linking not supported yet")
	default:
		return utils.Error(c, fiber.StatusBadRequest, "unknown provider")
	}

	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}

	err = h.SSOService.LinkAccount(c.Context(), user.ID, profile)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to link account")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "account linked"})
}
