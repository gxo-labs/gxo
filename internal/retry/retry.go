package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/gxo-labs/gxo/internal/template"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
)

type Operation func(ctx context.Context) error

type Config struct {
	Attempts      int
	Delay         time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        float64
	OnError       bool
	TaskName      string
}

type Helper struct {
	log              gxolog.Logger
	randSource       *rand.Rand
	redactedKeywords map[string]struct{}
}

func NewHelper(log gxolog.Logger) *Helper {
	if log == nil {
		panic("retry.NewHelper requires a non-nil logger")
	}
	return &Helper{
		log:              log,
		randSource:       rand.New(rand.NewSource(time.Now().UnixNano())),
		redactedKeywords: make(map[string]struct{}),
	}
}

func (h *Helper) SetRedactedKeywords(keywords map[string]struct{}) {
	h.redactedKeywords = keywords
}

func (h *Helper) Do(ctx context.Context, cfg Config, op Operation) error {
	if cfg.Attempts <= 0 {
		cfg.Attempts = 1
	}
	if cfg.BackoffFactor < 1.0 {
		cfg.BackoffFactor = 1.0
	}
	if cfg.Jitter < 0.0 {
		cfg.Jitter = 0.0
	} else if cfg.Jitter > 1.0 {
		cfg.Jitter = 1.0
	}
	if cfg.Delay < 0 {
		cfg.Delay = 0
	}
	if cfg.MaxDelay < 0 {
		cfg.MaxDelay = 0
	}

	var lastErr error
	logPrefix := ""
	if cfg.TaskName != "" {
		logPrefix = fmt.Sprintf("task=%s ", cfg.TaskName)
	}

	for attempt := 1; attempt <= cfg.Attempts; attempt++ {
		select {
		case <-ctx.Done():
			h.log.Warnf("%sRetry attempt %d/%d cancelled before start: %v", logPrefix, attempt, cfg.Attempts, ctx.Err())
			if lastErr == nil {
				return ctx.Err()
			}
			redactedLastErr := template.RedactSecretsInError(lastErr, h.redactedKeywords)
			return fmt.Errorf("retry cancelled after %d attempts with last error: %w (context: %v)", attempt-1, redactedLastErr, ctx.Err())
		default:
		}

		err := op(ctx)
		lastErr = err

		if err == nil {
			if attempt > 1 {
				h.log.Infof("%sOperation succeeded on attempt %d/%d", logPrefix, attempt, cfg.Attempts)
			}
			return nil
		}

		if attempt == cfg.Attempts || !cfg.OnError {
			break
		}

		currentBaseDelay := float64(cfg.Delay)
		if cfg.BackoffFactor > 1.0 && attempt > 0 {
			backoffMultiplier := math.Pow(cfg.BackoffFactor, float64(attempt-1))
			currentBaseDelay *= backoffMultiplier
		}

		if currentBaseDelay > float64(math.MaxInt64) {
			currentBaseDelay = float64(math.MaxInt64)
		}
		waitDelayDuration := time.Duration(currentBaseDelay)

		if cfg.Jitter > 0.0 {
			jitterFactor := cfg.Jitter * (h.randSource.Float64()*2.0 - 1.0)
			jitterAmount := time.Duration(float64(waitDelayDuration) * jitterFactor)
			waitDelayDuration += jitterAmount
			if waitDelayDuration < 0 {
				waitDelayDuration = 0
			}
		}

		if cfg.MaxDelay > 0 && waitDelayDuration > cfg.MaxDelay {
			waitDelayDuration = cfg.MaxDelay
		}

		redactedErr := template.RedactSecretsInError(err, h.redactedKeywords)
		h.log.Warnf("%sOperation failed on attempt %d/%d (retrying in %v): %v",
			logPrefix, attempt, cfg.Attempts, waitDelayDuration.Truncate(time.Millisecond), redactedErr)

		select {
		case <-time.After(waitDelayDuration):
		case <-ctx.Done():
			h.log.Warnf("%sRetry delay for attempt %d/%d cancelled: %v", logPrefix, attempt+1, cfg.Attempts, ctx.Err())
			redactedLastErr := template.RedactSecretsInError(lastErr, h.redactedKeywords)
			return fmt.Errorf("retry delay cancelled after attempt %d with error: %w (context: %v)", attempt, redactedLastErr, ctx.Err())
		}
	}

	if lastErr != nil {
		redactedErr := template.RedactSecretsInError(lastErr, h.redactedKeywords)
		h.log.Errorf("%sOperation failed definitively after %d attempts: %v", logPrefix, cfg.Attempts, redactedErr)
		return redactedErr
	}

	return gxoerrors.NewConfigError("retry loop finished unexpectedly without success or error", nil)
}