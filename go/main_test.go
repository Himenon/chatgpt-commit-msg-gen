package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCleanMessage(t *testing.T) {
	raw := "説明です\n```text\nfeat(cli): Codexフォールバックを追加\n```"
	if got := cleanMessage(raw); got != "feat(cli): Codexフォールバックを追加" {
		t.Fatalf("got %q", got)
	}
}

func TestIsConventionalCommit(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"feat: 機能を追加", true}, {"fix(api): handle timeout", true},
		{"Feature: invalid", false}, {"説明文", false}, {"feat(): empty scope", false},
	} {
		if got := isConventionalCommit(tc.value); got != tc.want {
			t.Errorf("isConventionalCommit(%q) = %v", tc.value, got)
		}
	}
}

func TestGenerateWithAPI(t *testing.T) {
	previousClient := apiHTTPClient
	apiHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization")
		}
		var body responseRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "test-model" || body.Input != "prompt" || body.Store {
			t.Errorf("unexpected body: %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"output":[{"type":"message","content":[{"type":"output_text","text":"feat: テストを追加"}]}]}`)),
		}, nil
	})}
	t.Cleanup(func() { apiHTTPClient = previousClient })
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("OPENAI_BASE_URL", "https://example.invalid")
	got, err := generateWithAPI("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "feat: テストを追加" {
		t.Fatalf("got %q", got)
	}
}

func TestCommitPrompt(t *testing.T) {
	t.Setenv("COMMIT_LANGUAGE", "en")
	if got := commitPrompt(); got != defaultPromptEn {
		t.Fatal("English prompt was not selected")
	}
	t.Setenv("COMMIT_PROMPT", " custom ")
	if got := commitPrompt(); got != "custom" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateRejectsInvalidAPIMessage(t *testing.T) {
	previousClient := apiHTTPClient
	apiHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"output":[{"content":[{"type":"output_text","text":"説明だけです"}]}]}`)),
		}, nil
	})}
	t.Cleanup(func() { apiHTTPClient = previousClient })
	t.Setenv("COMMIT_GENERATOR", "api")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", "https://example.invalid")
	if _, _, err := generate("prompt"); err == nil {
		t.Fatal("invalid API message should be rejected")
	}
}
