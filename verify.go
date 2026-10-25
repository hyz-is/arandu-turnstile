package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// The reasons a verification does not succeed. Every error Verify returns
// wraps exactly one of them, so a caller tells them apart with errors.Is.
var (
	// ErrMissingToken is a submission that carried no token: the widget was
	// not solved, or the form was sent by something that never drew it.
	ErrMissingToken = errors.New("turnstile: the submission carried no token")
	// ErrRejected is a token Cloudflare refused, or one it accepted for
	// another hostname or another action.
	ErrRejected = errors.New("turnstile: the token was refused")
	// ErrUnavailable is a verification that could not be completed: Cloudflare
	// did not answer, answered with an error, or answered something that is
	// not a verification.
	ErrUnavailable = errors.New("turnstile: the token could not be verified")
	// ErrMisconfigured is Cloudflare refusing the secret key. It is the
	// operator's to fix, and no visitor can do anything about it.
	ErrMisconfigured = errors.New("turnstile: Cloudflare refused the secret key")
)

// Verifier checks tokens against Cloudflare. One is built at boot and shared:
// it holds no state between calls and is safe for concurrent use.
type Verifier struct {
	siteKey   string
	secretKey string
	hostnames []string
	client    *http.Client
	endpoint  string
	messages  Messages
}

// New returns the Verifier, or the reason it cannot be built.
func New(cfg Config) (*Verifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	v := &Verifier{
		siteKey:   strings.TrimSpace(cfg.SiteKey),
		secretKey: strings.TrimSpace(cfg.SecretKey),
		hostnames: make([]string, 0, len(cfg.Hostnames)),
		client:    cfg.Client,
		endpoint:  cfg.Endpoint,
		messages:  cfg.Messages,
	}
	for _, host := range cfg.Hostnames {
		v.hostnames = append(v.hostnames, strings.ToLower(host))
	}
	if v.client == nil {
		v.client = &http.Client{Timeout: DefaultTimeout}
	}
	if v.endpoint == "" {
		v.endpoint = DefaultEndpoint
	}
	if v.messages.Rejected == "" {
		v.messages.Rejected = DefaultRejectedMessage
	}
	if v.messages.Unavailable == "" {
		v.messages.Unavailable = DefaultUnavailableMessage
	}
	return v, nil
}

// SiteKey is the public key of the widget, for the page that draws it.
func (v *Verifier) SiteKey() string { return v.siteKey }

// Challenge is one token to verify, with what it is expected to prove.
type Challenge struct {
	// Token is the value the widget wrote into FieldName.
	Token string
	// RemoteIP is the address of the visitor, as the application's proxy
	// configuration decided it. It is a hint to Cloudflare and may be empty;
	// a value that is not an address is not sent.
	RemoteIP string
	// Action, when set, is the action the widget had to be drawn with. A token
	// solved for another action is refused, which is what stops a token taken
	// from the newsletter form from being spent on the sign-up form.
	Action string
}

// Result is what Cloudflare answered about a token.
type Result struct {
	// Success is whether Cloudflare accepted the token.
	Success bool
	// ChallengeTS is when the challenge was solved.
	ChallengeTS time.Time
	// Hostname is the hostname of the page the challenge was solved on.
	Hostname string
	// Action and CData are what the widget was drawn with.
	Action string
	CData  string
	// ErrorCodes are Cloudflare's reasons for refusing the token.
	ErrorCodes []string
}

// siteverifyAnswer is the body Cloudflare answers with.
type siteverifyAnswer struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
	Action      string   `json:"action"`
	CData       string   `json:"cdata"`
}

// maxAnswerBytes bounds the body read from Cloudflare. An answer is a few
// hundred bytes, and a body read without a limit is memory spent on whatever
// is on the other end of the connection.
const maxAnswerBytes = 64 << 10

// Verify asks Cloudflare about one token.
//
// A token is spent by asking: Cloudflare answers each token once, and a second
// verification of the same token is refused as a duplicate. So Verify is
// called once per submission, by the handler that acts on it, and never by
// middleware that runs before a decision about whether the request will be
// acted on at all.
//
// It returns the Result whenever Cloudflare answered, including alongside an
// error, so a caller can record why a token was refused.
func (v *Verifier) Verify(ctx context.Context, c Challenge) (Result, error) {
	token := strings.TrimSpace(c.Token)
	if token == "" {
		return Result{}, ErrMissingToken
	}
	if len(token) > MaxTokenLength {
		return Result{}, fmt.Errorf("%w: it is %d bytes long, and no token is longer than %d", ErrRejected, len(token), MaxTokenLength)
	}

	form := url.Values{"secret": {v.secretKey}, "response": {token}}
	if ip := net.ParseIP(strings.TrimSpace(c.RemoteIP)); ip != nil {
		form.Set("remoteip", ip.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		// The error of a failed request names the address it was sent to and
		// never the body, so the secret does not travel with it.
		return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%w: Cloudflare answered %s", ErrUnavailable, resp.Status)
	}
	var answer siteverifyAnswer
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAnswerBytes)).Decode(&answer); err != nil {
		return Result{}, fmt.Errorf("%w: the answer is not a verification: %v", ErrUnavailable, err)
	}

	result := Result{
		Success:    answer.Success,
		Hostname:   answer.Hostname,
		Action:     answer.Action,
		CData:      answer.CData,
		ErrorCodes: answer.ErrorCodes,
	}
	if answer.ChallengeTS != "" {
		if ts, err := time.Parse(time.RFC3339, answer.ChallengeTS); err == nil {
			result.ChallengeTS = ts
		}
	}

	if !answer.Success {
		return result, refusal(answer.ErrorCodes)
	}
	if len(v.hostnames) > 0 && !slices.Contains(v.hostnames, strings.ToLower(answer.Hostname)) {
		return result, fmt.Errorf("%w: it was solved on %q, which is not one of the configured hostnames", ErrRejected, answer.Hostname)
	}
	if c.Action != "" && answer.Action != c.Action {
		return result, fmt.Errorf("%w: it was solved for the action %q, not %q", ErrRejected, answer.Action, c.Action)
	}
	return result, nil
}

// refusal maps Cloudflare's error codes to the reason a caller acts on.
//
// A code nobody listed is a refusal of the token: the verification answered,
// and it did not say yes.
func refusal(codes []string) error {
	listed := strings.Join(codes, ", ")
	if listed == "" {
		listed = "no reason given"
	}
	for _, code := range codes {
		switch code {
		case "missing-input-secret", "invalid-input-secret":
			return fmt.Errorf("%w (%s)", ErrMisconfigured, listed)
		case "internal-error", "bad-request":
			return fmt.Errorf("%w: Cloudflare answered %s", ErrUnavailable, listed)
		case "missing-input-response":
			return fmt.Errorf("%w (%s)", ErrMissingToken, listed)
		}
	}
	return fmt.Errorf("%w (%s)", ErrRejected, listed)
}
