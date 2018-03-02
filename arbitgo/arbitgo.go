package main

import (
	"os"

	"github.com/OopsMouse/arbitgo/models"

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
	var assetString string
	var server string

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "dryrun, dry, d",
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
			EnvVar:      "EXCHANGE_SECRET",
		},
		cli.StringFlag{
			Name:        "asset, as",
			Usage:       "main asset",
			Destination: &assetString,
			Value:       "BTC",
		},
		cli.StringFlag{
			Name:        "server",
			Usage:       "server host",
			Destination: &server,
		},
	}

	app.Action = func(c *cli.Context) error {
		if apiKey == "" || secret == "" {
			return cli.NewExitError("api key and secret is required", 0)
		}
		return nil
	}

	app.Run(os.Args)

	mainAsset := models.Asset(assetString)
	exchange := newExchange(apiKey, secret, mainAsset, dryrun)
	arbitrader := newTrader(exchange, mainAsset, &server)
	arbitrader.Run()
}

func newExchange(apikey string, secret string, mainAsset models.Asset, dryRun bool) usecase.Exchange {
	binance := infrastructure.NewBinance(
		apikey,
		secret,
	)

	if dryRun {
		balances := map[models.Asset]*models.Balance{}
		balances[mainAsset] = &models.Balance{
			Asset: mainAsset,
			Free:  0.12,
			Total: 0.12,
		}
		return infrastructure.NewExchangeStub(
			binance,
			balances,
		)
	}

	return binance
}

func newTrader(exchange usecase.Exchange, mainAsset models.Asset, server *string) *usecase.Trader {
	return usecase.NewTrader(
		exchange,
		mainAsset,
		server,
	)
}
