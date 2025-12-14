package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"lead_exchange/internal/config"
	"log/slog"
)

// Client — клиент для взаимодействия с ML сервисом генерации эмбеддингов.
type Client interface {
	PrepareAndEmbed(ctx context.Context, req PrepareAndEmbedRequest) (*PrepareAndEmbedResponse, error)
	GetModelInfo(ctx context.Context) (*ModelInfo, error)
}

type client struct {
	httpClient *http.Client
	baseURL    string
	log        *slog.Logger
}

// NewClient создаёт новый клиент для ML сервиса.
func NewClient(cfg config.MLConfig, log *slog.Logger) Client {
	if !cfg.Enabled {
		return &noopClient{log: log}
	}

	return &client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL: cfg.BaseURL,
		log:     log,
	}
}

// PrepareAndEmbedRequest — запрос на подготовку текста и генерацию эмбеддинга.
type PrepareAndEmbedRequest struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Requirement map[string]interface{} `json:"requirement,omitempty"`
	Price       *int64                 `json:"price,omitempty"`
	District    *string                `json:"district,omitempty"`
	Rooms       *int32                  `json:"rooms,omitempty"`
	Area        *float64                `json:"area,omitempty"`
	Address     *string                 `json:"address,omitempty"`
}

// PrepareAndEmbedResponse — ответ с эмбеддингом.
type PrepareAndEmbedResponse struct {
	Embedding    []float64 `json:"embedding"`
	Dimensions   int       `json:"dimensions"`
	PreparedText string   `json:"prepared_text"`
}

// ModelInfo — информация о модели.
type ModelInfo struct {
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

// PrepareAndEmbed отправляет запрос на подготовку текста и генерацию эмбеддинга.
func (c *client) PrepareAndEmbed(ctx context.Context, req PrepareAndEmbedRequest) (*PrepareAndEmbedResponse, error) {
	const op = "ml.Client.PrepareAndEmbed"

	url := fmt.Sprintf("%s/prepare-and-embed", c.baseURL)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal request: %w", op, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create request: %w", op, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: unexpected status code %d: %s", op, resp.StatusCode, string(body))
	}

	var result PrepareAndEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%s: failed to decode response: %w", op, err)
	}

	return &result, nil
}

// GetModelInfo получает информацию о модели.
func (c *client) GetModelInfo(ctx context.Context) (*ModelInfo, error) {
	const op = "ml.Client.GetModelInfo"

	url := fmt.Sprintf("%s/model-info", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create request: %w", op, err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: unexpected status code %d: %s", op, resp.StatusCode, string(body))
	}

	var result ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%s: failed to decode response: %w", op, err)
	}

	return &result, nil
}

// noopClient — заглушка для случая, когда ML сервис отключен.
type noopClient struct {
	log *slog.Logger
}

func (c *noopClient) PrepareAndEmbed(ctx context.Context, req PrepareAndEmbedRequest) (*PrepareAndEmbedResponse, error) {
	c.log.Warn("ML service is disabled, returning empty embedding")
	// Возвращаем пустой вектор размерности 384 (стандартная размерность)
	embedding := make([]float64, 384)
	return &PrepareAndEmbedResponse{
		Embedding:    embedding,
		Dimensions:   384,
		PreparedText: "",
	}, nil
}

func (c *noopClient) GetModelInfo(ctx context.Context) (*ModelInfo, error) {
	return &ModelInfo{
		Model:      "disabled",
		Dimensions: 384,
	}, nil
}

