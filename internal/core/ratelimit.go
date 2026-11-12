package core

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// authLimiter bounds credential guessing per client address. It counts only
// failed authentication; a client that stays under the limit is never delayed.
// Once an address exceeds the limit every credentialed request from it is
// refused until the window ends, because answering only the valid ones would
// tell a guesser which credential was right.
type authLimiter struct {
	mu      sync.Mutex
	clients map[string]*authFailures
}
type authFailures struct {
	since time.Time
	count int
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// retryAfter returns how long the client stays blocked, or zero.
func (l *authLimiter) retryAfter(client string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.clients[client]
	if f == nil || f.count < maxAuthFailures {
		return 0
	}
	if left := authFailureWindow - now.Sub(f.since); left > 0 {
		return left
	}
	delete(l.clients, client)
	return 0
}

// fail records one failure and reports whether it just triggered the block.
func (l *authLimiter) fail(client string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clients == nil {
		l.clients = map[string]*authFailures{}
	}
	f := l.clients[client]
	if f != nil && now.Sub(f.since) >= authFailureWindow {
		f = nil
	}
	if f == nil {
		if len(l.clients) >= maxTrackedClients {
			for address, old := range l.clients {
				if now.Sub(old.since) >= authFailureWindow {
					delete(l.clients, address)
				}
			}
		}
		if len(l.clients) >= maxTrackedClients {
			return false // Bounded memory wins over tracking one more address.
		}
		f = &authFailures{since: now}
		l.clients[client] = f
	}
	f.count++
	return f.count == maxAuthFailures
}

// authFailed must be called without g.mu held.
func (g *Gateway) authFailed(r *http.Request) {
	client := clientAddress(r)
	if g.limiter.fail(client, time.Now()) {
		g.mu.Lock()
		g.log("auth_blocked", client, "", r.URL.Path)
		g.mu.Unlock()
	}
}

// authBlocked answers 429 for a blocked client and reports whether it did.
func (g *Gateway) authBlocked(w http.ResponseWriter, r *http.Request) bool {
	left := g.limiter.retryAfter(clientAddress(r), time.Now())
	if left <= 0 {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(left/time.Second)+1))
	fail(w, 429, "too many failed authentication attempts; retry later")
	return true
}
