package email

import (
	"context"
	"sync"
)

// Recorder is a Sender that keeps messages instead of sending them, for
// tests and for running aio locally without SMTP.
type Recorder struct {
	mu   sync.Mutex
	Sent []Message
	Err  error
}

func (r *Recorder) Send(_ context.Context, msg Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.Sent = append(r.Sent, msg)
	return nil
}

func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.Sent...)
}
