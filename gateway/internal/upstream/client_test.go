package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListVoicesLoadsCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tts/list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
		  {"ShortName":"zh-CN-XiaoxiaoNeural","Locale":"zh-CN","Gender":"Female"},
		  {"ShortName":"zh-CN-YunxiNeural","Locale":"zh-CN","Gender":"Male"}
		]`))
	}))
	defer server.Close()

	client := New(server.URL, 5)
	voices, err := client.ListVoices(context.Background())
	if err != nil {
		t.Fatalf("list voices: %v", err)
	}

	if len(voices) != 2 {
		t.Fatalf("expected 2 voices, got %d", len(voices))
	}
}

func TestSynthesizeBuildsExpectedQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		if values.Get("text") != "hello world" {
			t.Fatalf("unexpected text: %q", values.Get("text"))
		}
		if values.Get("voiceName") != "zh-CN-XiaoxiaoNeural" {
			t.Fatalf("unexpected voice: %q", values.Get("voiceName"))
		}
		if values.Get("thread") != "1" {
			t.Fatalf("unexpected thread: %q", values.Get("thread"))
		}
		if values.Get("shardLength") != "400" {
			t.Fatalf("unexpected shardLength: %q", values.Get("shardLength"))
		}
		w.Header().Set("Content-Type", "audio/mp3")
		_, _ = w.Write([]byte("mp3-bytes"))
	}))
	defer server.Close()

	client := New(server.URL, 5)
	resp, err := client.Synthesize(context.Background(), SynthesizeParams{
		Text:        "hello world",
		Voice:       "zh-CN-XiaoxiaoNeural",
		Thread:      1,
		ShardLength: 400,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(body) != "mp3-bytes" {
		t.Fatalf("unexpected body %q", body)
	}
}
