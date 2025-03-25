package viperConf

import (
	"github.com/spf13/viper"
)

type Option func(*conf)

type conf struct {
	configDirPath  string
	configFileType string
	configFilename string
}

var c = &conf{
	configDirPath:  "../../configs",
	configFileType: "yaml",
	configFilename: "common",
}

func apply(opts ...Option) *conf {
	newConf := c
	for _, opt := range opts {
		opt(newConf)
	}
	return newConf
}

func WithDirPath(dirPath string) Option {
	return func(c *conf) {
		c.configDirPath = dirPath
	}
}

func WithFileType(fileType string) Option {
	return func(c *conf) {
		c.configFileType = fileType
	}
}

func WithFileName(filename string) Option {
	return func(c *conf) {
		c.configFilename = filename
	}
}

func Init(opts ...Option) {
	cur := apply(opts...)

	viper.SetConfigType(cur.configFileType)
	viper.AddConfigPath(cur.configDirPath)
	viper.SetConfigName(cur.configFilename)
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}
