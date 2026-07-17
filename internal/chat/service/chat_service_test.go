package service

import (
	"strings"
	"testing"
)

func TestFormatChatAssistantContentForWorkerUsesProblemID(t *testing.T) {
	raw := `{"answer":"先练最短路。","weak_tags":["图论"],"recommended_problems":[{"problem_id":42,"title":"最短路","reason":"练习 Dijkstra"}]}`
	content := FormatChatAssistantContentForWorker(raw)

	for _, expected := range []string{"先练最短路。", "Weak tags: 图论", "#42 最短路: 练习 Dijkstra"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("formatted content %q does not contain %q", content, expected)
		}
	}
}
