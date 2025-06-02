package config

import (
	"github.com/kelseyhightower/envconfig"
	"log"
)

type Config struct {
	TelegramBotApiToken string `required:"true" split_words:"true"`
}

func FromEnv() Config {
	var c Config
	err := envconfig.Process("", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	return c
}
