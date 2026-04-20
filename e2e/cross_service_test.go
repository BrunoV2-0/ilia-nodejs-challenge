package e2e_test

import (
	"net/http"
	"testing"
)

// TestDeleteUser_X01_BlockedByBalance verifies that ms-users calls ms-transactions
// internally and rejects deletion when the user's wallet has a positive balance.
func TestDeleteUser_X01_BlockedByBalance(t *testing.T) {
	id, token := createUser(t)
	creditUser(t, token, 50)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+id, nil, internalToken)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("X01: expected 422 (wallet not empty), got %d", resp.StatusCode)
	}
}

// TestDeleteUser_X02_SucceedsAfterZeroingBalance verifies the full lifecycle:
// credit → debit (zeroing balance) → delete succeeds.
func TestDeleteUser_X02_SucceedsAfterZeroingBalance(t *testing.T) {
	id, token := createUser(t)
	creditUser(t, token, 50)
	debitUser(t, token, 50)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+id, nil, internalToken)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("X02: expected 204 (deleted), got %d", resp.StatusCode)
	}
}

// TestDeleteUser_X03_MultipleCreditsFullDebitThenDelete verifies that aggregated
// credits and a single full debit correctly zero the balance, allowing deletion.
func TestDeleteUser_X03_MultipleCreditsFullDebitThenDelete(t *testing.T) {
	id, token := createUser(t)
	creditUser(t, token, 30)
	creditUser(t, token, 70)
	debitUser(t, token, 100)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+id, nil, internalToken)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("X03: expected 204, got %d", resp.StatusCode)
	}
}
