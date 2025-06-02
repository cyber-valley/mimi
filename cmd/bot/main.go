package main

import (
	"flag"
	"github.com/golang/glog"
	"mimi/internal/bot"
	"mimi/internal/config"
)

func main() {
	flag.Parse()
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	glog.Infoln("App is starting")
	config := config.FromEnv()
	bot.Start(config.TelegramBotApiToken)
}
