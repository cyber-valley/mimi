package main

import (
	"flag"
	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	glog.Infoln("App is starting")
}
