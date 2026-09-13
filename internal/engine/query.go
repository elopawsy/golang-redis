package engine

func (e *Engine) GetWhere(field, operator, operand string, number bool) ([]Entry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	keys, err := e.index.Find(field, operator, operand, number)
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	now := e.now()
	for _, key := range keys {
		entry := e.state[key]
		if expired(entry, now) {
			if !e.closed {
				e.remove(key)
			}
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
