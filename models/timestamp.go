package models

import (
	"database/sql/driver"
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"time"
)

type Timestamp time.Time

// MarshalJSON implements json.Marshaler.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	//do your serializing here
	stamp := fmt.Sprintf("%d", time.Time(t).UnixMilli())
	return []byte(stamp), nil
}

func (t Timestamp) GetBSON() (interface{}, error) {
	return time.Time(t), nil
}

func (t *Timestamp) SetBSON(raw bson.Raw) error {
	var tm time.Time
	err := raw.Unmarshal(&tm)
	if err != nil {
		return err
	}
	*t = Timestamp(tm)
	return nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) (err error) {
	var ts int64
	ts, err = strconv.ParseInt(string(data), 10, 64)
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

func (t *Timestamp) Scan(v interface{}) error {
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
