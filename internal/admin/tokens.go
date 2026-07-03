package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mind-systems/observe-write-proxy/internal/store"
)

// tokenStore is the minimal store capability the management API needs. It
// exists so this package depends only on the shape it uses, not on
// *store.Store's full surface — mirrors the grafanaValidator narrow-interface
// pattern in auth.go. *store.Store satisfies it via Create, List, SetActive,
// and Delete.
type tokenStore interface {
	Create(name string) (store.Token, error)
	List() ([]store.Token, error)
	SetActive(id string, active bool) error
	Delete(id string) error
}

// TokenHandlers implements the Grafana-gated management API for write-plane
// tokens: minting, listing, toggling, and deleting them via the underlying
// store.
type TokenHandlers struct {
	store tokenStore
}

// NewTokenHandlers returns a *TokenHandlers backed by s.
func NewTokenHandlers(s tokenStore) *TokenHandlers {
	return &TokenHandlers{store: s}
}

// tokenResponse is the JSON shape returned for a single token. Token carries
// the plaintext value — acceptable per the store's own convention (the value
// is meant to be read back in the GUI) — so it is never logged here.
type tokenResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"created_at"`
}

// newTokenResponse maps a store.Token onto its JSON DTO.
func newTokenResponse(t store.Token) tokenResponse {
	return tokenResponse{
		ID:        t.ID,
		Name:      t.Name,
		Token:     t.Token,
		Active:    t.Active,
		CreatedAt: t.CreatedAt,
	}
}

// writeJSON sets the JSON content type, writes status, and encodes payload
// as the response body.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Collection serves /api/tokens: GET lists every token, POST mints a new
// one. Any other method — notably DELETE and PATCH, which only make sense
// against a single token — is rejected with 405.
func (h *TokenHandlers) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tokens, err := h.store.List()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := make([]tokenResponse, len(tokens))
		for i, t := range tokens {
			resp[i] = newTokenResponse(t)
		}
		writeJSON(w, http.StatusOK, resp)

	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "malformed request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			http.Error(w, "name must not be empty", http.StatusBadRequest)
			return
		}

		t, err := h.store.Create(body.Name)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, newTokenResponse(t))

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Item serves /api/tokens/<id>: PATCH toggles the active flag, DELETE
// removes the token permanently. An empty id (a bare /api/tokens/ request)
// is a 404 — there is no item to address.
func (h *TokenHandlers) Item(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Active *bool `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "malformed request body", http.StatusBadRequest)
			return
		}
		if body.Active == nil {
			http.Error(w, "active field is required", http.StatusBadRequest)
			return
		}

		if err := h.store.SetActive(id, *body.Active); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		if err := h.store.Delete(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "PATCH, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
