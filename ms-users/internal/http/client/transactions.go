package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TransactionsClient struct {
	baseURL     string
	internalKey []byte
	http        *http.Client
}

func New(baseURL, internalKey string) *TransactionsClient {
	return &TransactionsClient{
		baseURL:     baseURL,
		internalKey: []byte(internalKey),
		http:        &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *TransactionsClient) HasBalance(userID uuid.UUID) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/wallets/balance", nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}

	token, err := c.mintToken(userID)
	if err != nil {
		return false, fmt.Errorf("minting internal token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling transactions service: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Balance float64 `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("decoding balance response: %w", err)
	}

	return body.Balance > 0, nil
}

func (c *TransactionsClient) mintToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(c.internalKey)
}
