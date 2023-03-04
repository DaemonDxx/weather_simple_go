package OpenWeatherAPI

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/valyala/fasthttp"
	"math"
	"sync"
	"time"
)

const host = "api.openweathermap.org"
const max_connections = 10

type ApiError struct {
	err error
}

type OpenWeatherAPI struct {
	http        *fasthttp.PipelineClient
	token       string
	measurement int
}

func (o *OpenWeatherAPI) GetTempByTime(ctx context.Context, date time.Time, location *Coordinates) (float64, error) {
	resultCh := make(chan float64)
	errCh := make(chan error)

	go func() {
		defer close(errCh)
		defer close(resultCh)

		req := fasthttp.AcquireRequest()
		url := fmt.Sprintf("https://api.openweathermap.org//data/3.0/onecall/timemachine?lat=%f&lon=%f&dt=%d&appid=%s&units=metric", location.Lat, location.Lon, date.Unix(), o.token)
		req.Header.SetRequestURI(url)
		req.Header.SetMethod(fasthttp.MethodGet)
		res := fasthttp.AcquireResponse()

		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(res)
		err := o.http.DoTimeout(req, res, 5*time.Second)

		if err != nil {
			errCh <- fmt.Errorf("send request error: %s", err)
			return
		}

		if status := res.StatusCode(); status != 200 {
			err := &ApiErr{
				Status:   status,
				Date:     date,
				Location: *location,
				Err:      nil,
			}

			switch status {
			case 429:
				err.Err = LimitErr
			default:
				err.Err = errors.New(string(res.Body()))
			}
			errCh <- err
			return
		}

		var r ResponseAPI
		err = json.Unmarshal(res.Body(), &r)

		if err != nil {
			errCh <- ParseErr
			return
		}

		if len(r.Data) == 0 {
			errCh <- ExcludeTempErr
			return
		}

		resultCh <- r.Data[0].Temp
	}()

	select {
	case temp := <-resultCh:
		return temp, nil
	case err := <-errCh:
		return 0, err
	case <-ctx.Done():
		return 0, CancelErr
	}
}

func (o *OpenWeatherAPI) GetDailyTemp(ctx context.Context, date time.Time, location *Coordinates) (float64, error) {
	resultCh := make(chan float64, o.measurement)
	errCh := make(chan error)
	sum := 0.0

	wg := sync.WaitGroup{}

	go func() {
		for i := 0; i < o.measurement; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				c := int(math.Ceil(180 + 24*60*float64(i)/float64(o.measurement)))
				dateOffset := date.Add(time.Duration(c) * time.Minute)

				t, err := o.GetTempByTime(ctx, dateOffset, location)
				if err != nil {
					errCh <- err
					return
				}

				resultCh <- t
			}(i)
		}
		wg.Wait()
		close(resultCh)
		<-time.After(1 * time.Second)
		close(errCh)
	}()

	for {
		select {
		case t, ok := <-resultCh:
			if !ok {
				return sum / float64(o.measurement), nil
			}
			sum += t
		case err := <-errCh:
			return 0, err
		}
	}
}

func New(cfg *Config) *OpenWeatherAPI {
	http := fasthttp.PipelineClient{
		Addr:     host,
		Name:     "API",
		MaxConns: max_connections,
	}

	return &OpenWeatherAPI{
		http:        &http,
		token:       cfg.Token,
		measurement: cfg.CountMeasurement,
	}
}
