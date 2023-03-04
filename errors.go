package OpenWeatherAPI

import (
	"fmt"
	"time"
)

var (
	LimitErr       = fmt.Errorf("request limit expired")
	ParseErr       = fmt.Errorf("parse response error")
	ExcludeTempErr = fmt.Errorf("response has not temperature")
	CancelErr      = fmt.Errorf("cancel request")
)

type ApiErr struct {
	Status   int
	Date     time.Time
	Location Coordinates
	Err      error
}

func (a ApiErr) Error() string {
	return fmt.Sprintf("[%d] - %s \n\r", a.Status, a.Err)
}

func (a ApiErr) Unwrap() error {
	return a.Err
}
