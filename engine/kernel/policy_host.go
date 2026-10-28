package kernel

// PolicyHost owns one linked catalog and its current prepared configuration.
// The zero value is not usable; construct it with NewPolicyHost.
type PolicyHost struct {
	catalog       PolicyCatalog
	configuration PolicyConfiguration
	prepared      *PreparedPolicyConfiguration
}

// NewPolicyHost applies catalog defaults and prepares them before returning.
func NewPolicyHost(catalog PolicyCatalog) (*PolicyHost, error) {
	catalog = catalog.Clone()
	configuration := DefaultPolicyConfiguration(&catalog)
	prepared, err := preparePolicyConfiguration(&catalog, &configuration)
	if err != nil {
		return nil, err
	}
	return &PolicyHost{
		catalog:       catalog,
		configuration: configuration,
		prepared:      prepared,
	}, nil
}

// Catalog returns an independent linked catalog snapshot.
func (h *PolicyHost) Catalog() PolicyCatalog {
	if h == nil {
		return PolicyCatalog{}
	}
	return h.catalog.Clone()
}

// Configuration returns an independent snapshot including overrides for
// disabled policies and explicit unbindings.
func (h *PolicyHost) Configuration() PolicyConfiguration {
	if h == nil {
		return PolicyConfiguration{}
	}
	return h.configuration.Clone()
}

// Prepared returns the current immutable prepared configuration owned by the
// host. Its maps are private, but callers that need to retain a historical
// snapshot across reconfiguration should use PreparedSnapshot.
func (h *PolicyHost) Prepared() *PreparedPolicyConfiguration {
	if h == nil {
		return nil
	}
	return h.prepared
}

// PreparedSnapshot returns an independent immutable snapshot.
func (h *PolicyHost) PreparedSnapshot() *PreparedPolicyConfiguration {
	if h == nil {
		return nil
	}
	return h.prepared.Clone()
}

// Reconfigure applies operation to a candidate copy and atomically installs the
// candidate configuration and keymap only after complete preparation succeeds.
func (h *PolicyHost) Reconfigure(
	operation func(*PolicyConfiguration) error,
) error {
	if h == nil || h.prepared == nil {
		return &PolicyError{Kind: PolicyErrorUnknownPolicy}
	}
	candidate := h.configuration.Clone()
	if operation != nil {
		if err := operation(&candidate); err != nil {
			return err
		}
	}
	prepared, err := preparePolicyConfiguration(&h.catalog, &candidate)
	if err != nil {
		return err
	}
	h.configuration = candidate
	h.prepared = prepared
	return nil
}

// Clone returns an independent host. Later reconfiguration of either host does
// not affect the other.
func (h *PolicyHost) Clone() *PolicyHost {
	if h == nil {
		return nil
	}
	cloned := &PolicyHost{
		catalog:       h.catalog.Clone(),
		configuration: h.configuration.Clone(),
	}
	if h.prepared != nil {
		cloned.prepared = h.prepared.Clone()
	}
	return cloned
}

func (p *PreparedPolicyConfiguration) Clone() *PreparedPolicyConfiguration {
	if p == nil {
		return nil
	}
	cloned := &PreparedPolicyConfiguration{
		enabled:  make(map[PolicyID]struct{}, len(p.enabled)),
		settings: make(map[SettingKey]SettingValue, len(p.settings)),
		keymap:   p.keymap.Clone(),
	}
	for policy := range p.enabled {
		cloned.enabled[policy] = struct{}{}
	}
	for key, value := range p.settings {
		cloned.settings[key] = value
	}
	return cloned
}
