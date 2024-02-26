package channels

import (
	"errors"
	"github.com/gotify/plugin-api"
)

type NoneClient struct {
}

func NewNoneClient() *NoneClient {
	return &NoneClient{}
}

func (c *NoneClient) SendMessage(plugin.Message) error {
	return errors.New("unsupported channel type")
}
