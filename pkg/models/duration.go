package models

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/constants"

	"github.com/fxamacker/cbor/v2"
)

const (
	nsPerNanoSecond  = int64(1)
	nsPerMicroSecond = 1000 * nsPerNanoSecond
	nsPerMilliSecond = 1000 * nsPerMicroSecond
	nsPerSecond      = 1000 * nsPerMilliSecond
	nsPerMinute      = 60 * nsPerSecond
	nsPerHour        = 60 * nsPerMinute
	nsPerDay         = 24 * nsPerHour
	nsPerWeek        = 7 * nsPerDay
	nsPerYear        = 365 * nsPerDay
)

type CustomDuration struct {
	time.Duration
}

func (d *CustomDuration) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := d.Nanoseconds()
	s := totalNS / constants.OneSecondToNanoSecond
	ns := totalNS % constants.OneSecondToNanoSecond

	return enc.Marshal(cbor.Tag{
		Number:  TagCustomDuration,
		Content: [2]int64{s, ns},
	})
}

func (d *CustomDuration) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0].(int64)
	ns := temp[1].(int64)

	*d = CustomDuration{time.Duration((float64(s) * constants.OneSecondToNanoSecond) + float64(ns))}

	return nil
}

func (d *CustomDuration) String() string {
	return FormatDuration(d.Nanoseconds())
}

func (d *CustomDuration) ToCustomDurationString() CustomDurationString {
	return CustomDurationString(d.String())
}

//------------------------------------------------------------------------------------------------------------------------------//

type CustomDurationString string

func (d *CustomDurationString) String() string {
	return string(*d)
}

func (d *CustomDurationString) ToDuration() time.Duration {
	ns, err := ParseDuration(d.String())
	if err != nil {
		panic(err)
	}

	return time.Duration(ns)
}

func (d *CustomDurationString) ToCustomDuration() CustomDuration {
	return CustomDuration{d.ToDuration()}
}

//------------------------------------------------------------------------------------------------------------------------------//

func FormatDuration(ns int64) string {
	years := ns / nsPerYear
	ns %= nsPerYear

	weeks := ns / nsPerWeek
	ns %= nsPerWeek

	days := ns / nsPerDay
	ns %= nsPerDay

	hours := ns / nsPerHour
	ns %= nsPerHour

	minutes := ns / nsPerMinute
	ns %= nsPerMinute

	seconds := ns / nsPerSecond
	ns %= nsPerSecond

	milliseconds := ns / nsPerMilliSecond
	ns %= nsPerMilliSecond

	microseconds := ns / nsPerMicroSecond
	ns %= nsPerMicroSecond

	result := ""
	if years > 0 {
		result += fmt.Sprintf("%dy", years)
	}
	if weeks > 0 {
		result += fmt.Sprintf("%dw", weeks)
	}
	if days > 0 {
		result += fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 {
		result += fmt.Sprintf("%ds", seconds)
	}
	if milliseconds > 0 {
		result += fmt.Sprintf("%dms", milliseconds)
	}
	if microseconds > 0 {
		result += fmt.Sprintf("%dµs", microseconds)
	}
	if ns > 0 {
		result += fmt.Sprintf("%dns", ns)
	}

	return result
}

func ParseDuration(duration string) (int64, error) {
	// Regular expression to match the units in the duration string
	re := regexp.MustCompile(`(\d+)([a-zµ]+)`)
	matches := re.FindAllStringSubmatch(duration, -1)

	var totalNanoseconds int64

	for _, match := range matches {
		value, _ := strconv.ParseInt(match[1], 10, 64)
		unit := match[2]

		switch unit {
		case "y":
			totalNanoseconds += value * nsPerYear
		case "w":
			totalNanoseconds += value * nsPerWeek
		case "d":
			totalNanoseconds += value * nsPerDay
		case "h":
			totalNanoseconds += value * nsPerHour
		case "m":
			totalNanoseconds += value * nsPerMinute
		case "s":
			totalNanoseconds += value * nsPerSecond
		case "ms":
			totalNanoseconds += value * nsPerMilliSecond
		case "µs", "us":
			totalNanoseconds += value * nsPerMicroSecond
		case "ns":
			totalNanoseconds += value * nsPerNanoSecond
		}
	}

	return totalNanoseconds, nil
}
