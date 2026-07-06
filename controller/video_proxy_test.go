package controller

import "testing"

func TestResolvePossiblyRelativeVideoURL(t *testing.T) {
	t.Run("relative output path uses channel base URL", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("/outputs/task_abc.mp4", "http://provider:38472")
		want := "http://provider:38472/outputs/task_abc.mp4"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("absolute URL unchanged", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("https://cdn.example.com/task.mp4", "http://provider:38472")
		want := "https://cdn.example.com/task.mp4"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("data URL unchanged", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("data:video/mp4;base64,AAAA", "http://provider:38472")
		want := "data:video/mp4;base64,AAAA"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestUpstreamVideoOutputURL(t *testing.T) {
	got := upstreamVideoOutputURL("http://paco-dola2api-er9b9x-dola2api-1:38472", "1311ad37-7251-4fad-8c08-4fafd9bebe96")
	want := "http://paco-dola2api-er9b9x-dola2api-1:38472/outputs/task_1311ad37-7251-4fad-8c08-4fafd9bebe96.mp4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := upstreamVideoOutputURL("https://api.openai.com", "abc"); got != "" {
		t.Fatalf("openai base should not map to outputs URL, got %q", got)
	}
}
