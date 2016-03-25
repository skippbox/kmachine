package commands

import (
	"github.com/docker/machine/libmachine/log"

	"github.com/docker/machine/cli"
)

func cmdStart(c *cli.Context) error {
	if err := runActionWithContext("start", c); err != nil {
		return err
	}

	log.Info("Started machines may have new IP addresses. You may need to re-run the `kmachine env` command.")

	return nil
}
