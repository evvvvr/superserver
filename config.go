package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

type serviceConfig struct {
	Name        string
	Port        int
	Program     string
	ProgramArgs []string `toml:"program-args"`
}

type config struct {
	Services []serviceConfig `toml:"service"`
}

func readConfig(reader io.Reader) (*config, error) {
	var res config

	_, err := toml.DecodeReader(reader, &res)
	if err != nil {
		return nil, err
	}

	for i, service := range res.Services {
		if service.Name == "" {
			return nil, fmt.Errorf("Service name required")
		}

		if service.Port == 0 {
			return nil, fmt.Errorf("Service port required")
		}

		if service.Program == "" {
			return nil, fmt.Errorf("Service program required")
		}

		for j, anotherService := range res.Services {
			if j == i {
				continue
			}

			if anotherService.Name == service.Name {
				return nil, fmt.Errorf("Duplicate service name: %s",
					service.Name)
			}

			if anotherService.Port == service.Port {
				return nil, fmt.Errorf("Duplicate service port: %d",
					service.Port)
			}
		}
	}

	return &res, nil
}

func readConfigFromFile(fileName string) (*config, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	return readConfig(file)
}
