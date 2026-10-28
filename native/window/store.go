package window

import (
	"sync"

	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/kernel"
)

type policyStore struct {
	mu            sync.RWMutex
	catalog       kernel.PolicyCatalog
	configuration kernel.PolicyConfiguration
	prepared      *kernel.PreparedPolicyConfiguration
	workspace     *workspace.Workspace
}

func newPolicyStore(
	catalog kernel.PolicyCatalog,
	configuration kernel.PolicyConfiguration,
	prepared *kernel.PreparedPolicyConfiguration,
	pages *workspace.Workspace,
) *policyStore {
	return &policyStore{
		catalog:       catalog,
		configuration: configuration.Clone(),
		prepared:      prepared.Clone(),
		workspace:     pages,
	}
}

func (s *policyStore) PolicyCatalog() kernel.PolicyCatalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalog.Clone()
}

func (s *policyStore) Configuration() kernel.PolicyConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configuration.Clone()
}

func (s *policyStore) PreparedConfiguration() *kernel.PreparedPolicyConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.prepared.Clone()
}

func (s *policyStore) Adopt(configuration kernel.PolicyConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, err := kernel.NewPolicyHost(s.catalog)
	if err != nil {
		return err
	}
	if err := host.Reconfigure(func(candidate *kernel.PolicyConfiguration) error {
		*candidate = configuration.Clone()
		return nil
	}); err != nil {
		return err
	}
	if s.workspace != nil {
		if err := s.workspace.PublishPolicy(host.Configuration()); err != nil {
			return err
		}
	}
	s.configuration = host.Configuration()
	s.prepared = host.PreparedSnapshot()
	return nil
}

var _ settings.Store = (*policyStore)(nil)
