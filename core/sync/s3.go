package sync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrS3 is returned when the S3 service refuses a request.
const ErrS3 errs.Code = "sync.s3"

// S3Store keeps the files in a bucket of Amazon S3 or an S3-compatible
// service (MinIO, Cloudflare R2, Backblaze B2, Wasabi...). Requests use
// path-style URLs and signature version 4.
type S3Store struct {
	// Endpoint is the service URL, e.g. https://s3.eu-west-1.amazonaws.com
	// or https://<account>.r2.cloudflarestorage.com.
	Endpoint  string
	Region    string
	Bucket    string
	Prefix    string // folder inside the bucket; "Shellvane-sync" when empty
	AccessKey string
	SecretKey string
	Client    *http.Client
	Now       func() time.Time
}

// Where is the bucket and folder, without credentials.
func (s S3Store) Where() string {
	return strings.TrimSuffix(s.Endpoint, "/") + "/" + s.Bucket + "/" + s.prefix()
}

func (s S3Store) prefix() string {
	p := strings.Trim(s.Prefix, "/")
	if p == "" {
		p = "Shellvane-sync"
	}
	return p
}

func (s S3Store) region() string {
	if s.Region == "" {
		return "us-east-1"
	}
	return s.Region
}

func (s S3Store) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// do sends a signed request; key is the object key (empty for the bucket).
func (s S3Store) do(ctx context.Context, method, key string, query url.Values, body []byte) (*http.Response, error) {
	u, err := url.Parse(strings.TrimSuffix(s.Endpoint, "/"))
	if err != nil || u.Host == "" {
		return nil, errs.New(ErrS3, errs.Params{"status": "endpoint"})
	}
	u.Path = "/" + s.Bucket
	if key != "" {
		u.Path += "/" + key
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, errs.Wrap(err, ErrS3, errs.Params{"status": ""})
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	sign(req, body, s.AccessKey, s.SecretKey, s.region(), "s3", now().UTC())
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, errs.Wrap(err, ErrS3, errs.Params{"status": ""})
	}
	return resp, nil
}

func (s S3Store) fail(resp *http.Response) error {
	var e struct {
		Code string `xml:"Code"`
	}
	xml.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&e)
	status := resp.Status
	if e.Code != "" {
		status += " " + e.Code
	}
	return errs.New(ErrS3, errs.Params{"status": status})
}

// Get downloads an object.
func (s S3Store) Get(ctx context.Context, name string) ([]byte, error) {
	resp, err := s.do(ctx, http.MethodGet, s.prefix()+"/"+name, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode/100 != 2 {
		return nil, s.fail(resp)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

// Put uploads an object.
func (s S3Store) Put(ctx context.Context, name string, data []byte) error {
	resp, err := s.do(ctx, http.MethodPut, s.prefix()+"/"+name, nil, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return s.fail(resp)
	}
	return nil
}

// List names the objects of the folder (ListObjectsV2, all pages).
func (s S3Store) List(ctx context.Context) ([]string, error) {
	var out []string
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {s.prefix() + "/"}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		resp, err := s.do(ctx, http.MethodGet, "", q, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode/100 != 2 {
			err := s.fail(resp)
			resp.Body.Close()
			return nil, err
		}
		var page struct {
			Contents []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
			IsTruncated bool   `xml:"IsTruncated"`
			Next        string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, errs.Wrap(err, ErrS3, errs.Params{"status": "list"})
		}
		for _, c := range page.Contents {
			name := strings.TrimPrefix(c.Key, s.prefix()+"/")
			if name != "" && !strings.Contains(name, "/") {
				out = append(out, name)
			}
		}
		if !page.IsTruncated || page.Next == "" {
			return out, nil
		}
		token = page.Next
	}
}

// sign adds AWS signature version 4 headers to req.
func sign(req *http.Request, body []byte, accessKey, secretKey, region, service string, t time.Time) {
	amzDate := t.Format("20060102T150405Z")
	day := t.Format("20060102")
	payload := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(payload[:])
	req.Header.Set("X-Amz-Date", amzDate)
	if service == "s3" {
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	}

	// Canonical headers: host plus every header set so far, lower case.
	headers := map[string]string{"host": req.URL.Host}
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ",")
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + strings.TrimSpace(headers[k]) + "\n")
	}
	signed := strings.Join(names, ";")

	canonical := strings.Join([]string{
		req.Method,
		canonicalPath(req.URL),
		canonicalQuery(req.URL.Query()),
		canonHeaders.String(),
		signed,
		payloadHash,
	}, "\n")
	scope := day + "/" + region + "/" + service + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := hmacSHA256([]byte("AWS4"+secretKey), day)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+", SignedHeaders="+signed+", Signature="+signature)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// canonicalPath URI-encodes each segment (RFC 3986 unreserved kept).
func canonicalPath(u *url.URL) string {
	p := u.Path
	if p == "" {
		return "/"
	}
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = awsEscape(s)
	}
	return strings.Join(parts, "/")
}

func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, awsEscape(k)+"="+awsEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

func awsEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}
