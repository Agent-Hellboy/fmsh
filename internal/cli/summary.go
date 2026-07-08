package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"fmsh/internal/events"
	"fmsh/internal/output"
)

// summary aggregates a window of events for human-readable reports.
type summary struct {
	FilesCreated  []string
	FilesModified []string
	FilesDeleted  []string
	Processes     []string // started, deduped, with tool hint
	Ports         []string // "localhost:8000 by python3"
	RisksHigh     []string
	RisksMedium   []string
	RisksLow      []string
}

func buildSummary(evs []events.Event) summary {
	var s summary
	seenProc := map[string]bool{}
	seenPort := map[string]bool{}
	seenFile := map[string]bool{}

	for _, ev := range evs {
		switch ev.Type {
		case events.TypeFileCreate:
			addFile(&s.FilesCreated, seenFile, "c|"+ev.Path, shortPath(ev.Path))
		case events.TypeFileWrite, events.TypeGitFileModified:
			addFile(&s.FilesModified, seenFile, "m|"+ev.Path, shortPath(ev.Path))
		case events.TypeGitFileAdded:
			addFile(&s.FilesCreated, seenFile, "c|"+ev.Path, shortPath(ev.Path))
		case events.TypeFileDelete, events.TypeGitFileDeleted:
			addFile(&s.FilesDeleted, seenFile, "d|"+ev.Path, shortPath(ev.Path))
		case events.TypeProcessStart:
			label := ev.ProcessName
			if tool, ok := ev.Metadata["detected_tool"].(string); ok && tool != "" {
				label = tool
			}
			if label != "" && !seenProc[label] {
				seenProc[label] = true
				s.Processes = append(s.Processes, label)
			}
		case events.TypePortOpen:
			line := ev.Subject
			if ev.ProcessName != "" {
				line += " by " + ev.ProcessName
			}
			if !seenPort[line] {
				seenPort[line] = true
				s.Ports = append(s.Ports, line)
			}
		}
		if ev.Category == events.CategoryRisk {
			switch ev.Severity {
			case events.SeverityHigh:
				s.RisksHigh = append(s.RisksHigh, ev.Subject)
			case events.SeverityMedium:
				s.RisksMedium = append(s.RisksMedium, ev.Subject)
			default:
				s.RisksLow = append(s.RisksLow, ev.Subject)
			}
		}
	}
	return s
}

func addFile(list *[]string, seen map[string]bool, key, val string) {
	if val == "" || seen[key] {
		return
	}
	seen[key] = true
	*list = append(*list, val)
}

// suggestedReview derives review guidance from the summary.
func (s summary) suggestedReview() []string {
	var out []string
	if len(s.RisksHigh) > 0 || hasDependencyRisk(s) {
		out = append(out, "Inspect dependency and lockfile changes.")
	}
	if hasSecretRisk(s) {
		out = append(out, "Review secret/config file access.")
	}
	if len(s.Ports) > 0 {
		out = append(out, "Check local server processes and why ports opened.")
	}
	if len(s.FilesModified)+len(s.FilesCreated) > 0 {
		out = append(out, "Run tests before committing.")
	}
	if len(out) == 0 {
		out = append(out, "Nothing notable — routine activity.")
	}
	return out
}

func hasSecretRisk(s summary) bool {
	for _, r := range append(append([]string{}, s.RisksHigh...), s.RisksMedium...) {
		if strings.Contains(strings.ToLower(r), "secret") || strings.Contains(r, ".env") ||
			strings.Contains(strings.ToLower(r), "config") {
			return true
		}
	}
	return false
}

func hasDependencyRisk(s summary) bool {
	for _, r := range s.RisksMedium {
		if strings.Contains(strings.ToLower(r), "dependency") || strings.Contains(strings.ToLower(r), "manifest") {
			return true
		}
	}
	return false
}

// renderWhatChanged prints the "what changed" style report.
func renderWhatChanged(w io.Writer, s summary) {
	output.Section(w, "Files:")
	if len(s.FilesModified)+len(s.FilesCreated)+len(s.FilesDeleted) == 0 {
		output.EmptyNote(w, "no file changes")
	} else {
		if n := len(s.FilesModified); n > 0 {
			fmt.Fprintf(w, "  %s %d file(s) modified\n", output.SymModified, n)
		}
		if n := len(s.FilesCreated); n > 0 {
			fmt.Fprintf(w, "  %s %d file(s) created\n", output.Green(output.SymAdded), n)
		}
		if n := len(s.FilesDeleted); n > 0 {
			fmt.Fprintf(w, "  %s %d file(s) deleted\n", output.Yellow(output.SymRemoved), n)
		}
	}

	output.Section(w, "Processes:")
	if len(s.Processes) == 0 {
		output.EmptyNote(w, "none observed")
	} else {
		for _, p := range dedupeSorted(s.Processes) {
			fmt.Fprintf(w, "  %s %s\n", output.Green(output.SymAdded), p)
		}
	}

	output.Section(w, "Network:")
	if len(s.Ports) == 0 {
		output.EmptyNote(w, "no ports opened")
	} else {
		for _, p := range s.Ports {
			fmt.Fprintf(w, "  %s %s\n", output.Green(output.SymAdded), p)
		}
	}

	output.Section(w, "Risks:")
	renderRiskLines(w, s)

	output.Section(w, "Suggested review:")
	for i, r := range s.suggestedReview() {
		fmt.Fprintf(w, "  %d. %s\n", i+1, r)
	}
}

func renderRiskLines(w io.Writer, s summary) {
	if len(s.RisksHigh)+len(s.RisksMedium)+len(s.RisksLow) == 0 {
		output.EmptyNote(w, "none detected")
		return
	}
	for _, r := range s.RisksHigh {
		fmt.Fprintf(w, "  %s %s\n", output.Red(output.SymRisk), r)
	}
	for _, r := range s.RisksMedium {
		fmt.Fprintf(w, "  %s %s\n", output.Yellow(output.SymRisk), r)
	}
	for _, r := range s.RisksLow {
		fmt.Fprintf(w, "  %s %s\n", output.SymRisk, r)
	}
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
