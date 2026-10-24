package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	jst       = time.FixedZone("JST", 9*60*60)
	dateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	hhmmRe    = regexp.MustCompile(`^\d{2}:\d{2}$`)
	leadIntRe = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?\d+`)
)

const datetimeLayout = "2006-01-02 15:04:05"

type ctx struct {
	*app
	w http.ResponseWriter
	r *http.Request
}

func errBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func (c *ctx) json(status int, v interface{}) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("encode response: %v", err)
		status = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"error":"Internal Server Error"}`)
	}
	c.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.w.WriteHeader(status)
	c.w.Write(buf.Bytes())
}

func (c *ctx) fail(err error) {
	log.Printf("%s %s: %v", c.r.Method, c.r.URL.RequestURI(), err)
	c.json(http.StatusInternalServerError, errBody(err.Error()))
}

func (c *ctx) methodNotAllowed() {
	c.json(http.StatusMethodNotAllowed, errBody("Method Not Allowed"))
}

func now() time.Time {
	return time.Now().In(jst)
}

func todayDate() string {
	t := now()
	if t.Hour() < 5 {
		t = t.AddDate(0, 0, -1)
	}
	return t.Format("2006-01-02")
}

func (c *ctx) query(key string) (string, bool) {
	v, ok := c.r.URL.Query()[key]
	if !ok || len(v) == 0 {
		return "", false
	}
	return v[len(v)-1], true
}

func (c *ctx) queryInt(key string, def int64) int64 {
	s, ok := c.query(key)
	if !ok {
		return def
	}
	return phpIntString(s)
}

type body map[string]interface{}

func (c *ctx) body() body {
	dec := json.NewDecoder(c.r.Body)
	dec.UseNumber()
	var b body
	if err := dec.Decode(&b); err != nil || b == nil {
		return body{}
	}
	return b
}

func (b body) has(key string) bool {
	return b[key] != nil
}

func (b body) str(key string) (string, bool) {
	s, ok := b[key].(string)
	return s, ok
}

func (b body) strictInt(key string) (int64, bool) {
	n, ok := b[key].(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return i, err == nil
}

func (b body) toInt(key string) int64 {
	switch v := b[key].(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return int64(f)
		}
	case string:
		return phpIntString(v)
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

func (b body) toString(key string) string {
	switch v := b[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "1"
		}
	}
	return ""
}

func phpIntString(s string) int64 {
	m := leadIntRe.FindString(s)
	if m == "" {
		return 0
	}
	i, err := strconv.ParseInt(strings.TrimLeft(m, " \t\n\r\v\f"), 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func phpTrim(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0B")
}
