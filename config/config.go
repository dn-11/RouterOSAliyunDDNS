package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Update struct {
	From string
	To   string
}

type RouterOSClientConfig struct {
	Host      string `yaml:"host"`
	User      string `yaml:"user"`
	Password  string `yaml:"password"`
	Interface string `yaml:"interface"`
	EnableV4  string `yaml:"enable_v4"`
	EnableV6  string `yaml:"enable_v6"`
}

type UpdateDecl struct {
	From     string `yaml:"from"`
	LoadType string `yaml:"load_type"`
	To       string `yaml:"to"`
}

type ConfType struct {
	Interval       int                             `yaml:"interval"`
	RouterOSClient map[string]RouterOSClientConfig `yaml:"ros_client"`
	DDNSProvider   map[string]map[string]any       `yaml:"ddns_provider"`
	Updates        []UpdateDecl                    `yaml:"update"`
}

var Conf ConfType

func Parse() error {
	configData, err := os.ReadFile("./config.yaml")
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(configData, &Conf); err != nil {
		return err
	}

	// 设置默认配置
	if Conf.Interval == 0 {
		Conf.Interval = 300 // 默认5分钟
	}

	return nil
}
