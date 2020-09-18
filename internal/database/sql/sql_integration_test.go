// +build integration

package sql

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	_ "github.com/lib/pq"
)

var (
	mtx        sync.Mutex
	conn       *sql.DB
	controller *Controller

	testUser = UserRequest{
		Subscription: &webpush.Subscription{
			Endpoint: "http://example.com",
			Keys: webpush.Keys{
				Auth:   "priv_key",
				P256dh: "pub_key",
			},
		},
		Longitude:    0.0,
		Latitude:     0.0,
		AQIThreshold: 50.0,
	}
)

func init() {
	var err error

	conn, err = sql.Open("postgres", fmt.Sprintf(
		"dbname=%s user=%s password=%s sslmode=disable", os.Getenv("PGDBNAME"), os.Getenv("PGUSER"), os.Getenv("PGPASS"),
	))
	if err != nil {
		fmt.Printf("could not init database connection: %s\n", err)
		os.Exit(1)
	}

	data, err := ioutil.ReadFile(os.Getenv("DBSPEC"))
	if err != nil {
		fmt.Printf("could not open file %s: %s\n", os.Getenv("DBSPEC"), err)
		os.Exit(1)
	}

	_, err = conn.Query(string(data))
	if err != nil {
		fmt.Printf("could not init database: %s\n", err)
		os.Exit(1)
	}

	controller, err = NewController(conn)
	if err != nil {
		fmt.Printf("could not init database controller: %s\n", err)
		os.Exit(1)
	}
}

func runSeq() func() {
	mtx.Lock()
	return func() {
		mtx.Unlock()
	}
}

func TestCreateUser(t *testing.T) {
	defer runSeq()()

	createdID, err := controller.CreateUser(context.Background(), testUser)
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	rows, err := conn.Query("select * from users")
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	defer rows.Close()

	var (
		id         int
		pushURL    string
		privateKey string
		publicKey  string
		longitude  float64
		latitude   float64
		threshold  float64
	)

	if !rows.Next() {
		t.Error("expected 1 row")
	}

	err = rows.Scan(&id, &pushURL, &privateKey, &publicKey, &longitude, &latitude, &threshold)
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	if rows.NextResultSet() {
		t.Error("got more than 1 result when only 1 was expected")
	}

	if id != createdID {
		t.Errorf("expected id to be %d, got %d", createdID, id)
	}

	if pushURL != testUser.Subscription.Endpoint {
		t.Errorf("expected push url to be %s, got %s", pushURL, testUser.Subscription.Endpoint)
	}

	if privateKey != testUser.Subscription.Keys.Auth {
		t.Errorf("expected private key to be %s, got %s", privateKey, testUser.Subscription.Keys.Auth)
	}

	if publicKey != testUser.Subscription.Keys.P256dh {
		t.Errorf("expected public key to be %s, got %s", publicKey, testUser.Subscription.Keys.P256dh)
	}

	if longitude != testUser.Longitude {
		t.Errorf("expected longitude to be %f, got %f", longitude, testUser.Longitude)
	}

	if latitude != testUser.Latitude {
		t.Errorf("expected latitude to be %f, got %f", latitude, testUser.Latitude)
	}

	if threshold != testUser.AQIThreshold {
		t.Errorf("expected threshold to be %f, got %f", threshold, testUser.AQIThreshold)
	}
}

func TestGetAllUsers(t *testing.T) {
	defer runSeq()()

	users, err := controller.GetAllUsers(context.Background())
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	if !cmp.Equal(users, []UserRequest{testUser}, cmpopts.IgnoreFields(UserRequest{}, "ID")) {
		t.Errorf("expected %#v\ngot %#v", []UserRequest{testUser}, users)
	}
}

func TestGetUserWithID(t *testing.T) {
	defer runSeq()()

	user, err := controller.GetUserWithID(context.Background(), 1)
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	if !cmp.Equal(user, testUser, cmpopts.IgnoreFields(UserRequest{}, "ID")) {
		t.Errorf("expected %#v\ngot %#v", testUser, user)
	}
}

func TestGetUserWithInvalidID(t *testing.T) {
	defer runSeq()()

	_, err := controller.GetUserWithID(context.Background(), 100)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDeleteUser(t *testing.T) {
	defer runSeq()()

	createdID, err := controller.CreateUser(context.Background(), UserRequest{
		Subscription: &webpush.Subscription{
			Endpoint: "http://example.net",
			Keys: webpush.Keys{
				Auth:   "priv_key",
				P256dh: "pub_key",
			},
		},
		Longitude:    1.0,
		Latitude:     1.0,
		AQIThreshold: 60.0,
	})
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	err = controller.DeleteUser(context.Background(), testUser)
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	rows, err := conn.Query("select id from users")
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	var id int

	if !rows.Next() {
		t.Error("expected 1 row")
	}

	err = rows.Scan(&id)
	if err != nil {
		t.Errorf("got unexepected error: %s", err)
	}

	if rows.NextResultSet() {
		t.Error("got more than 1 result when only 1 was expected")
	}

	if id != createdID {
		t.Errorf("expected id to be %d, got %d", createdID, id)
	}
}
