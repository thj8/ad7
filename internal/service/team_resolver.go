package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TeamResolver 用于解析用户所在队伍，通过 HTTP 调用 auth 服务。
type TeamResolver struct {
	authURL string
	client  *http.Client
}

// NewTeamResolver 创建新的 TeamResolver。
func NewTeamResolver(authURL string) *TeamResolver {
	return &TeamResolver{
		authURL: authURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetUserTeam 获取用户当前所在的队伍 ID。如果用户没有加入任何队伍，返回空字符串。
func (r *TeamResolver) GetUserTeam(ctx context.Context, userID string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s/teams", r.authURL, userID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call auth service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read response body: %w", err)
		}

		var result struct {
			Teams []struct {
				ID string `json:"id"`
			} `json:"teams"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return "", nil
		}
		if len(result.Teams) > 0 {
			return result.Teams[0].ID, nil
		}
	}

	return "", nil
}
