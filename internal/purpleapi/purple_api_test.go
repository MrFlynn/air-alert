package purpleapi

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Example data taken from Purple Air's API.
var (
	expectedResponse = []Response{
		{
			ID:          14633,
			Location:    Outside,
			Latitude:    37.275561,
			LastUpdated: 1598334840,
			Longitude:   -121.964134,
			PM25:        24.68,
		},
		{
			ID:          14634,
			ParentID:    14633,
			Location:    Unknown,
			LastUpdated: 1598334840,
			Latitude:    37.275561,
			Longitude:   -121.964134,
			PM25:        25.12,
		},
	}
	testData = `{
		"mapVersion":"0.17",
		"baseVersion":"7",
		"mapVersionString":"",
		"results":[
			{
				"ID":14633,
				"Label":" Hazelwood canary ",
				"DEVICE_LOCATIONTYPE":"outside",
				"THINGSPEAK_PRIMARY_ID":"559921",
				"THINGSPEAK_PRIMARY_ID_READ_KEY":"CU4BQZZ38WO5UJ4C",
				"THINGSPEAK_SECONDARY_ID":"559922",
				"THINGSPEAK_SECONDARY_ID_READ_KEY":"D0YNZ1LM59LL49VQ",
				"Lat":37.275561,
				"Lon":-121.964134,
				"PM2_5Value":"24.68",
				"LastSeen":1598334840,
				"Type":"PMS5003+PMS5003+BME280",
				"Hidden":"false",
				"isOwner":0,
				"humidity":"50",
				"temp_f":"79",
				"pressure":"1003.86",
				"AGE":0,
				"Stats":"{\"v\":24.68,\"v1\":23.86,\"v2\":23.55,\"v3\":28.07,\"v4\":59.12,\"v5\":61.21,\"v6\":25.77,\"pm\":24.68,\"lastModified\":1598334840227,\"timeSinceModified\":119989}"
			},
			{
				"ID":14634,
				"ParentID":14633,
				"Label":" Hazelwood canary  B",
				"THINGSPEAK_PRIMARY_ID":"559923",
				"THINGSPEAK_PRIMARY_ID_READ_KEY":"DULWDNCI9M6PCIPC",
				"THINGSPEAK_SECONDARY_ID":"559924",
				"THINGSPEAK_SECONDARY_ID_READ_KEY":"EY2CNMYRUZHDW1AL",
				"Lat":37.275561,
				"Lon":-121.964134,
				"PM2_5Value":"25.12",
				"LastSeen":1598334840,
				"Hidden":"false",
				"Flag":1,
				"isOwner":0,
				"AGE":0,
				"Stats":"{\"v\":25.12,\"v1\":23.7,\"v2\":23.24,\"v3\":27.63,\"v4\":58.6,\"v5\":59.65,\"v6\":21.82,\"pm\":25.12,\"lastModified\":1598334840228,\"timeSinceModified\":119990}"
			}
		]
	}`
)

func TestDecode(t *testing.T) {
	reader := ioutil.NopCloser(strings.NewReader(testData))

	resp, err := decode(reader)
	if err != nil {
		t.Errorf("Got error during decode: %s", err)
	}

	if !cmp.Equal(resp, expectedResponse) {
		t.Errorf("Expected: %+v\nGot: %+v\n", expectedResponse, resp)
	}
}
