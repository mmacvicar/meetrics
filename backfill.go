package main

import (
	_ "github.com/jinzhu/gorm/dialects/mysql"

	"github.com/chasdevs/meetrics/pkg/data"
	"github.com/chasdevs/meetrics/pkg/metrics"
	"github.com/chasdevs/meetrics/pkg/util"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.InfoLevel)
	data.Init()
}

// Main

func main() {
	computeLastDays(1000)
}

func computeLastDays(days int) {
	for i := 1; i < days+1; i++ {
		date := util.BeginningOfDay(i)
		if util.IsWeekday(date) {
			log.WithField("date", date).Info("Compiling metrics for date.")
			metrics.CompileMetrics(date)
		}
	}
}
