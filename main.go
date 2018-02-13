package main

import (
	"os"

	"github.com/OopsMouse/arbitgo/common"
	"github.com/OopsMouse/arbitgo/infrastructure"
	"github.com/OopsMouse/arbitgo/usecase"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "arbitgo"
	app.Usage = "A Bot for arbit rage with one exchange, multi currency"
	app.Version = "0.0.1"

	var dryrun bool
	var apiKey string
	var secret string
	var asset string

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "dryrun, d",
			Usage:       "dry run mode",
			Destination: &dryrun,
		},
		cli.StringFlag{
			Name:        "apikey, a",
			Usage:       "api key of exchange",
			Destination: &apiKey,
			EnvVar:      "EXCHANGE_APIKEY",
		},
		cli.StringFlag{
			Name:        "secret, s",
			Usage:       "secret of exchange",
			Destination: &secret,
			EnvVar:      "EXCAHNGE_SECRET",
		},
		cli.StringFlag{
			Name:        "asset, as",
			Usage:       "start asset",
			Destination: &asset,
		},
	}

	app.Action = func(c *cli.Context) error {
		if apiKey == "" || secret == "" {
			return cli.NewExitError("api key and secret is required", 0)
		}
		if asset == "" {
			asset = common.BTC
		}
		var exchange usecase.Exchange
		if dryrun {
			exchange = infrastructure.NewExchangeStub(
				apiKey,
				secret,
				asset,
			)
		} else {
			exchange = infrastructure.NewExchange(
				apiKey,
				secret,
			)
		}
		anlyzr := usecase.NewMarketAnalyzer()
		trader := usecase.NewArbitrader(
			exchange,
			anlyzr,
			asset,
		)
		trader.Run()
		return nil
	}

	app.Run(os.Args)
}
