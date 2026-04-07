package main

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressAccessLogAnalyzer parses Nginx Ingress and Traefik access logs
// to detect non-2xx HTTP responses (3xx/4xx/5xx).
type IngressAccessLogAnalyzer struct{}

func (a *IngressAccessLogAnalyzer) Name() string { return "IngressAccessLog" }

type ingressLogTarget struct {
	Description   string
	LabelSelector string
	Container     string // empty = default container
	ParseLine     func(string) *accessLogEntry
}

type accessLogEntry struct {
	StatusCode int
	Method     string
	Path       string
	Upstream   string
}

type errorSummary struct {
	StatusCode int
	Method     string
	Path       string
	Upstream   string
	Count      int
}

// Nginx Ingress default log format:
//
//	$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" ...
//
// Example: 10.0.0.1 - - [06/Apr/2026:10:00:00 +0000] "GET /api/v1/users HTTP/1.1" 502 150 "-" "curl/7.68" ... upstream: "10.0.1.5:8080"
var nginxLogRe = regexp.MustCompile(`"(\w+)\s+(\S+)\s+[^"]*"\s+(\d{3})`)
var nginxUpstreamRe = regexp.MustCompile(`upstream:\s*"([^"]*)"`)

func parseNginxAccessLog(line string) *accessLogEntry {
	m := nginxLogRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	code, _ := strconv.Atoi(m[3])
	entry := &accessLogEntry{StatusCode: code, Method: m[1], Path: m[2]}
	if um := nginxUpstreamRe.FindStringSubmatch(line); um != nil {
		entry.Upstream = um[1]
	}
	return entry
}

// Traefik default Common Log Format:
//
//	$remote_addr - $user [$time] "$method $path $proto" $status $size
//
// Also handles Traefik JSON logs with "status" field.
var traefikLogRe = regexp.MustCompile(`"(\w+)\s+(\S+)\s+[^"]*"\s+(\d{3})`)
var traefikJSONStatusRe = regexp.MustCompile(`"(?:OriginStatus|DownstreamStatus|status)":\s*(\d{3})`)
var traefikJSONMethodRe = regexp.MustCompile(`"(?:RequestMethod|method)":\s*"(\w+)"`)
var traefikJSONPathRe = regexp.MustCompile(`"(?:RequestPath|request)":\s*"([^"]+)"`)
var traefikJSONUpstreamRe = regexp.MustCompile(`"(?:ServiceURL|ServiceAddr|serviceUrl)":\s*"([^"]+)"`)

func parseTraefikAccessLog(line string) *accessLogEntry {
	// Try CLF format first
	if m := traefikLogRe.FindStringSubmatch(line); m != nil {
		code, _ := strconv.Atoi(m[3])
		return &accessLogEntry{StatusCode: code, Method: m[1], Path: m[2]}
	}
	// Try JSON format
	sm := traefikJSONStatusRe.FindStringSubmatch(line)
	if sm == nil {
		return nil
	}
	code, _ := strconv.Atoi(sm[1])
	entry := &accessLogEntry{StatusCode: code}
	if mm := traefikJSONMethodRe.FindStringSubmatch(line); mm != nil {
		entry.Method = mm[1]
	}
	if pm := traefikJSONPathRe.FindStringSubmatch(line); pm != nil {
		entry.Path = pm[1]
	}
	if um := traefikJSONUpstreamRe.FindStringSubmatch(line); um != nil {
		entry.Upstream = um[1]
	}
	return entry
}

var ingressLogTargets = []ingressLogTarget{
	{"Nginx Ingress", "app.kubernetes.io/name=ingress-nginx", "controller", parseNginxAccessLog},
	{"Nginx Ingress (alt)", "app=nginx-ingress", "", parseNginxAccessLog},
	{"Traefik", "app.kubernetes.io/name=traefik", "traefik", parseTraefikAccessLog},
	{"Traefik (legacy)", "app=traefik", "", parseTraefikAccessLog},
}

const (
	accessLogTailLines = 200 // last N lines per pod
	maxErrorGroups     = 20  // max distinct error groups to report
)

func (a *IngressAccessLogAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	for _, target := range ingressLogTargets {
		sel := target.LabelSelector
		if labelSelector != "" {
			sel = sel + "," + labelSelector
		}
		pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: sel,
		})
		if err != nil || len(pods.Items) == 0 {
			continue
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}

			container := target.Container
			if container == "" && len(pod.Spec.Containers) > 0 {
				container = pod.Spec.Containers[0].Name
			}

			tailLines := int64(accessLogTailLines)
			logOpts := &corev1.PodLogOptions{
				Container: container,
				TailLines: &tailLines,
			}

			stream, err := client.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts).Stream(ctx)
			if err != nil {
				continue
			}

			summaries := parseAccessLogs(stream, target.ParseLine)
			stream.Close()

			if len(summaries) == 0 {
				continue
			}

			var failures []string
			for _, s := range summaries {
				line := fmt.Sprintf("HTTP %d", s.StatusCode)
				if s.Method != "" {
					line += " " + s.Method
				}
				if s.Path != "" {
					line += " " + s.Path
				}
				if s.Upstream != "" {
					line += " -> " + s.Upstream
				}
				line += fmt.Sprintf(" (%d 次)", s.Count)
				failures = append(failures, line)
			}

			parent, _ := client.GetParent(pod.ObjectMeta)
			results = append(results, AnalyzeResult{
				Kind:      fmt.Sprintf("IngressLog[%s]", target.Description),
				Name:      fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Error:     failures,
				ParentObj: parent,
			})
		}
	}

	return results, nil
}

// parseAccessLogs reads log lines, extracts non-2xx entries, and returns aggregated summaries.
func parseAccessLogs(reader interface{ Read([]byte) (int, error) }, parseLine func(string) *accessLogEntry) []errorSummary {
	scanner := bufio.NewScanner(reader)
	counts := make(map[string]*errorSummary)

	for scanner.Scan() {
		entry := parseLine(scanner.Text())
		if entry == nil {
			continue
		}
		// Report 3xx, 4xx, 5xx
		if entry.StatusCode < 300 {
			continue
		}

		// Normalize path: strip query string, collapse IDs for grouping
		path := normalizePath(entry.Path)
		key := fmt.Sprintf("%d|%s|%s", entry.StatusCode, entry.Method, path)

		if existing, ok := counts[key]; ok {
			existing.Count++
			if existing.Upstream == "" && entry.Upstream != "" {
				existing.Upstream = entry.Upstream
			}
		} else {
			counts[key] = &errorSummary{
				StatusCode: entry.StatusCode,
				Method:     entry.Method,
				Path:       path,
				Upstream:   entry.Upstream,
				Count:      1,
			}
		}
	}

	// Sort by count desc, then status code desc
	sorted := make([]errorSummary, 0, len(counts))
	for _, s := range counts {
		sorted = append(sorted, *s)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].StatusCode > sorted[j].StatusCode
	})

	if len(sorted) > maxErrorGroups {
		sorted = sorted[:maxErrorGroups]
	}
	return sorted
}

// normalizePath strips query strings and collapses UUID/numeric path segments for grouping.
var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
var numericSegmentRe = regexp.MustCompile(`/\d+(/|$)`)

func normalizePath(path string) string {
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	path = uuidRe.ReplaceAllString(path, ":id")
	path = numericSegmentRe.ReplaceAllString(path, "/:id$1")
	return path
}
