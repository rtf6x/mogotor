package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

const goModelCollectTimeout = 8 * time.Second

func CollectGoModel(baseURL, apiKey string) models.GoModelSnapshot {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	empty := models.GoModelSnapshot{Models: []models.GoModelModel{}}
	if baseURL == "" {
		empty.Error = "gomodel url is empty"
		return empty
	}
	if strings.TrimSpace(apiKey) == "" {
		empty.Error = "gomodel api key is empty"
		return empty
	}

	client := &http.Client{Timeout: goModelCollectTimeout}

	var inventory []goModelInventoryItem
	if err := goModelGetJSON(client, baseURL+"/admin/models", apiKey, &inventory); err != nil {
		empty.Error = err.Error()
		return empty
	}

	var live goModelListResponse
	if err := goModelGetJSON(client, baseURL+"/v1/models", apiKey, &live); err != nil {
		empty.Error = err.Error()
		return empty
	}
	liveIDs := map[string]struct{}{}
	for _, m := range live.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		liveIDs[id] = struct{}{}
	}

	out := make([]models.GoModelModel, 0)
	for _, item := range inventory {
		if !item.Access.EffectiveEnabled {
			continue
		}
		selector := strings.TrimSpace(item.Selector)
		if selector == "" {
			selector = strings.TrimSpace(item.Model.ID)
		}
		if selector == "" {
			continue
		}
		available := modelLive(selector, item.Model.ID, item.ProviderName, liveIDs)
		out = append(out, models.GoModelModel{
			Selector:     selector,
			ProviderName: item.ProviderName,
			ProviderType: item.ProviderType,
			Available:    available,
		})
	}

	return models.GoModelSnapshot{
		Available: true,
		Models:    out,
	}
}

func modelLive(selector, modelID, providerName string, liveIDs map[string]struct{}) bool {
	candidates := []string{selector, modelID}
	if providerName != "" && modelID != "" && !strings.Contains(modelID, "/") {
		candidates = append(candidates, providerName+"/"+modelID)
	}
	for _, id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := liveIDs[id]; ok {
			return true
		}
	}
	return false
}

type goModelInventoryItem struct {
	Selector     string `json:"selector"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	Model        struct {
		ID string `json:"id"`
	} `json:"model"`
	Access struct {
		EffectiveEnabled bool `json:"effective_enabled"`
	} `json:"access"`
}

type goModelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func goModelGetJSON(client *http.Client, url, apiKey string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("gomodel %s: %s", resp.Status, truncate(msg, 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("gomodel decode: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
