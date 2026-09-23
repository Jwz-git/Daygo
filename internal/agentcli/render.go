package agentcli

import (
	"fmt"
	"io"

	"github.com/Jwz-git/Daygo/internal/agentread"
)

// renderText prints a concise human summary. It is a convenience face; the
// stable, contract-checked output is --json (docs/05 §5.9.1), so these lines
// are not part of the diff gate.
func renderText(w io.Writer, result any) {
	switch r := result.(type) {
	case agentread.StatusResult:
		fmt.Fprintf(w, "database: %s\n", r.DatabasePath)
		fmt.Fprintf(w, "schema_version: %d\n", r.SchemaVersion)
		fmt.Fprintf(w, "db_user_version: %d\n", r.DBUserVersion)
		fmt.Fprintf(w, "generated_at: %s\n", r.GeneratedAt)
	case agentread.TimelineResult:
		fmt.Fprintf(w, "%s  tracked %.0fm  idle %.0fm  (%d cards)\n",
			r.Day, r.TrackedMinutes, r.IdleMinutes, len(r.Cards))
		for _, c := range r.Cards {
			fmt.Fprintf(w, "  %d  %s–%s  [%s] %s  (%.0fm)\n",
				c.ID, c.Start, c.End, c.Category, c.Title, c.DurationMinutes)
		}
	case agentread.CardResult:
		fmt.Fprintf(w, "card %d  %s\n", r.ID, r.Day)
		fmt.Fprintf(w, "  %s–%s  [%s] %s  (%.0fm)\n",
			r.Start, r.End, r.Category, r.Title, r.DurationMinutes)
		if r.Summary != "" {
			fmt.Fprintf(w, "  %s\n", r.Summary)
		}
	case agentread.DailyResult:
		fmt.Fprintf(w, "%s  journal:%s\n", r.Day, r.Journal.Status)
		if r.Goal.Exists {
			fmt.Fprintf(w, "  goal: focus %dm / distraction %dm  skipped=%t\n",
				r.Goal.FocusTargetMinutes, r.Goal.DistractionLimitMinutes, r.Goal.IsSkipped)
		} else {
			fmt.Fprintf(w, "  goal: none\n")
		}
	case agentread.WeeklyResult:
		fmt.Fprintf(w, "week %s  tracked %.0fm  focus %.0fm\n",
			r.WeekStart, r.TrackedMinutes, r.FocusMinutes)
		for _, c := range r.Categories {
			fmt.Fprintf(w, "  %-16s %.0fm  %.0f%%\n", c.Name, c.Minutes, c.Share*100)
		}
	case agentread.CategoriesResult:
		for _, c := range r.Categories {
			fmt.Fprintf(w, "  %-16s system=%t idle=%t\n", c.Name, c.IsSystem, c.IsIdle)
		}
	default:
		fmt.Fprintf(w, "%v\n", r)
	}
}
