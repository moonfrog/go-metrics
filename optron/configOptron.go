package optron

import (
	"fmt"

	"github.com/moonfrog/nucleus/zootils"
)

type ConfigOptronDef struct {
	Address        string
	HasBulkSupport bool `json:",string"`
	BatchSize      int  `json:",string"`
}

func getOptronConfig(configUri string) (*ConfigOptronDef, error) {
	config := &ConfigOptronDef{}
	err := zootils.GetInstance().LoadConfig(config, configUri, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("optron: load: %v", err)
	}

	if config.Address == "" {
		return nil, fmt.Errorf("Optron config: %+v", config)
	}

	return config, nil
}
