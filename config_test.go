package main

import (
	"strings"
	"testing"
)

func TestReadConfig(t *testing.T) {
	tests := []struct {
		input          string
		shouldError    bool
		expectedConfig config
	}{
		{input: ``, shouldError: false, expectedConfig: config{
			Services: []serviceConfig{},
		}},
		{input: `
			[[service]]
			name = "foo"
			`,
			shouldError: true,
		},
		{input: `
			[[service]]
			name = "foo"
			port = 3030
			program = "bar"
			[[service]]
			name = "foo"
			port = 3040
			program = "bar"
			`,
			shouldError: true,
		},
		{input: `
			[[service]]
			name = "foo"
			port = 3030
			program = "/bin/foo"
			[[service]]
			name = "bar"
			port = 8080
			program = "/bin/bar"
			program-args = ["first", "second"]
			`,
			shouldError: false,
			expectedConfig: config{
				Services: []serviceConfig{
					{
						Name:    "foo",
						Port:    3030,
						Program: "/bin/foo",
					},
					{
						Name:        "bar",
						Port:        8080,
						Program:     "/bin/bar",
						ProgramArgs: []string{"first", "second"},
					},
				},
			},
		},
	}

	for i, test := range tests {
		cfg, err := readConfig(strings.NewReader(test.input))

		if err != nil {
			if !test.shouldError {
				t.Fatalf("Test %d: Unexpected error %v\n", i, err)
			}
			continue
		}

		if test.shouldError {
			t.Fatalf("Test %d: No expected error returned\n", i)
		}

		expectedConfig := test.expectedConfig
		if len(cfg.Services) != len(expectedConfig.Services) {
			t.Fatalf("Test %d: Wrong number of services in config: %d, expected %d\n",
				i, len(cfg.Services), len(expectedConfig.Services))
		}

		for serviceIdx, serviceCfg := range cfg.Services {
			expectedServiceCfg := expectedConfig.Services[serviceIdx]

			if serviceCfg.Name != expectedServiceCfg.Name {
				t.Errorf("Test %d: Wrong service name '%s', expected '%s'\n",
					i, serviceCfg.Name, expectedServiceCfg.Name)
			}

			if serviceCfg.Port != expectedServiceCfg.Port {
				t.Errorf("Test %d: Wrong service port '%d', expected '%d'\n",
					i, serviceCfg.Port, expectedServiceCfg.Port)
			}

			if serviceCfg.Program != expectedServiceCfg.Program {
				t.Errorf("Test %d: Wrong service program '%s', expected '%s'\n",
					i, serviceCfg.Program, expectedServiceCfg.Program)
			}

			if len(serviceCfg.ProgramArgs) != len(expectedServiceCfg.ProgramArgs) {
				t.Errorf("Test %d: Wrong number of program args in config: %d, expected %d\n",
					i, len(serviceCfg.ProgramArgs), len(expectedServiceCfg.ProgramArgs))

				for programArgIdx, programArg := range serviceCfg.ProgramArgs {
					expectedProgramArg := expectedServiceCfg.ProgramArgs[programArgIdx]

					if programArg != expectedProgramArg {
						t.Errorf("Test %d: Wrong program arg value '%s', expected '%s'\n",
							i, programArg, expectedProgramArg)
					}
				}
			}
		}
	}
}
