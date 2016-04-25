package main

import (
	"github.com/docker/machine/drivers/aurora"
	"github.com/docker/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(aurora.NewDriver("", ""))
}
