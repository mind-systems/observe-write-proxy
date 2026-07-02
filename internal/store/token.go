package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned by SetActive and Delete when no row matches the
// given id, so callers (the admin handler) can map it to a 404 instead of
// silently succeeding.
var ErrNotFound = errors.New("store: token not found")

// Token is the row shape returned by Create, List, and Lookup.
type Token struct {
	ID        string
	Name      string
	Token     string
	Active    bool
	CreatedAt int64
}

// newID returns a random, URL-safe, unpadded identifier built from 16
// crypto/rand bytes.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newToken returns a fresh write token: an "otlp_" prefix followed by the
// URL-safe, unpadded encoding of 32 crypto/rand bytes.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: generate token: %w", err)
	}
	return "otlp_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// Create mints a new write token with the given display name, always
// inserted active, and returns the fully populated row.
func (s *Store) Create(name string) (Token, error) {
	id, err := newID()
	if err != nil {
		return Token{}, err
	}
	token, err := newToken()
	if err != nil {
		return Token{}, err
	}
	createdAt := time.Now().Unix()

	if _, err := s.db.Exec(
		`INSERT INTO tokens(id, name, token, active, created_at) VALUES(?, ?, ?, 1, ?)`,
		id, name, token, createdAt,
	); err != nil {
		return Token{}, fmt.Errorf("store: create token: %w", err)
	}

	return Token{ID: id, Name: name, Token: token, Active: true, CreatedAt: createdAt}, nil
}

// List returns every token row, active and inactive, ordered by creation
// time — the admin GUI needs to show and reactivate inactive tokens too.
func (s *Store) List() ([]Token, error) {
	rows, err := s.db.Query(`SELECT id, name, token, active, created_at FROM tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("store: list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var (
			t      Token
			active int
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &active, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: list tokens: %w", err)
		}
		t.Active = active != 0
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list tokens: %w", err)
	}

	return tokens, nil
}

// Lookup finds an active token by its value. The active=1 filter lives in
// SQL — never loaded then checked in Go, never cached — so deactivation
// takes effect on the very next request. A miss (unknown or inactive token)
// is the normal 401 path, not an error: ok is false with a nil error.
func (s *Store) Lookup(token string) (Token, bool, error) {
	var (
		t      Token
		active int
	)
	err := s.db.QueryRow(
		`SELECT id, name, token, active, created_at FROM tokens WHERE token=? AND active=1`,
		token,
	).Scan(&t.ID, &t.Name, &t.Token, &active, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: lookup token: %w", err)
	}
	t.Active = active != 0

	return t, true, nil
}

// SetActive flips the active flag for the token with the given id — the
// reversible soft-stop (deactivate/reactivate); the row is kept. Returns
// ErrNotFound if no row matches.
func (s *Store) SetActive(id string, active bool) error {
	value := 0
	if active {
		value = 1
	}

	res, err := s.db.Exec(`UPDATE tokens SET active=? WHERE id=?`, value, id)
	if err != nil {
		return fmt.Errorf("store: set active: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: set active: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete permanently removes the token with the given id — the only method
// that issues a DELETE. Returns ErrNotFound if no row matches.
func (s *Store) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM tokens WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("store: delete token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: delete token: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}
