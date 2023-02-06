package emarsys

import (
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type OptionHTTPRequestFn func(req *http.Request) (*http.Response, error)

func (fn OptionHTTPRequestFn) apply(c *Client) error {
	c.doFn = fn
	return nil
}

func WithHTTPClient(doFn OptionHTTPRequestFn) Option {
	return doFn
}

// WithTime shall only be used for testing. It also sets the rand source to the time.
func WithTime(now func() time.Time) Option {
	return optionFn(func(c *Client) error {
		c.rand = rand.NewSource(now().UnixNano())
		c.now = now
		return nil
	})
}

func WithCredentials(user, secret string) Option {
	return optionFn(func(c *Client) error {
		c.user = user
		c.secret = secret
		return nil
	})
}

func WithEnableStaging(envvarName ...string) Option {
	return optionFn(func(c *Client) error {
		if len(envvarName) == 1 {
			b, err := strconv.ParseBool(os.Getenv(envvarName[0]))
			c.isStaging = b && err == nil
		} else {
			c.isStaging = true
		}

		return nil
	})
}

type optionFn func(c *Client) error

func (o optionFn) apply(c *Client) error {
	return o(c)
}
