package models

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Timestamp time.Time

// MarshalJSON implements json.Marshaler.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	//do your serializing here
	stamp := fmt.Sprintf("%d", time.Time(t).UnixMilli())
	return []byte(stamp), nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) (err error) {
	s := strings.TrimSpace(string(data))
	// 兼容带引号的字符串与 null
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "null" || s == "" {
		return nil
	}
	var ts int64
	ts, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	theTime := time.UnixMilli(ts)
	*t = Timestamp(theTime)
	return nil
}

func (t Timestamp) Value() (driver.Value, error) {
	return time.Time(t), nil
}

func (t *Timestamp) Scan(v any) error {
	value, ok := v.(time.Time)
	if ok {
		*t = Timestamp(value)
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}

func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

func NewTimestamp(t time.Time) Timestamp {
	return Timestamp(t)
}
