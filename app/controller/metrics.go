package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
)

// MetricSource identifies the immutable semantic text root described by an
// asynchronous metrics computation.
type MetricSource struct {
	Node    model.NodeID
	Root    model.TextRoot
	HasRoot bool
}

// Compute calculates semantic scalar, word, and paragraph counts for the
// source's retained text root.
func (s MetricSource) Compute() (Metrics, bool) {
	if !s.HasRoot {
		return Metrics{}, true
	}
	counts, ok := editor.ComputeTextMetrics(s.Root.String())
	if !ok {
		return Metrics{}, false
	}
	return Metrics{
		Characters: counts.Characters,
		Words:      counts.Words,
		Paragraphs: counts.Paragraphs,
	}, true
}

func (s MetricSource) same(other MetricSource) bool {
	if s.Node != other.Node || s.HasRoot != other.HasRoot {
		return false
	}
	return !s.HasRoot || s.Root.Same(other.Root)
}

func metricSource(source editor.TextMetricsSource) MetricSource {
	return MetricSource{
		Node:    source.Node,
		Root:    source.Root,
		HasRoot: source.HasRoot,
	}
}

func metricsFrom(source editor.TextMetricsSource) Metrics {
	return Metrics{
		Characters: source.Metrics.Characters,
		Words:      source.Metrics.Words,
		Paragraphs: source.Metrics.Paragraphs,
	}
}

// MetricsForCurrentText computes semantic counts from the current node.
// It is the synchronous fallback for command-line and native status surfaces;
// asynchronous callers can still use PendingMetricSource.
func (c *DocumentController) MetricsForCurrentText() (Metrics, bool) {
	source, ok := c.session.Statistics()
	if !ok {
		return Metrics{}, false
	}
	metrics, ok := metricSource(source).Compute()
	if !ok {
		return Metrics{}, false
	}
	if !c.InstallMetrics(metricSource(source), metrics) {
		return Metrics{}, false
	}
	return metrics, true
}

// Metrics returns the last installed semantic counts.
func (c *DocumentController) Metrics() Metrics { return c.metrics }

// MetricsPending reports whether a newer semantic text root awaits metrics.
func (c *DocumentController) MetricsPending() bool { return c.metricsQueued }

// PendingMetricSource returns the root whose metrics should be computed.
func (c *DocumentController) PendingMetricSource() (MetricSource, bool) {
	return c.pendingMetric, c.metricsQueued
}

func (c *DocumentController) initializeMetrics() {
	source, ok := c.session.Statistics()
	if !ok {
		return
	}
	c.metrics = metricsFrom(source)
	c.metricsSource = metricSource(source)
	c.metricsReady = true
}

func (c *DocumentController) refreshMetricSource() {
	source, ok := c.session.Statistics()
	if !ok {
		c.metricsQueued = false
		return
	}
	next := metricSource(source)
	if !c.metricsReady || !c.metricsSource.same(next) {
		c.pendingMetric = next
		c.metricsQueued = true
		return
	}
	c.metricsQueued = false
}

// InstallMetrics applies a computed source only if its text root is still
// current. It returns false for stale or invalid work.
func (c *DocumentController) InstallMetrics(source MetricSource, metrics Metrics) bool {
	if metrics.Characters < 0 || metrics.Words < 0 || metrics.Paragraphs < 0 {
		return false
	}
	current, ok := c.session.Statistics()
	if !ok || !source.same(metricSource(current)) {
		return false
	}
	c.metrics = metrics
	c.metricsSource = source
	c.metricsReady = true
	if c.metricsQueued && c.pendingMetric.same(source) {
		c.metricsQueued = false
	}
	return true
}

// RefreshMetrics computes and installs the current pending source
// synchronously. Long documents may instead use PendingMetricSource.
func (c *DocumentController) RefreshMetrics() bool {
	var source MetricSource
	if c.metricsQueued {
		source = c.pendingMetric
	} else if c.metricsReady {
		source = c.metricsSource
	} else {
		return false
	}
	metrics, ok := source.Compute()
	if !ok {
		return false
	}
	return c.InstallMetrics(source, metrics)
}
