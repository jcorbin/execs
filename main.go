package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"os"

	"github.com/jcorbin/anansi/x/platform"
)

var errInt = errors.New("interrupt")

var cfg = config{
	Platform: platform.Config{
		LogFileName: "game.log",
	},
}

func init() {
	cfg.AddFlags(flag.CommandLine)
}

func main() {
	// TODO load config from file
	flag.Parse()
	platform.MustRun(os.Stdout, func(p *platform.Platform) error {
		for {
			if err := p.Run(newGame()); platform.IsReplayDone(err) {
				continue // loop replay
			} else if err == io.EOF || err == errInt {
				return nil
			} else if err != nil {
				log.Printf("exiting due to %v", err)
				return err
			}
		}
	}, platform.FrameRate(60), cfg.Platform)
}

type config struct {
	Platform platform.Config
}

func (cfg *config) AddFlags(f *flag.FlagSet) {
	cfg.Platform.AddFlags(flag.CommandLine, "")
}
