package health

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxLinkIssuesSample = 40
	maxFilesPerWalk     = 250_000
)

type FSAuditResult struct {
	Zone          string
	Root          string
	FilesScanned  int
	FilesOK       int
	FilesExcluded int
	FilesSkipped  int
	Issues        []LinkIssue
	IssueCount    int
	Truncated     bool
}

func AuditRoot(ctx context.Context, root, zone string, ex PathExclusions) FSAuditResult {
	res := FSAuditResult{Zone: zone, Root: root}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			res.FilesSkipped = -1
			return res
		}
		res.Issues = append(res.Issues, LinkIssue{
			Path: root, Zone: zone, Reason: fmt.Sprintf("stat root: %v", err),
		})
		return res
	}
	if !info.IsDir() {
		res.Issues = append(res.Issues, LinkIssue{
			Path: root, Zone: zone, Reason: "path is not a directory",
		})
		return res
	}

	removingDest := make(map[string]struct{}, len(ex.RemovingDest))
	for _, p := range ex.RemovingDest {
		removingDest[filepath.Clean(p)] = struct{}{}
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if res.FilesScanned >= maxFilesPerWalk {
			res.Truncated = true
			return filepath.SkipAll
		}

		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			res.FilesSkipped++
			return nil
		}

		res.FilesScanned++

		cleanPath := filepath.Clean(path)
		excluded := PathHasPrefix(cleanPath, ex.Prefixes)
		if excluded {
			res.FilesExcluded++
			return nil
		}

		_, isRemovingDest := removingDest[cleanPath]
		for dest := range removingDest {
			if dest != "" && strings.HasPrefix(cleanPath, dest+string(os.PathSeparator)) {
				isRemovingDest = true
				break
			}
		}

		fi, err := d.Info()
		if err != nil {
			res.FilesSkipped++
			return nil
		}

		nlink, ok := fileNLink(fi)
		if !ok {
			res.FilesSkipped++
			return nil
		}

		if strings.HasPrefix(zone, "library") && isRemovingDest {
			res.FilesExcluded++
			return nil
		}

		if nlink >= 2 {
			res.FilesOK++
			return nil
		}

		reason := "expected at least 2 hardlinks (download + library)"
		if zone == "downloads" {
			reason = "expected at least 2 hardlinks (still only in downloads folder)"
		}

		res.IssueCount++
		if len(res.Issues) < maxLinkIssuesSample {
			res.Issues = append(res.Issues, LinkIssue{
				Path: cleanPath, Zone: zone, NLink: nlink, Reason: reason,
			})
		}
		return nil
	})

	return res
}

func PathHasPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if path == p || strings.HasPrefix(path, p+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func FSResultToCheck(id, name string, r FSAuditResult) Check {
	if r.FilesSkipped == -1 {
		return Check{
			ID: id, Name: name, Status: CheckSkip,
			Message: fmt.Sprintf("path not found: %s", r.Root),
			Details: map[string]any{"root": r.Root},
		}
	}

	status := CheckOK
	msg := fmt.Sprintf("%d files scanned, all linked", r.FilesScanned)
	if r.IssueCount > 0 {
		status = CheckFail
		msg = fmt.Sprintf("%d files missing expected hardlink (sample up to %d)", r.IssueCount, maxLinkIssuesSample)
	}
	if r.Truncated {
		if status == CheckOK {
			status = CheckWarn
		}
		msg += "; scan truncated at file limit"
	}

	details := map[string]any{
		"root":           r.Root,
		"zone":           r.Zone,
		"files_scanned":  r.FilesScanned,
		"files_ok":       r.FilesOK,
		"files_excluded": r.FilesExcluded,
		"files_skipped":  r.FilesSkipped,
		"issue_count":    r.IssueCount,
		"issues_sample":  r.Issues,
		"truncated":      r.Truncated,
	}
	return Check{ID: id, Name: name, Status: status, Message: msg, Details: details}
}
