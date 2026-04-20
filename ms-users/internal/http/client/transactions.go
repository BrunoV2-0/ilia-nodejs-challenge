package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const callerIdentity = "ms-users"

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
	url := fmt.Sprintf("%s/wallets/balance?user_id=%s", c.baseURL, userID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}

	token, err := c.mintToken()
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

func (c *TransactionsClient) mintToken() (string, error) {
	claims := jwt.MapClaims{
		"sub": callerIdentity,
		"exp": time.Now().Add(time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(c.internalKey)
}
