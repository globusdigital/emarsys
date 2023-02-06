package emarsys

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/go-cleanhttp"
)

const (
	mockURL     = `https://stoplight.io/mocks/emarsys-sap/emarsys-api/182542`
	endpointURL = `https://api.emarsys.net`
	pathBase    = `/api/v2`
)

type Client struct {
	user      string
	secret    string
	doFn      OptionHTTPRequestFn
	now       func() time.Time
	rand      rand.Source
	isStaging bool // will be added to the request and PROD EMARSYS handles those emails internally as they do not have a dev env.
}

type Option interface {
	apply(*Client) error
}

func MakeClient(opts ...Option) (Client, error) {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return Client{}, errors.New("cannot seed math/rand package with cryptographically secure random number generator")
	}

	c := Client{
		rand: rand.NewSource(int64(binary.LittleEndian.Uint64(b[:]))),
		now:  time.Now,
	}
	for _, opt := range opts {
		if err := opt.apply(&c); err != nil {
			return Client{}, err
		}
	}

	if c.doFn == nil {
		hc := cleanhttp.DefaultClient()
		if hc.Transport.(*http.Transport).TLSClientConfig == nil {
			hc.Transport.(*http.Transport).TLSClientConfig = &tls.Config{}
		}
		hc.Transport.(*http.Transport).TLSClientConfig.MinVersion = tls.VersionTLS12
		hc.Timeout = 2 * time.Minute
		c.doFn = hc.Do
	}

	return c, nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var lenLetterBytes = int64(len(letterBytes))

func generateRandString(digestBuf *bytes.Buffer) string {
	var b [36]byte
	for i := range b {
		b[i] = letterBytes[rand.Int63()%lenLetterBytes]
	}
	digestBuf.Write(b[:])
	return string(b[:])
}

// generateWSSE creates the auth string
// https://dev.emarsys.com/docs/emarsys-api/ZG9jOjI0ODk5NzAx-authentication
func (c *Client) generateWSSE() string {
	rand.Seed(c.now().UnixNano())
	timestamp := c.now().Format(time.RFC3339)

	var digestBuf bytes.Buffer
	nonce := generateRandString(&digestBuf)
	digestBuf.WriteString(timestamp)
	digestBuf.WriteString(c.secret)

	h := sha1.New()
	_, _ = digestBuf.WriteTo(h)
	hashed := hex.EncodeToString(h.Sum(nil))
	passwordDigest := base64.StdEncoding.EncodeToString([]byte(hashed))

	var hdr strings.Builder
	fmt.Fprintf(
		&hdr,
		"UsernameToken Username=%q,PasswordDigest=%q,Nonce=%q,Created=%q",
		c.user,
		passwordDigest,
		nonce,
		timestamp,
	)

	return hdr.String()
}

func (c *Client) do(req *http.Request, v any) error {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-WSSE", c.generateWSSE())

	return backoff.Retry(func() error {
		resp, err := c.doFn(req)
		defer closeResponse(resp)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("emarsys.client: failed to execute HTTP request: %w", err))
		}

		var buf bytes.Buffer
		body := io.TeeReader(resp.Body, &buf)
		dec := json.NewDecoder(body)

		if !c.isSuccess(resp.StatusCode) {
			var errResp ResponseEnvelope
			if err := dec.Decode(&errResp); err != nil {
				return fmt.Errorf("emarsys.client: failed to unmarshal HTTP error response: %w", err)
			}
			errResp.HTTPStatusCode = resp.StatusCode
			errResp.Data = buf.Bytes()
			// TODO figure out which codes need retries
			return &errResp
		}

		respEnv := ResponseEnvelope{
			HTTPStatusCode: resp.StatusCode,
		}
		if err := dec.Decode(&respEnv); err != nil {
			respEnv.UnmarshalErr = fmt.Errorf("emarsys.client: failed to unmarshal HTTP envelop response: %w", err)
			return backoff.Permanent(&respEnv)
		}

		if respEnv.ReplyCode != 0 {
			// https://dev.emarsys.com/docs/emarsys-api/ZG9jOjI0ODk5NzY4-http-200-errors
			return backoff.Permanent(&respEnv)
		}

		if err := json.Unmarshal(respEnv.Data, v); err != nil {
			respEnv.UnmarshalErr = fmt.Errorf("emarsys.client: failed to unmarshal HTTP data response: %w", err)
			return backoff.Permanent(&respEnv)
		}

		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
}

func (c *Client) isSuccess(statusCode int) bool {
	return statusCode != http.StatusOK
}

func closeResponse(r *http.Response) {
	if r == nil || r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
}
