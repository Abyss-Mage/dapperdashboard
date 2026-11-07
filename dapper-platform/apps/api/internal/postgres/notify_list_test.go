package postgres

import (
	"strings"
	"testing"
)

func TestNotifyList(t *testing.T) {
	got, ok := notifyList(" owner@dapper.ca, manager@gmail.com;third@x.io ")
	if !ok || strings.Join(got, "|") != "owner@dapper.ca|manager@gmail.com|third@x.io" {
		t.Fatal("several addresses not read", got, ok)
	}
	if got, ok := notifyList(""); !ok || len(got) != 0 {
		t.Fatal("blank should mean none", got, ok)
	}
	if _, ok := notifyList("owner@dapper.ca, not-an-email"); ok {
		t.Fatal("a bad address was accepted")
	}
	if _, ok := notifyList("Owner <owner@dapper.ca>"); ok {
		t.Fatal("a display name should be refused")
	}
	if _, ok := notifyList(strings.Repeat("a@b.co,", 11)); ok {
		t.Fatal("more than ten addresses accepted")
	}
}
