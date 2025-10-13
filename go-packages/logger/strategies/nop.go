package strategies

// Nop is a no-operation strategy that discards all log entries
// This is useful for testing where you don't want any log output
type Nop struct{}

// NewNop creates a new Nop strategy
func NewNop() *Nop {
	return &Nop{}
}

// Log implements the Strategy interface (does nothing)
func (n *Nop) Log(entry Entry) error {
	return nil
}

// Sync implements the Strategy interface (does nothing)
func (n *Nop) Sync() error {
	return nil
}
