package emarsys

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_generateWSSE(t *testing.T) {
	c, _ := MakeClient(
		WithTime(func() time.Time {
			return time.Date(2023, 2, 6, 0, 0, 0, 0, time.UTC)
		}),
		WithCredentials("userX", "passY"),
	)
	got := c.generateWSSE()

	var userName, pwd, nonce, created string
	_, err := fmt.Fscanf(
		strings.NewReader(got),
		"UsernameToken Username=%q,PasswordDigest=%q,Nonce=%q,Created=%q",
		&userName,
		&pwd,
		&nonce,
		&created,
	)
	require.NoError(t, err)
	assert.Exactly(t, "userX", userName)
	assert.Exactly(t, "NGE4MWNkMWUwZDVjYjlhNjVkOWMzODk3ZGRmNGMwODlmYWMzMWMwNg==", pwd)
	assert.Exactly(t, "VqRNlECxJcvmZpKDdXKpZNoXZeNZLAJbSZiP", nonce)
	assert.Exactly(t, "2023-02-06T00:00:00Z", created)
}
