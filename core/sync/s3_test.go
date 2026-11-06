package sync

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// The example of the AWS Signature Version 4 documentation (IAM ListUsers).
func TestSignMatchesAWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	at, _ := time.Parse("20060102T150405Z", "20150830T123600Z")
	sign(req, nil, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "iam", at)
	want := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization =\n%s\nwant\n%s", got, want)
	}
}

// fakeS3 is an in-memory bucket that pages listings one key at a time.
type fakeS3 struct {
	mu      gosync.Mutex
	objects map[string][]byte
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AK/") || r.Header.Get("X-Amz-Content-Sha256") == "" {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "<Error><Code>SignatureDoesNotMatch</Code></Error>")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/bucket/")
	switch {
	case r.Method == http.MethodPut:
		data, _ := io.ReadAll(r.Body)
		f.objects[key] = data
	case r.Method == http.MethodGet && r.URL.Path == "/bucket" && r.URL.Query().Get("list-type") == "2":
		var keys []string
		for k := range f.objects {
			if strings.HasPrefix(k, r.URL.Query().Get("prefix")) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		start, _ := strconv.Atoi(r.URL.Query().Get("continuation-token"))
		type content struct{ Key string }
		res := struct {
			XMLName     xml.Name `xml:"ListBucketResult"`
			Contents    []content
			IsTruncated bool
			Next        string `xml:"NextContinuationToken,omitempty"`
		}{}
		if start < len(keys) {
			res.Contents = []content{{keys[start]}}
		}
		if start+1 < len(keys) {
			res.IsTruncated, res.Next = true, strconv.Itoa(start+1)
		}
		xml.NewEncoder(w).Encode(res)
	case r.Method == http.MethodGet:
		data, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(data)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestSyncThroughS3(t *testing.T) {
	srv := httptest.NewServer(&fakeS3{objects: map[string][]byte{"other/x.tsync": []byte("not ours")}})
	defer srv.Close()
	st := func() S3Store {
		return S3Store{Endpoint: srv.URL, Region: "eu-west-1", Bucket: "bucket", AccessKey: "AK", SecretKey: "SK"}
	}
	a, b := deviceOn(t, st(), "laptop"), deviceOn(t, st(), "desktop")
	roundTrip(t, a, b)

	bad := st()
	bad.AccessKey = "WRONG"
	if _, err := bad.List(t.Context()); errs.CodeOf(err) != ErrS3 {
		t.Errorf("bad credentials: %v", err)
	}
	if w := st().Where(); w != srv.URL+"/bucket/Shellvane-sync" {
		t.Errorf("Where = %q", w)
	}
}
