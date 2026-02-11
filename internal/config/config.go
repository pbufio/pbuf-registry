package config

import (
	"bytes"
	_ "embed"
	"strings"
	"time"

	"github.com/spf13/viper"
)

//go:embed config.yaml
var embeddedConfigBytes []byte

type Auth struct {
	Enabled bool   `mapstructure:"enabled"`
	Type    string `mapstructure:"type"`
}

type Server struct {
	HTTP struct {
		Addr    string        `mapstructure:"addr"`
		Timeout time.Duration `mapstructure:"timeout"`
		Auth    Auth          `mapstructure:"auth"`
	} `mapstructure:"http"`
	GRPC struct {
		Addr    string        `mapstructure:"addr"`
		Timeout time.Duration `mapstructure:"timeout"`
		TLS     struct {
			Enabled  bool   `mapstructure:"enabled"`
			CertFile string `mapstructure:"certFile"`
			KeyFile  string `mapstructure:"keyFile"`
		} `mapstructure:"tls"`
		Auth Auth `mapstructure:"auth"`
	} `mapstructure:"grpc"`
	Debug struct {
		Addr    string        `mapstructure:"addr"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"debug"`
}

type Config struct {
	Server Server `mapstructure:"server"`

	Data struct {
		Database struct {
			DSN string `mapstructure:"dsn"`
		} `mapstructure:"database"`
	} `mapstructure:"data"`

	Daemons struct {
		Compaction struct {
			CronSchedule string `mapstructure:"cron"`
		} `mapstructure:"compaction"`
		ProtoParsing struct {
			CronSchedule string `mapstructure:"cron"`
		} `mapstructure:"protoparsing"`
		DriftDetection struct {
			CronSchedule string `mapstructure:"cron"`
		} `mapstructure:"driftdetection"`
	} `mapstructure:"daemons"`
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

	if loader.envPrefix != "" {
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
