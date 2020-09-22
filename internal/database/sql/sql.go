package sql

import (
	"context"
	"database/sql"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/mrflynn/air-alert/internal/database/sql/models"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

//go:generate sqlboiler --wipe psql

// Controller is a container for the SQL backend connection.
type Controller struct {
	db *sql.DB
}

// NewController verifies the connection with the SQL backend
// and returns a new instance of a Controller.
func NewController(conn *sql.DB) (*Controller, error) {
	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return &Controller{
		db: conn,
	}, nil
}

// Shutdown closes the database connection.
func (c *Controller) Shutdown() error {
	log.Debug("attempting to shutdown sql database controller...")
	err := c.db.Close()
	log.Debug("sql database controller has shutdown")
	return err
}

// UserRequest is a container for storing details about a user from a request
// object.
type UserRequest struct {
	ID            int                   `json:"-"`
	Subscription  *webpush.Subscription `json:"subscription"`
	Longitude     float64               `json:"longitude"`
	Latitude      float64               `json:"latitude"`
	AQIThreshold  float64               `json:"threshold"`
	LastCrossover null.Time             `json:"-"`
}

func userModelToUserRequest(m *models.User) UserRequest {
	return UserRequest{
		ID: m.ID,
		Subscription: &webpush.Subscription{
			Endpoint: m.PushURL,
			Keys: webpush.Keys{
				Auth:   m.PrivateKey,
				P256dh: m.PublicKey,
			},
		},
		Longitude:     m.Longitude,
		Latitude:      m.Latitude,
		AQIThreshold:  m.Threshold,
		LastCrossover: m.LastCrossover,
	}
}

func userRequestToUserModel(u UserRequest) *models.User {
	return &models.User{
		PushURL:    u.Subscription.Endpoint,
		PrivateKey: u.Subscription.Keys.Auth,
		PublicKey:  u.Subscription.Keys.P256dh,
		Longitude:  u.Longitude,
		Latitude:   u.Latitude,
		Threshold:  u.AQIThreshold,
	}
}

// CreateUser creates a new user from a request.
func (c *Controller) CreateUser(ctx context.Context, u UserRequest) (int, error) {
	newUser := userRequestToUserModel(u)

	err := newUser.Insert(ctx, c.db, boil.Infer())
	return newUser.ID, err
}

// GetAllUsers returns a list of all users from the database.
func (c *Controller) GetAllUsers(ctx context.Context) ([]UserRequest, error) {
	users, err := models.Users().All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	requests := make([]UserRequest, 0, len(users))
	for _, u := range users {
		requests = append(requests, userModelToUserRequest(u))
	}

	return requests, nil
}

// GetUserWithID returns the user with the matching ID, if they exist.
func (c *Controller) GetUserWithID(ctx context.Context, id int) (UserRequest, error) {
	user, err := models.FindUser(ctx, c.db, id)
	if err != nil {
		return UserRequest{}, err
	}

	return userModelToUserRequest(user), nil
}

// UpdateCrossoverTime updates the last_notification column in the database for a specific user.
func (c *Controller) UpdateCrossoverTime(ctx context.Context, id int, updated time.Time) error {
	user, err := models.FindUser(ctx, c.db, id)
	if err != nil {
		return err
	}

	user.LastCrossover = null.TimeFrom(updated)
	_, err = user.Update(ctx, c.db, boil.Whitelist(models.UserColumns.LastCrossover))
	if err != nil {
		return err
	}

	return nil
}

// DeleteUser deletes a user that has a matching push url, public, and private keys.
func (c *Controller) DeleteUser(ctx context.Context, u UserRequest) error {
	_, err := models.Users(
		models.UserWhere.PushURL.EQ(u.Subscription.Endpoint),
		models.UserWhere.PrivateKey.EQ(u.Subscription.Keys.Auth),
		models.UserWhere.PublicKey.EQ(u.Subscription.Keys.P256dh),
	).DeleteAll(ctx, c.db)

	return err
}
