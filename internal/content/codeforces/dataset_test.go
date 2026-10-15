package codeforces

import (
	"net/http"
	"testing"
)

func TestNextLink(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Add("Link", `<https://huggingface.co/api/datasets/x/tree/main?cursor=abc>; rel="next"`)
	if got := nextLink(h); got != "https://huggingface.co/api/datasets/x/tree/main?cursor=abc" {
		t.Errorf("nextLink = %q", got)
	}
	if got := nextLink(http.Header{}); got != "" {
		t.Errorf("nextLink of nothing = %q", got)
	}
}
