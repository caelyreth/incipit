package settings

import (
	"sync"

	"github.com/caelyreth/neverland/engine/kernel"
)

// HostStore is the policy-authoritative settings store used by the app
// controller. Adoption validates the complete candidate before publication.
type HostStore struct {
	mu      sync.RWMutex
	host    *kernel.PolicyHost
	publish func(kernel.PolicyConfiguration) error
}

// NewHostStore installs the linked catalog's defaults.
func NewHostStore(catalog kernel.PolicyCatalog) (*HostStore, error) {
	return NewHostStoreWithPublisher(catalog, nil)
}

// NewHostStoreWithPublisher prepares policy changes before publishing them to
// the workspace. A failed publisher leaves the store unchanged. The publisher
// must not call back into the store.
func NewHostStoreWithPublisher(
	catalog kernel.PolicyCatalog,
	publish func(kernel.PolicyConfiguration) error,
) (*HostStore, error) {
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		return nil, err
	}
	return &HostStore{host: host, publish: publish}, nil
}

// Snapshot returns a coherent, independently owned policy generation.
func (s *HostStore) Snapshot() PolicySnapshot {
	if s == nil {
		return PolicySnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.host == nil {
		return PolicySnapshot{}
	}
	return PolicySnapshot{
		Catalog: s.host.Catalog(), Configuration: s.host.Configuration(),
		Prepared: s.host.PreparedSnapshot(),
	}
}

func (s *HostStore) PolicyCatalog() kernel.PolicyCatalog {
	if s == nil {
		return kernel.PolicyCatalog{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.host.Catalog()
}

func (s *HostStore) Configuration() kernel.PolicyConfiguration {
	if s == nil {
		return kernel.PolicyConfiguration{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.host.Configuration()
}

func (s *HostStore) PreparedConfiguration() *kernel.PreparedPolicyConfiguration {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.host.PreparedSnapshot()
}

// Adopt atomically installs a complete source configuration.
func (s *HostStore) Adopt(configuration kernel.PolicyConfiguration) error {
	if s == nil {
		return ErrInvalidChange
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.host == nil {
		return ErrInvalidChange
	}
	host := s.host.Clone()
	if err := host.Reconfigure(func(candidate *kernel.PolicyConfiguration) error {
		*candidate = configuration.Clone()
		return nil
	}); err != nil {
		return err
	}
	if s.publish != nil {
		if err := s.publish(host.Configuration()); err != nil {
			return err
		}
	}
	s.host = host
	return nil
}
