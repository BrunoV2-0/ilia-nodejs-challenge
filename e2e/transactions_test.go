package e2e_test

import (
	"net/http"
	"testing"
)

// ── POST /transactions ───────────────────────────────────────────────────────

func TestCreateTransaction_T01_Credit(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 100.0,
		"type":   "CREDIT",
	}, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("T01: expected 201, got %d", resp.StatusCode)
	}
	if body["id"] == nil {
		t.Fatal("T01: transaction id missing from response")
	}
}

func TestCreateTransaction_T02_DebitAfterCredit(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 100)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 40.0,
		"type":   "DEBIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("T02: expected 201, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T03_InsufficientFunds(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 10)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 50.0,
		"type":   "DEBIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("T03: expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T04_DebitOnZeroBalance(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 1.0,
		"type":   "DEBIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("T04: expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T05_InvalidType(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 10.0,
		"type":   "INVALID",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("T05: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T06_ZeroAmount(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 0.0,
		"type":   "CREDIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("T06: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T07_NegativeAmount(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": -5.0,
		"type":   "CREDIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("T07: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T08_MissingAmount(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"type": "CREDIT",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("T08: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_T09_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": 10.0,
		"type":   "CREDIT",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("T09: expected 401, got %d", resp.StatusCode)
	}
}

// ── GET /transactions ────────────────────────────────────────────────────────

func TestListTransactions_TL01_All(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 50)
	creditUser(t, token, 30)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/transactions", nil, token)
	var txs []map[string]any
	decodeBody(t, resp, &txs)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TL01: expected 200, got %d", resp.StatusCode)
	}
	if len(txs) < 2 {
		t.Fatalf("TL01: expected at least 2 transactions, got %d", len(txs))
	}
}

func TestListTransactions_TL02_FilterCredit(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 100)
	debitUser(t, token, 40)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/transactions?type=CREDIT", nil, token)
	var txs []map[string]any
	decodeBody(t, resp, &txs)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TL02: expected 200, got %d", resp.StatusCode)
	}
	for _, tx := range txs {
		if tx["type"] != "CREDIT" {
			t.Fatalf("TL02: expected only CREDIT, got %v", tx["type"])
		}
	}
}

func TestListTransactions_TL03_FilterDebit(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 100)
	debitUser(t, token, 40)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/transactions?type=DEBIT", nil, token)
	var txs []map[string]any
	decodeBody(t, resp, &txs)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TL03: expected 200, got %d", resp.StatusCode)
	}
	for _, tx := range txs {
		if tx["type"] != "DEBIT" {
			t.Fatalf("TL03: expected only DEBIT, got %v", tx["type"])
		}
	}
}

func TestListTransactions_TL04_EmptyHistory(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/transactions", nil, token)
	var txs []map[string]any
	decodeBody(t, resp, &txs)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TL04: expected 200, got %d", resp.StatusCode)
	}
	if len(txs) != 0 {
		t.Fatalf("TL04: expected empty array, got %d items", len(txs))
	}
}

func TestListTransactions_TL05_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodGet, transactionsURL+"/transactions", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("TL05: expected 401, got %d", resp.StatusCode)
	}
}

// ── GET /wallets/balance ─────────────────────────────────────────────────────

func TestGetBalance_B01_CreditsOnly(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 100)
	creditUser(t, token, 50)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/wallets/balance", nil, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("B01: expected 200, got %d", resp.StatusCode)
	}
	if body["balance"] != 150.0 {
		t.Fatalf("B01: expected balance 150, got %v", body["balance"])
	}
}

func TestGetBalance_B02_AfterDebit(t *testing.T) {
	_, token := createUser(t)
	creditUser(t, token, 100)
	debitUser(t, token, 30)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/wallets/balance", nil, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("B02: expected 200, got %d", resp.StatusCode)
	}
	if body["balance"] != 70.0 {
		t.Fatalf("B02: expected balance 70, got %v", body["balance"])
	}
}

func TestGetBalance_B03_NewUser(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodGet, transactionsURL+"/wallets/balance", nil, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("B03: expected 200, got %d", resp.StatusCode)
	}
	if body["balance"] != 0.0 {
		t.Fatalf("B03: expected balance 0, got %v", body["balance"])
	}
}

func TestGetBalance_B04_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodGet, transactionsURL+"/wallets/balance", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("B04: expected 401, got %d", resp.StatusCode)
	}
}
