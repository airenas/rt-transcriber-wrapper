package utils

import (
	"fmt"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
)

func MeasureTime(name string, start time.Time) {
	elapsed := time.Since(start)
	goapp.Log.Info().Str("elapsed", fmt.Sprintf("%v", elapsed)).Str("func", name).Msg("time")
}
