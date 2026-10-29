package schedule

import (
	"math"
	"time"
)

const (
	PolarDay   = "day"   // midnight sun: no sunrise or sunset
	PolarNight = "night" // the sun never rises
)

// Sun is the sunrise and sunset for one day (as instants; show them in the
// home's time zone). Polar is set instead when there is no sunrise/sunset.
type Sun struct {
	Rise, Set time.Time
	Polar     string
}

// SunOn computes sunrise and sunset for the calendar day of `day` (in its own
// time zone) at lat/lon, with NOAA's approximation (about ±1 minute; the sun's
// upper edge with normal refraction, zenith 90.833°).
func SunOn(day time.Time, lat, lon float64) Sun {
	y, m, d := day.Date()
	loc := day.Location()
	// solar noon of that local day, in UTC, as the reference instant
	noon := time.Date(y, m, d, 12, 0, 0, 0, loc).UTC()
	doy := float64(noon.YearDay())
	daysInYear := 365.0
	if isLeap(noon.Year()) {
		daysInYear = 366
	}
	g := 2 * math.Pi / daysInYear * (doy - 1 + (float64(noon.Hour())-12)/24)
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(g) - 0.032077*math.Sin(g) -
		0.014615*math.Cos(2*g) - 0.040849*math.Sin(2*g))
	decl := 0.006918 - 0.399912*math.Cos(g) + 0.070257*math.Sin(g) - 0.006758*math.Cos(2*g) +
		0.000907*math.Sin(2*g) - 0.002697*math.Cos(3*g) + 0.00148*math.Sin(3*g)

	latR := lat * math.Pi / 180
	cosHA := math.Cos(90.833*math.Pi/180)/(math.Cos(latR)*math.Cos(decl)) - math.Tan(latR)*math.Tan(decl)
	switch {
	case cosHA > 1:
		return Sun{Polar: PolarNight}
	case cosHA < -1:
		return Sun{Polar: PolarDay}
	}
	ha := math.Acos(cosHA) * 180 / math.Pi

	midnightUTC := time.Date(noon.Year(), noon.Month(), noon.Day(), 0, 0, 0, 0, time.UTC)
	at := func(minutes float64) time.Time {
		return midnightUTC.Add(time.Duration(minutes * float64(time.Minute))).Round(time.Second)
	}
	return Sun{
		Rise: at(720 - 4*(lon+ha) - eqTime),
		Set:  at(720 - 4*(lon-ha) - eqTime),
	}
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }
