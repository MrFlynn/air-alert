package notifications

import (
	"bytes"
	"context"
	"net/http"
	"sync"

	"github.com/SherClockHolmes/webpush-go"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/database/redis"
	"github.com/mrflynn/air-alert/internal/database/sql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Sender is a notification sending system for web push notifications.
type Sender struct {
	Threads uint
	Group   string

	pubKey     string
	privKey    string
	subscriber string

	datastore *redis.Controller
	users     *sql.Controller

	stop chan bool
	ack  chan bool
}

// NewSender creates a new notification sender.
func NewSender(datastore *redis.Controller, users *sql.Controller) *Sender {
	return &Sender{
		Threads:    viper.GetUint("web.notifications.threads"),
		Group:      viper.GetString("web.notifications.group"),
		pubKey:     viper.GetString("web.notifications.public_key"),
		privKey:    viper.GetString("web.notifications.private_key"),
		subscriber: viper.GetString("web.notifications.admin_mail"),
		datastore:  datastore,
		users:      users,
		stop:       make(chan bool),
		ack:        make(chan bool),
	}
}

func (s *Sender) dispatch() {
	consumer := utils.CreateRandomString(16)
	ctx := context.Background()

	for {
		ok := true

		notifications, err := s.datastore.NotificationConsumerRead(ctx, s.Group, consumer, 1)
		if err != nil {
			log.Errorf("notification consumer read error: %s", err)
			ok = false
		} else if notifications == nil {
			// If we got nothing, then we should resume from the top.
			ok = false
		}

		if ok {
			for _, n := range notifications {
				user, err := s.users.GetUserWithID(ctx, n.UID)
				if err != nil {
					log.Errorf("could not get user %d from database", n.UID)
					continue
				}

				msg := createNotificationText(n)
				resp, err := webpush.SendNotification(msg, user.Subscription, &webpush.Options{
					Subscriber:      s.subscriber,
					TTL:             10,
					VAPIDPublicKey:  s.pubKey,
					VAPIDPrivateKey: s.privKey,
				})

				if err != nil {
					log.Errorf("got error from web push delivery service: %s", err)
				} else if resp.StatusCode != http.StatusOK {
					log.Error("got non-200 response from push service")
				} else {
					s.datastore.ACKNotifications(ctx, s.Group, n)
				}
			}
		}

		select {
		case <-s.stop:
			s.ack <- true
			return
		default:
			continue
		}
	}
}

// Run starts all notification delivery threads.
func (s *Sender) Run() {
	var i uint

	s.datastore.CreateConsumerGroup(context.Background(), s.Group)

	for ; i < s.Threads; i++ {
		go s.dispatch()
	}
}

// Shutdown sends a shutdown signal to all sender threads.
func (s *Sender) Shutdown() {
	var (
		wg sync.WaitGroup
		i  uint
	)

	for ; i < s.Threads; i++ {
		go func() {
			wg.Add(1)
			s.stop <- true
			// Wait for ack.
			<-s.ack
			wg.Done()
		}()
	}

	wg.Wait()
}

func createNotificationText(n redis.NotificationStream) []byte {
	msgBuff := bytes.NewBufferString("The AQI is ")

	msgBuff.WriteString(decimal.NewFromFloat(n.AQI).Round(1).String())
	msgBuff.WriteString(". ")

	if n.Forecast == redis.AQIIncreasing {
		msgBuff.WriteString("Time to go inside.")
	} else {
		msgBuff.WriteString("Time to get some fresh air!")
	}

	return msgBuff.Bytes()
}
