package testing

import "github.com/cresta/go-servers/config/shared/grpcclient/config"

type FakeConfig struct {
	config.Client
}

func (*FakeConfig) GetProfileConfig(customerID string, profileID string) (*config.ProfileConfig, error) {
	return &config.ProfileConfig{}, nil
}
