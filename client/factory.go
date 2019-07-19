package client

import (
	"fmt"
)

// NewProviderFactory is a helper method to create a Retriever each Provider being used
func NewProviderFactory(config *Config) (*Factory, error) {
	retrievers := make(map[string]Retriever)
	var err error
	for provider := range config.Providers {
		switch provider {
		case "azure":
			retrievers[provider], err = NewAzureRetriever(config.Providers[provider])
		case "osprey":
			retrievers[provider], err = NewOspreyRetriever(config.Providers[provider])
		default:
			return nil, fmt.Errorf("unsupported provider: %s", provider)
		}
		if retrievers[provider] != nil && config.Interactive {
			retrievers[provider].SetInteractive(config.Interactive)
		}
	}
	if err != nil {
		return nil, err
	}

	return &Factory{
		retrievers: retrievers,
	}, nil
}

// Factory holds a map of Provider names to Retrievers
type Factory struct {
	retrievers map[string]Retriever
}

// GetRetriever Returns the Retriever object for a named Provider
func (c *Factory) GetRetriever(providerType string) (Retriever, error) {
	retriever := c.retrievers[providerType]
	if retriever == nil {
		return nil, fmt.Errorf("unable to find retriever type for %s", providerType)
	}
	return retriever, nil
}
