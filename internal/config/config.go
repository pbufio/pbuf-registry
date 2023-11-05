package config

import (
	"bytes"
	_ "embed"
	"strings"

	"github.com/spf13/viper"
)

//go:embed config.yaml
var embeddedConfigBytes []byte

type Config struct {
	Server struct {
		HTTP struct {
			Addr string `mapstructure:"addr"`
		}
		GRPC struct {
			Addr string `mapstructure:"addr"`
		}
	}

	Data struct {
		Database struct {
			DSN string `mapstructure:"dsn"`
		}
	}
}

// Cfg is the global config
var Cfg = &Config{}

// Loader is the config loader
type Loader struct {
	defaults       map[string]interface{}
	automaticEnv   bool
	envPrefix      string
	replacer       *strings.Replacer
	configType     string
	embeddedConfig []byte
}

// NewLoader creates a new Loader instance.
func NewLoader() *Loader {
	return &Loader{
		defaults:       make(map[string]interface{}, 0),
		automaticEnv:   true,
		envPrefix:      "",
		replacer:       strings.NewReplacer(".", "_"),
		configType:     "yaml",
		embeddedConfig: embeddedConfigBytes,
	}
}

// MustLoad loads the config from the config file and environment variables
func (loader *Loader) MustLoad() {
	v := viper.New()
	loader.decorateViper(v)

	if err := v.Unmarshal(Cfg); err != nil {
		panic(err)
	}
}

func (loader *Loader) decorateViper(v *viper.Viper) {
	v.SetConfigType(loader.configType)

	if "" != loader.envPrefix {
		v.SetEnvPrefix(loader.envPrefix)
	}

	for key, value := range loader.defaults {
		v.SetDefault(key, value)
	}

	if loader.automaticEnv {
		v.AutomaticEnv()
	}

	if nil != loader.replacer {
		v.SetEnvKeyReplacer(loader.replacer)
	}

	if len(loader.embeddedConfig) > 0 {
		if err := v.ReadConfig(bytes.NewReader(loader.embeddedConfig)); err != nil {
			panic(err)
		}
	}
}
