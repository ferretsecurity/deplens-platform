package snyk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestClientHTTPError(t *testing.T) {
	for _, resource := range []string{"targets", "projects", "dependency_graph"} {
		t.Run(resource, func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				if resource == "projects" && strings.HasSuffix(r.URL.Path, "/targets") {
					fmt.Fprint(w, `{"data":[],"links":{}}`)
					return
				}
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":"untrusted-upstream-secret fixture-token fixture-org"}`)
			}))
			defer server.Close()
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			client := NewClient()
			client.HTTP.Transport = fixtureTransport{target}
			if resource == "dependency_graph" {
				_, err = client.Snapshot(context.Background(), "fixture-org", "us", "fixture-token", Repository{Projects: []Project{{ID: "fixture-project", Type: "npm"}}})
			} else {
				_, err = client.Repositories(context.Background(), "fixture-org", "us", "fixture-token")
			}
			var upstream *HTTPError
			if !errors.As(err, &upstream) || upstream.StatusCode != http.StatusForbidden || string(upstream.Resource) != resource {
				t.Fatalf("expected typed %s HTTP 403 error, got %v", resource, err)
			}
			if !strings.HasPrefix(err.Error(), "Snyk request failed (HTTP 403)") {
				t.Fatalf("lost safe import error prefix: %v", err)
			}
			for _, secret := range []string{"untrusted-upstream-secret", "fixture-token", "fixture-org", "/rest/", "/v1/"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error contains private request or response data: %v", err)
				}
			}
			if resource == "projects" && !reflect.DeepEqual(requests, []string{"/rest/orgs/fixture-org/targets", "/rest/orgs/fixture-org/projects"}) {
				t.Fatalf("unexpected request sequence: %v", requests)
			}
		})
	}
}
