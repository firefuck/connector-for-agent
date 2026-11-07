package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

func Secret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func bearer(r *http.Request) string {
	s := r.Header.Get("Authorization")
	if !strings.HasPrefix(s, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(s, "Bearer ")
}
func (g *Gateway) agent(key string) *Agent {
	if key == "" {
		return nil
	}
	h := Hash(key)
	for _, a := range g.State.Agents {
		if a.Key == h && !a.Revoked {
			return a
		}
	}
	return nil
}
func (g *Gateway) device(key string) *Device {
	if key == "" {
		return nil
	}
	h := Hash(key)
	for _, d := range g.State.Devices {
		if d.Key == h && !d.Revoked {
			return d
		}
	}
	return nil
}
func allowed(a *Agent, id string) bool {
	for _, d := range a.Devices {
		if d == id || d == "*" {
			return true
		}
	}
	return false
}
