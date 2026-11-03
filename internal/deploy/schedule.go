package deploy

import (
	"errors"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

// A minute-resolution five-field cron parser. Calendar iteration uses UTC minutes
// so daylight-saving gaps and repeated local times follow the selected timezone.
func NextSchedule(expression, zone string, after time.Time) (time.Time, error) {
	if len(expression) > 256 || len(zone) > 128 {
		return time.Time{}, errors.New("Schedule is too long")
	}
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return time.Time{}, errors.New("Choose a valid IANA timezone")
	}
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return time.Time{}, errors.New("Use a five-field cron schedule")
	}
	limits := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	allowed := make([]map[int]bool, 5)
	for i, f := range fields {
		allowed[i] = map[int]bool{}
		for _, part := range strings.Split(f, ",") {
			base, stepText, hasStep := strings.Cut(part, "/")
			step := 1
			if hasStep {
				step, e = strconv.Atoi(stepText)
				if e != nil || step < 1 || step > limits[i][1]+1 {
					return time.Time{}, errors.New("Invalid cron step")
				}
			}
			lo, hi := limits[i][0], limits[i][1]
			if base != "*" {
				left, right, isRange := strings.Cut(base, "-")
				lo, e = strconv.Atoi(left)
				if e != nil {
					return time.Time{}, errors.New("Invalid cron value")
				}
				hi = lo
				if isRange {
					hi, e = strconv.Atoi(right)
					if e != nil {
						return time.Time{}, errors.New("Invalid cron range")
					}
				} else if hasStep {
					hi = limits[i][1]
				}
			}
			if lo < limits[i][0] || hi > limits[i][1] || lo > hi {
				return time.Time{}, errors.New("Cron value is outside its range")
			}
			for n := lo; n <= hi; n += step {
				if i == 4 && n == 7 {
					allowed[i][0] = true
				} else {
					allowed[i][n] = true
				}
			}
		}
	}
	t := after.UTC().Truncate(time.Minute).Add(time.Minute)
	end := t.AddDate(5, 0, 0)
	for t.Before(end) {
		local := t.In(loc)
		day := allowed[2][local.Day()]
		week := allowed[4][int(local.Weekday())]
		if fields[2] != "*" && fields[4] != "*" {
			day = day || week
		} else {
			day = day && week
		}
		if allowed[0][local.Minute()] && allowed[1][local.Hour()] && day && allowed[3][int(local.Month())] {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, errors.New("Schedule has no occurrence in the next five years")
}
