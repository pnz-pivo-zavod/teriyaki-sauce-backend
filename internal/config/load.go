package config

import (
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

func LoadAPI() (APIConfig, error) {
	var cfg APIConfig
	if err := load(&cfg, &cfg.Common); err != nil {
		return APIConfig{}, err
	}

	if err := validateAPI(&cfg); err != nil {
		return APIConfig{}, err
	}

	return cfg, nil
}

func LoadWorker() (WorkerConfig, error) {
	var cfg WorkerConfig
	if err := load(&cfg, &cfg.Common); err != nil {
		return WorkerConfig{}, err
	}

	if err := validateWorker(&cfg); err != nil {
		return WorkerConfig{}, err
	}

	return cfg, nil
}

func LoadMigrate() (MigrateConfig, error) {
	var cfg MigrateConfig
	if err := load(&cfg, &cfg.Common); err != nil {
		return MigrateConfig{}, err
	}

	if err := validateMigrate(&cfg); err != nil {
		return MigrateConfig{}, err
	}

	return cfg, nil
}

func load(cfg any, common *CommonConfig) error {
	configFile := strings.TrimSpace(os.Getenv(configFileField))
	processEnvironment := strings.ToLower(strings.TrimSpace(os.Getenv(envField)))
	if configFile != "" && processEnvironment == EnvironmentProduction {
		return invalid(configFileField, "is not allowed in production")
	}

	var err error
	if configFile == "" {
		err = cleanenv.ReadEnv(cfg)
	} else {
		err = cleanenv.ReadConfig(configFile, cfg)
	}
	if err != nil {
		return ErrInvalidConfig
	}

	if configFile != "" && strings.EqualFold(strings.TrimSpace(common.Environment), EnvironmentProduction) {
		return invalid(configFileField, "is not allowed in production")
	}

	return nil
}
