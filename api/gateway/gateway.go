package gateway

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/PylonSchema/server/auth"
	"github.com/PylonSchema/server/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	pingTick    = 10 * time.Second
	pongTimeout = (pingTick * 19) / 10
)

type Database interface {
	GetChannelsByUserUUID(uuid uuid.UUID) (*[]model.Channel, error)
}

type Gateway struct {
	Upgrader websocket.Upgrader
	m        *sync.RWMutex
	channels map[uint][]*Client
	JwtAuth  *auth.JwtAuth
	db       Database
}

func New(jwtAuth *auth.JwtAuth) *Gateway {
	return &Gateway{
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool { // origin check for dev, allow all origin
				return true
			},
		},
		JwtAuth:  jwtAuth,
		channels: make(map[uint][]*Client),
		m:        new(sync.RWMutex),
	}
}

func (g *Gateway) OpenGateway(c *gin.Context) {
	conn, err := g.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message":    "internal server error",
			"trace code": "create upgrade connection",
			"error":      err,
		})
		return
	}
	client := &Client{
		conn:         conn,
		gatewayPipe:  g,
		writeChannel: make(chan *Message),
		username:     "",
		uuid:         uuid.UUID{},
	}

	go client.readHandler(pongTimeout)
	go client.writeHandler(pingTick)
}

func (g *Gateway) Inject(c *Client) error { // inject client to channel
	channels, err := g.db.GetChannelsByUserUUID(c.uuid)
	if err != nil {
		return err
	}
	g.m.Lock()
	defer g.m.Unlock()

	for _, channel := range *channels {
		g.channels[channel.Id] = append(g.channels[channel.Id], c)
	}
	return nil
}

func (g *Gateway) Remove(c *Client) error { //  remove client from channel
	channels, err := g.db.GetChannelsByUserUUID(c.uuid)
	if err != nil {
		return err
	}
	g.m.Lock()
	defer g.m.Unlock()

	for _, channel := range *channels {
		for i, client := range g.channels[channel.Id] {
			if client != c {
				continue
			}
			g.channels[channel.Id] = append(g.channels[channel.Id][:i], g.channels[channel.Id][i+1:]...)
			break
		}
	}
	return nil
}

func (g *Gateway) Boardcast(channelId uint, message *Message) error {
	g.m.RLock()
	defer g.m.RUnlock()
	clients, ok := g.channels[channelId]
	if !ok {
		return errors.New("boardcast error no valid channel id")
	}
	for _, client := range clients {
		client.writeChannel <- message
	}
	return nil
}
