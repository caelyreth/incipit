package controller

// MetricsForCurrentText returns the page's semantic counts. The session keeps
// them current block by block, so they are always ready.
func (c *DocumentController) MetricsForCurrentText() (Metrics, bool) {
	counts, ok := c.session.Statistics()
	if !ok {
		return Metrics{}, false
	}
	return Metrics{
		Characters: counts.Characters,
		Words:      counts.Words,
		Paragraphs: counts.Paragraphs,
	}, true
}

// Metrics returns the page's semantic counts, or none while they cannot be
// counted.
func (c *DocumentController) Metrics() Metrics {
	metrics, _ := c.MetricsForCurrentText()
	return metrics
}
