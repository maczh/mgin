package utils

import (
	"fmt"
	"github.com/araddon/dateparse"
	"math"
	"strings"
	"time"
)

func ToDateTimeString(date time.Time) string {
	return date.Format("2006-01-02 15:04:05")
}

func ToDateString(date time.Time) string {
	return date.Format("2006-01-02")
}

func ToTimeString(date time.Time) string {
	return date.Format("15:04:05")
}

func TimeSubDays(t1, t2 time.Time) int {
	if t1.Location().String() != t2.Location().String() {
		return -1
	}
	hours := t1.Sub(t2).Hours()

	if hours <= 0 {
		return -1
	}
	// sub hours less than 24
	if hours < 24 {
		// may same day
		t1y, t1m, t1d := t1.Date()
		t2y, t2m, t2d := t2.Date()
		isSameDay := (t1y == t2y && t1m == t2m && t1d == t2d)

		if isSameDay {

			return 0
		} else {
			return 1
		}

	} else { // equal or more than 24

		if (hours/24)-float64(int(hours/24)) == 0 { // just 24's times
			return int(hours / 24)
		} else { // more than 24 hours
			return int(hours/24) + 1
		}
	}

}

func GormTimeFormat(t string) string {
	return strings.ReplaceAll(strings.ReplaceAll(t, "T", " "), "+08:00", "")
}

// Format time.Time struct to string
// MM - month - 01
// M - month - 1, single bit
// DD - day - 02
// D - day 2
// YYYY - year - 2006
// YY - year - 06
// HH - 24 hours - 03
// H - 24 hours - 3
// hh - 12 hours - 03
// h - 12 hours - 3
// mm - minute - 04
// m - minute - 4
// ss - second - 05
// s - second = 5
func DateFormat(t time.Time, format string) string {
	res := strings.Replace(format, "MMMM", "January", -1)
	res = strings.Replace(res, "MMM", "Jan", -1)
	res = strings.Replace(res, "MM", "01", -1)
	res = strings.Replace(res, "M", "1", -1)
	res = strings.Replace(res, "dddd", "Monday", -1)
	res = strings.Replace(res, "ddd", "Mon", -1)
	res = strings.Replace(res, "dd", "02", -1)
	res = strings.Replace(res, "d", "2", -1)
	res = strings.Replace(res, "yyyy", "2006", -1)
	res = strings.Replace(res, "yy", "06", -1)
	res = strings.Replace(res, "hh", "15", -1)
	res = strings.Replace(res, "HH", "03", -1)
	res = strings.Replace(res, "H", "3", -1)
	res = strings.Replace(res, "mm", "04", -1)
	res = strings.Replace(res, "m", "4", -1)
	res = strings.Replace(res, "ss", "05", -1)
	res = strings.Replace(res, "s", "5", -1)
	res = strings.Replace(res, "tt", "PM", -1)
	res = strings.Replace(res, "ZZZ", "MST", -1)
	res = strings.Replace(res, "Z", "Z07:00", -1)
	return t.Format(res)
}

func ConvertDateFormat(timeStr string, format string) string {
	t, err := dateparse.ParseLocal(timeStr)
	if err != nil {
		fmt.Println(err.Error())
		return ""
	}
	return DateFormat(t, format)
}

func GetNowDateTime() string {
	var cstZone = time.FixedZone("CST", 8*3600)
	return time.Now().In(cstZone).Format("2006-01-02 15:04:05")
}

func GetDate() string {
	var cstZone = time.FixedZone("CST", 8*3600)
	return time.Now().In(cstZone).Format("2006-01-02")
}

//判断时间是当年的第几周
func WeekByDate(t time.Time) int {
	yearDay := t.YearDay()
	yearFirstDay := t.AddDate(0, 0, -yearDay+1)
	firstDayInWeek := int(yearFirstDay.Weekday())

	//今年第一周有几天
	firstWeekDays := 1
	if firstDayInWeek != 0 {
		firstWeekDays = 7 - firstDayInWeek + 1
	}
	var week int
	if yearDay <= firstWeekDays {
		week = 1
	} else {
		week = (yearDay-firstWeekDays)/7 + 2
	}
	return week
}

