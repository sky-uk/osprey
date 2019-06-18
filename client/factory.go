package client

import (
	"fmt"
)

func NewProviderFactory(config *Config) (*factory, error) {
	retrievers := make(map[string]Retriever)
	var err error
	for provider := range config.Providers {
		switch provider {
		case "azure":
			retrievers[provider], err = NewAzureRetriever(config.Providers[provider])
		case "google":
			retrievers[provider] = NewGoogleRetriever(config.Providers[provider])
		case "osprey":
			retrievers[provider] = NewOspreyRetriever(config.Providers[provider])
		default:
			return nil, fmt.Errorf("unsupported provider: %s", provider)
		}
		retrievers[provider].SetInteractive(config.Interactive)
	}
	if err != nil {
		return nil, err
	}

	return &factory{
		retrievers: retrievers,
	}, nil
}

type factory struct {
	retrievers map[string]Retriever
}

func (c *factory) GetRetriever(providerType string) (Retriever, error) {
	retriever := c.retrievers[providerType]
	if retriever == nil {
		return nil, fmt.Errorf("unable to find retriever type for %s", providerType)
	}
	return retriever, nil
}
