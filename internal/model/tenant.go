package model

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var slugRegex = regexp.MustCompile(`^[a-z][-a-z0-9]{1,26}[a-z0-9]$`)

type Tenant struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NowUTC() string {
	return time.Now().UTC().Truncate(time.Millisecond).Format(time.RFC3339)
}

func (t *Tenant) Validate() error {
	if t.Slug == "" {
		return errors.New("slug is required")
	}
	if !slugRegex.MatchString(t.Slug) {
		return fmt.Errorf("invalid slug %q: must match ^[a-z][-a-z0-9]{1,26}[a-z0-9]$", t.Slug)
	}
	if t.Name == "" {
		return errors.New("name is required")
	}
	return nil
}