type WeekDate struct {
	WeekTh    int
	StartDate string
	EndDate   string
}

// 将开始时间和结束时间分割为周为单位
func GroupByWeekDate(startTime, endTime time.Time) []WeekDate {
	weekDate := make([]WeekDate, 0)
	diffDuration := endTime.Sub(startTime)
	days := int(math.Ceil(float64(diffDuration/(time.Hour*24)))) + 1

	currentWeekDate := WeekDate{}
	currentWeekDate.WeekTh = WeekByDate(endTime)
	currentWeekDate.EndDate = DateFormat(endTime, "yyyy-MM-dd")
	currentWeekDay := int(endTime.Weekday())
	if currentWeekDay == 0 {
		currentWeekDay = 7
	}
	startDate := endTime.AddDate(0, 0, -currentWeekDay+1)
	currentWeekDate.StartDate = DateFormat(startDate, "yyyy-MM-dd")
	nextWeekEndTime := startDate
	weekDate = append(weekDate, currentWeekDate)

	for i := 0; i < (days-currentWeekDay)/7; i++ {
		weekData := WeekDate{}
		weekData.EndDate = DateFormat(nextWeekEndTime, "yyyy-MM-dd")
		startDate = nextWeekEndTime.AddDate(0, 0, -7)
		weekData.StartDate = DateFormat(startDate, "yyyy-MM-dd")
		weekData.WeekTh = WeekByDate(startDate)
		nextWeekEndTime = startDate
		weekDate = append(weekDate, weekData)
	}

	if lastDays := (days - currentWeekDay) % 7; lastDays > 0 {
		lastData := WeekDate{}
		lastData.EndDate = DateFormat(nextWeekEndTime, "yyyy-MM-dd")
		startDate = nextWeekEndTime.AddDate(0, 0, -lastDays)
		lastData.StartDate = DateFormat(startDate, "yyyy-MM-dd")
		lastData.WeekTh = WeekByDate(startDate)
		weekDate = append(weekDate, lastData)
	}

	return weekDate
}

// WaitNextMinute 下一分钟, 对齐时间, 0 秒
func WaitNextMinute() {
	now := time.Now()
	<-time.After(Get0Second(now.Add(time.Minute)).Sub(now))
}

// Get0Hour 当天 0 点
func Get0Hour(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Get0Yesterday 昨天 0 点
func Get0Yesterday(t time.Time) time.Time {
	return Get0Hour(t.AddDate(0, 0, -1))
}

// Get0Tomorrow 昨天 0 点
func Get0Tomorrow(t time.Time) time.Time {
	return Get0Hour(t.AddDate(0, 0, 1))
}

// Get0Minute 0 分
func Get0Minute(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
}

// Get0Second 0 秒
func Get0Second(t time.Time) time.Time {
	t.Truncate(time.Minute)
	y, m, d := t.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, t.Location())
}

// Get0Week 本周一 0 点
func Get0Week(t time.Time) time.Time {
	offset := int(time.Monday - t.Weekday())
	if offset > 0 {
		offset = -6
	}

	return Get0Hour(t).AddDate(0, 0, offset)
}

// Get0LastWeek 上周一 0 点
func Get0LastWeek(t time.Time) time.Time {
	return Get0Week(t.AddDate(0, 0, -7))
}

// Get0NextWeek 下周一 0 点
func Get0NextWeek(t time.Time) time.Time {
	return Get0Week(t.AddDate(0, 0, 7))
}

// Get0Month 当月第一天 0 点
func Get0Month(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// Get0LastMonth 上月第一天 0 点
func Get0LastMonth(t time.Time) time.Time {
	return Get0Month(t.AddDate(0, -1, 0))
}

// Get0NextMonth 下月第一天 0 点
func Get0NextMonth(t time.Time) time.Time {
	return Get0Month(t.AddDate(0, 1, 0))
}

// GetMonthDays 当月天数
func GetMonthDays(t time.Time) int {
	return int(Get0NextMonth(t).Sub(Get0Month(t)).Hours() / 24)
}
