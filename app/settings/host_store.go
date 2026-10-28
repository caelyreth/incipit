package settings

import "github.com/caelyreth/neverland/engine/kernel"

// HostStore is the policy-authoritative settings store used by the app
// controller. Adoption validates the complete candidate before publication.
type HostStore struct {
	host *kernel.PolicyHost
}

// NewHostStore installs the linked catalog's defaults.
func NewHostStore(catalog kernel.PolicyCatalog) (*HostStore, error) {
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		return nil, err
	}
	return &HostStore{host: host}, nil
}

func (s *HostStore) PolicyCatalog() kernel.PolicyCatalog {
	if s == nil || s.host == nil {
		return kernel.PolicyCatalog{}
	}
	return s.host.Catalog()
}

func (s *HostStore) Configuration() kernel.PolicyConfiguration {
	if s == nil || s.host == nil {
		return kernel.PolicyConfiguration{}
	}
	return s.host.Configuration()
}

func (s *HostStore) PreparedConfiguration() *kernel.PreparedPolicyConfiguration {
	if s == nil || s.host == nil {
		return nil
	}
	return s.host.PreparedSnapshot()
}

// Adopt atomically installs a complete source configuration.
func (s *HostStore) Adopt(configuration kernel.PolicyConfiguration) error {
	if s == nil || s.host == nil {
		return ErrInvalidChange
	}
	return s.host.Reconfigure(func(candidate *kernel.PolicyConfiguration) error {
		*candidate = configuration.Clone()
		return nil
	})
}
