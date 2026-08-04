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

const rabbitCollectTimeout = 5 * time.Second

func CollectRabbit(baseURL, user, password string) models.RabbitSnapshot {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return models.RabbitSnapshot{Error: "rabbit url is empty", Listeners: []models.RabbitListener{}, Queues: []models.RabbitQueueSnapshot{}}
	}

	client := &http.Client{Timeout: rabbitCollectTimeout}

	var overview rabbitOverviewAPI
	if err := rabbitGetJSON(client, baseURL+"/api/overview", user, password, &overview); err != nil {
		return models.RabbitSnapshot{
			Error:     err.Error(),
			Listeners: []models.RabbitListener{},
			Queues:    []models.RabbitQueueSnapshot{},
		}
	}

	snap := models.RabbitSnapshot{
		Available:       true,
		Version:         overview.RabbitmqVersion,
		ErlangVersion:   overview.ErlangVersion,
		ClusterName:     overview.ClusterName,
		Node:            overview.Node,
		Connections:     overview.ObjectTotals.Connections,
		Channels:        overview.ObjectTotals.Channels,
		Consumers:       overview.ObjectTotals.Consumers,
		QueueCount:      overview.ObjectTotals.Queues,
		Exchanges:       overview.ObjectTotals.Exchanges,
		MessagesReady:   overview.QueueTotals.MessagesReady,
		MessagesUnacked: overview.QueueTotals.MessagesUnacked,
		MessagesTotal:   overview.QueueTotals.Messages,
		Listeners:       make([]models.RabbitListener, 0, len(overview.Listeners)),
		Queues:          []models.RabbitQueueSnapshot{},
	}
	for _, l := range overview.Listeners {
		snap.Listeners = append(snap.Listeners, models.RabbitListener{
			Node:      l.Node,
			Protocol:  l.Protocol,
			IPAddress: l.IPAddress,
			Port:      l.Port,
		})
	}

	var nodes []rabbitNodeAPI
	if err := rabbitGetJSON(client, baseURL+"/api/nodes", user, password, &nodes); err == nil {
		for _, n := range nodes {
			if overview.Node != "" && n.Name != overview.Node {
				continue
			}
			snap.NodeInfo = &models.RabbitNodeInfo{
				Name:         n.Name,
				Running:      n.Running,
				MemUsedBytes: n.MemUsed,
				UptimeMs:     n.Uptime,
				Type:         n.Type,
			}
			break
		}
		if snap.NodeInfo == nil && len(nodes) > 0 {
			n := nodes[0]
			snap.NodeInfo = &models.RabbitNodeInfo{
				Name:         n.Name,
				Running:      n.Running,
				MemUsedBytes: n.MemUsed,
				UptimeMs:     n.Uptime,
				Type:         n.Type,
			}
		}
	}

	var queues []rabbitQueueAPI
	if err := rabbitGetJSON(client, baseURL+"/api/queues", user, password, &queues); err != nil {
		if snap.Error == "" {
			snap.Error = "queues: " + err.Error()
		}
		return snap
	}
	snap.Queues = make([]models.RabbitQueueSnapshot, 0, len(queues))
	for _, q := range queues {
		snap.Queues = append(snap.Queues, models.RabbitQueueSnapshot{
			Name:            q.Name,
			Vhost:           q.Vhost,
			State:           q.State,
			Messages:        q.Messages,
			MessagesReady:   q.MessagesReady,
			MessagesUnacked: q.MessagesUnacked,
			Consumers:       q.Consumers,
			PublishRate:     q.MessageStats.PublishDetails.Rate,
			DeliverRate:     q.MessageStats.DeliverGetDetails.Rate,
		})
	}
	return snap
}

type rabbitOverviewAPI struct {
	RabbitmqVersion string `json:"rabbitmq_version"`
	ErlangVersion   string `json:"erlang_version"`
	ClusterName     string `json:"cluster_name"`
	Node            string `json:"node"`
	ObjectTotals    struct {
		Connections int `json:"connections"`
		Channels    int `json:"channels"`
		Consumers   int `json:"consumers"`
		Queues      int `json:"queues"`
		Exchanges   int `json:"exchanges"`
	} `json:"object_totals"`
	QueueTotals struct {
		Messages        int `json:"messages"`
		MessagesReady   int `json:"messages_ready"`
		MessagesUnacked int `json:"messages_unacknowledged"`
	} `json:"queue_totals"`
	Listeners []struct {
		Node      string `json:"node"`
		Protocol  string `json:"protocol"`
		IPAddress string `json:"ip_address"`
		Port      int    `json:"port"`
	} `json:"listeners"`
}

type rabbitNodeAPI struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	MemUsed uint64 `json:"mem_used"`
	Uptime  int64  `json:"uptime"`
	Type    string `json:"type"`
}

type rabbitQueueAPI struct {
	Name            string `json:"name"`
	Vhost           string `json:"vhost"`
	State           string `json:"state"`
	Messages        int    `json:"messages"`
	MessagesReady   int    `json:"messages_ready"`
	MessagesUnacked int    `json:"messages_unacknowledged"`
	Consumers       int    `json:"consumers"`
	MessageStats    struct {
		PublishDetails struct {
			Rate float64 `json:"rate"`
		} `json:"publish_details"`
		DeliverGetDetails struct {
			Rate float64 `json:"rate"`
		} `json:"deliver_get_details"`
	} `json:"message_stats"`
}

func rabbitGetJSON(client *http.Client, url, user, password string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateRabbitErr(msg))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}
	return nil
}

func truncateRabbitErr(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "..."
}
