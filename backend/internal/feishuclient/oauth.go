package feishuclient

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// FeishuUserInfo represents user information from Feishu API.
type FeishuUserInfo struct {
	OpenID    string `json:"open_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// TokenResponse represents the response from Feishu token endpoint.
type TokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	Data              *TokenData `json:"data"`
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token"`
	TokenType         string `json:"token_type"`
	ExpiresIn         int    `json:"expires_in"`
	RefreshExpiresIn  int    `json:"refresh_expires_in"`
}

type TokenData struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// UserResponse represents the response from Feishu user info endpoint.
type UserResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data *UserData   `json:"data"`
}

type UserData struct {
	OpenID    string     `json:"open_id"`
	Name      string     `json:"name"`
	AvatarURL AvatarInfo `json:"avatar_url"`
}

type AvatarInfo struct {
	Avatar72     string `json:"avatar_72"`
	Avatar240    string `json:"avatar_240"`
	Avatar640    string `json:"avatar_640"`
	AvatarOrigin string `json:"avatar_origin"`
}

// getFeishuConfig gets Feishu SSO configuration from database or environment variables.
func getFeishuConfig() (appID, appSecret, redirectURI string, enabled bool) {
	// Try to get from database first
	var config model.FeishuSSOConfig
	if err := database.DB.First(&config).Error; err == nil && config.AppID != "" {
		return config.AppID, config.AppSecret, config.RedirectURI, config.Enabled
	}

	// Fallback to environment variables
	return os.Getenv("FEISHU_APP_ID"), os.Getenv("FEISHU_APP_SECRET"), os.Getenv("FEISHU_REDIRECT_URI"), true
}

// IsConfigured checks if Feishu SSO is properly configured.
func IsConfigured() bool {
	appID, appSecret, redirectURI, enabled := getFeishuConfig()
	return enabled && appID != "" && appSecret != "" && redirectURI != ""
}

// GetAppID returns the Feishu app ID (safe to expose).
func GetAppID() string {
	appID, _, _, _ := getFeishuConfig()
	return appID
}

// GetRedirectURI returns the Feishu redirect URI.
func GetRedirectURI() string {
	_, _, redirectURI, _ := getFeishuConfig()
	return redirectURI
}

// ExchangeCodeForToken exchanges authorization code for user_access_token.
func ExchangeCodeForToken(code string) (string, error) {
	appID, appSecret, redirectURI, _ := getFeishuConfig()

	if appID == "" || appSecret == "" {
		return "", fmt.Errorf("飞书 SSO 未配置")
	}

	// Feishu OAuth token endpoint
	tokenURL := "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"

	// Prepare request body
	reqBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     appID,
		"client_secret": appSecret,
		"redirect_uri":  redirectURI,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求飞书 API 失败: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// Check for API error
	if tokenResp.Code != 0 {
		return "", fmt.Errorf("飞书 API 错误: %s", tokenResp.Msg)
	}

	// Get access_token from nested data or top level
	accessToken := ""
	if tokenResp.Data != nil {
		accessToken = tokenResp.Data.AccessToken
	} else if tokenResp.AccessToken != "" {
		accessToken = tokenResp.AccessToken
	}

	if accessToken == "" {
		return "", fmt.Errorf("未获取到 access_token")
	}

	return accessToken, nil
}

// GetUserInfo gets user information from Feishu API using access_token.
func GetUserInfo(accessToken string) (FeishuUserInfo, error) {
	// Feishu user info endpoint
	userInfoURL := "https://open.feishu.cn/open-apis/authen/v1/user_info"

	// Create HTTP request
	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return FeishuUserInfo{}, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return FeishuUserInfo{}, fmt.Errorf("请求飞书 API 失败: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FeishuUserInfo{}, fmt.Errorf("读取响应失败: %w", err)
	}

	// Parse response
	var userResp UserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return FeishuUserInfo{}, fmt.Errorf("解析响应失败: %w", err)
	}

	// Check for API error
	if userResp.Code != 0 {
		return FeishuUserInfo{}, fmt.Errorf("飞书 API 错误: %s", userResp.Msg)
	}

	if userResp.Data == nil {
		return FeishuUserInfo{}, fmt.Errorf("未获取到用户信息")
	}

	// Use avatar_origin as the avatar URL
	avatarURL := userResp.Data.AvatarURL.AvatarOrigin
	if avatarURL == "" {
		avatarURL = userResp.Data.AvatarURL.Avatar640
	}

	return FeishuUserInfo{
		OpenID:    userResp.Data.OpenID,
		Name:      userResp.Data.Name,
		AvatarURL: avatarURL,
	}, nil
}

// BuildAuthURL constructs the Feishu OAuth authorization URL.
func BuildAuthURL(state string) string {
	appID, _, redirectURI, _ := getFeishuConfig()

	params := url.Values{}
	params.Set("app_id", appID)
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	params.Set("response_type", "code")

	return "https://open.feishu.cn/open-apis/authen/v1/authorize?" + params.Encode()
}
