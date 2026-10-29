package memory

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Keys is the self-service key sub-client returned by [Client.Keys]. It manages
// scoped bearer keys under /api/v1/{context}/keys.
type Keys struct {
	client *Client
}

// MintedKey is a freshly minted or rotated key. Key is the full bearer secret
// (`sp-{id}-{secret}`) and is returned only once, at creation or rotation time;
// store it immediately.
type MintedKey struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	ValidUntil string `json:"validUntil,omitempty"`
}

// KeyDetail describes an existing key. The secret is never returned here.
type KeyDetail struct {
	ID         string              `json:"id"`
	Name       string              `json:"name,omitempty"`
	Grants     map[string][]string `json:"grants,omitempty"`
	CreatedAt  string              `json:"createdAt,omitempty"`
	LastUsedAt string              `json:"lastUsedAt,omitempty"`
	ValidUntil string              `json:"validUntil,omitempty"`
}

// CreateKeyRequest is the input to [Keys.Create].
type CreateKeyRequest struct {
	Name   string              `json:"name,omitempty"`
	Grants map[string][]string `json:"grants,omitempty"`
	// TTLSeconds, when greater than zero, bounds the key's lifetime. It is sent
	// as the ttlSeconds query parameter rather than in the request body.
	TTLSeconds int `json:"-"`
}

// ttlSecondsQuery builds the ttlSeconds query parameter, or nil when unset.
func ttlSecondsQuery(ttl int) url.Values {
	if ttl <= 0 {
		return nil
	}
	q := url.Values{}
	q.Set("ttlSeconds", strconv.Itoa(ttl))
	return q
}

// Create mints a new self-service key. The returned MintedKey.Key holds the
// secret, exposed only on this response.
func (k *Keys) Create(ctx context.Context, req CreateKeyRequest) (*MintedKey, error) {
	path := k.client.base + "/keys"
	if q := ttlSecondsQuery(req.TTLSeconds); q != nil {
		path += "?" + q.Encode()
	}
	var out MintedKey
	if err := k.client.doJSON(ctx, http.MethodPost, path, req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the keys registered for the context.
func (k *Keys) List(ctx context.Context) ([]KeyDetail, error) {
	var out []KeyDetail
	if err := k.client.getJSON(ctx, k.client.base+"/keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete revokes a key by name.
func (k *Keys) Delete(ctx context.Context, name string) error {
	path := k.client.base + "/keys/" + url.PathEscape(name)
	return k.client.doJSON(ctx, http.MethodDelete, path, nil, nil, false)
}

// Rotate issues a fresh secret for an existing key, optionally resetting its
// TTL. The previous secret stops working. The returned MintedKey.Key holds the
// new secret, exposed only on this response.
func (k *Keys) Rotate(ctx context.Context, name string, ttlSeconds int) (*MintedKey, error) {
	path := k.client.base + "/keys/" + url.PathEscape(name) + "/rotate"
	if q := ttlSecondsQuery(ttlSeconds); q != nil {
		path += "?" + q.Encode()
	}
	var out MintedKey
	if err := k.client.doJSON(ctx, http.MethodPost, path, nil, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}
