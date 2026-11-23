// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package relaypub

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublishRejectsUnsafeEventTypes(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hit = true }))
	defer srv.Close()
	for _, gw := range []string{srv.URL, ""} {
		c := &Client{GatewayBase: gw, RelayBase: srv.URL}
		for _, typ := range []string{"", "../admin", "a/b", "a?x=1", "a b", "a:publish", "a\nb", "-lead", "a%2fb"} {
			if _, err := c.PublishEventType(typ, "info", "s", "k", nil); err == nil {
				t.Fatalf("gateway=%q type %q accepted", gw, typ)
			}
		}
	}
	if hit {
		t.Fatal("unsafe type reached the network")
	}
	c := &Client{RelayBase: srv.URL}
	if _, err := c.PublishEventType("crop.advisory", "info", "s", "k", nil); err != nil {
		t.Fatal(err)
	}
}
