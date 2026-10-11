package catalog

import (
	"jstihuo/internal/app/identity"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Input struct {
	Code         string      `json:"code"`
	Name         string      `json:"name"`
	CategoryID   string      `json:"category_id,omitempty"`
	Attributes   []Attribute `json:"attributes"`
	Description  string      `json:"description_html"`
	MediaIDs     []string    `json:"media_ids"`
	ExternalCode string      `json:"external_spec_code,omitempty"`
	ExternalName string      `json:"external_spec_name,omitempty"`
	From         *time.Time  `json:"redeem_from,omitempty"`
	Until        *time.Time  `json:"redeem_until,omitempty"`
	Excluded     []string    `json:"excluded_region_codes"`
	Sort         int         `json:"sort_order"`
	Version      int64       `json:"version,omitempty"`
}
type Product struct {
	Input
	ID     string `json:"id"`
	Status string `json:"status"`
}
type Public struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Attributes  []Attribute `json:"attributes"`
	Description string      `json:"description_html"`
	Images      []string    `json:"image_urls"`
}
type CategoryInput struct {
	Name string `json:"name"`
	Sort int    `json:"sort_order"`
}
type Category struct {
	CategoryInput
	ID      string `json:"id"`
	Version int64  `json:"version"`
}
type StatusInput struct {
	Version int64  `json:"version"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}
type File struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	State string `json:"state"`
	MIME  string `json:"mime_type"`
	Size  int64  `json:"size_bytes"`
}
type Ref struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	State   string `json:"state"`
	Type    string `json:"entity_type"`
	HTTP    int    `json:"http_status"`
}
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_after_id,omitempty"`
}
type Filter struct {
	After     int64
	Limit     int
	Q, Status string
}

func fault(n int, c string) error { return &identity.Fault{Status: n, Code: c} }
func parseID(s string) (int64, error) {
	if !regexp.MustCompile(`^[1-9][0-9]{0,19}$`).MatchString(s) {
		return 0, fault(422, "INVALID_ARGUMENT")
	}
	v, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return 0, fault(422, "INVALID_ARGUMENT")
	}
	return v, nil
}
func length(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max && (min == 0 || strings.TrimSpace(s) != "")
}
func Validate(in *Input, update bool) error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if !regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`).MatchString(in.Code) || !length(in.Name, 1, 120) || !length(in.Description, 0, 50000) || in.Sort < 0 || in.Sort > 100000 || !length(in.ExternalCode, 0, 64) || !length(in.ExternalName, 0, 120) {
		return fault(422, "INVALID_ARGUMENT")
	}
	if update && (in.Version < 1 || in.Version > 9007199254740991) {
		return fault(422, "INVALID_ARGUMENT")
	}
	if in.CategoryID != "" {
		if _, e := parseID(in.CategoryID); e != nil {
			return e
		}
	}
	if in.Attributes == nil || len(in.Attributes) > 20 || in.Excluded == nil || len(in.Excluded) > 500 || len(in.MediaIDs) < 1 || len(in.MediaIDs) > 9 {
		return fault(422, "INVALID_ARGUMENT")
	}
	seen := map[string]bool{}
	for _, m := range in.MediaIDs {
		if _, e := parseID(m); e != nil {
			return e
		}
		if seen[m] {
			return fault(422, "INVALID_ARGUMENT")
		}
		seen[m] = true
	}
	for _, a := range in.Attributes {
		if !length(a.Name, 1, 40) || !length(a.Value, 1, 120) {
			return fault(422, "INVALID_ARGUMENT")
		}
	}
	for _, r := range in.Excluded {
		if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(r) {
			return fault(422, "INVALID_ARGUMENT")
		}
	}
	if in.From != nil && in.Until != nil && !in.From.Before(*in.Until) {
		return fault(422, "INVALID_ARGUMENT")
	}
	return nil
}
func project(p Product) Public {
	images := []string{}
	for _, id := range p.MediaIDs {
		images = append(images, "/media/"+id)
	}
	return Public{p.ID, p.Name, p.Attributes, p.Description, images}
}
