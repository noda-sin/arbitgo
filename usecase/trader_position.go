package usecase

import (
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) addPosition(asset string) {
	trader.positions.Append(asset)
	trader.printPositions()
}

func (trader *Trader) delPosition(asset string) {
	trader.positions.Remove(asset)
	trader.printPositions()
}

func (trader *Trader) isRunningPosition(asset string) bool {
	return trader.positions.Include(asset)
}

func (trader *Trader) printPositions() {
	log.Info("Positions : ", trader.positions.ToSlice())
}
