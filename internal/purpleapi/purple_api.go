package purpleapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const apiURL = "https://www.purpleair.com/json"

// Enable encoding/json compat.
var json = jsoniter.ConfigCompatibleWithStandardLibrary
var limiter = rate.NewLimiter(rate.Every(10*time.Second), 1)

// Sensor location enum.
type Location int

const (
	Unknown Location = iota
	Outside
	Inside
)

func toLocation(s string) Location {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "outside":
		return Outside
	case "inside":
		return Inside
	default:
		return Unknown
	}
}

type outerResponse struct {
	Results []Response `json:"results"`
}

// Response encodes the relevant data from the Purple Air api.
type Response struct {
	ID        int      `json:"ID" db:"primary_key"`
	ParentID  int      `json:"ParentID,omitempty" db:"append_only"`
	Location  Location `json:"DEVICE_LOCATIONTYPE,omitempty" db:"ignore"`
	Latitude  float64  `json:"Lat" db:"value,geo"`
	Longitude float64  `json:"Lon" db:"value,geo"`
	PM25      float32  `json:"PM2_5Value,string" db:"value,quality"`
}

// UnmarshalJSON implements a custom unmarshaller for the Response type.
// This is to transform the location string into the proper enum type.
func (r *Response) UnmarshalJSON(data []byte) error {
	type Alias Response
	aux := &struct {
		Location string `json:"DEVICE_LOCATIONTYPE,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.Location = toLocation(aux.Location)

	return nil
}

func decode(r io.ReadCloser) ([]Response, error) {
	apiData := &outerResponse{}

	err := json.NewDecoder(r).Decode(apiData)
	if err != nil {
		return []Response{}, err
	}

	return apiData.Results, nil
}

// Get grabs data from Purple Air's API and returns a list of current measurements.
func Get(ctx context.Context) ([]Response, error) {
	var err error
	var resp *http.Response

	for i := 0; i < 5; i++ {
		err = limiter.Wait(ctx)
		if err != nil {
			return []Response{}, err
		}

		resp, err = http.Get(apiURL)
		if err != nil {
			return []Response{}, err
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		log.Debugf(`hit purple air api rate limit. retrying in %.1f seconds`, limiter.Limit())
	}

	if resp.StatusCode != http.StatusOK {
		return []Response{}, errors.New("failed to get sensor data due to rate limiting")
	}

	defer resp.Body.Close()
	return decode(resp.Body)
}
