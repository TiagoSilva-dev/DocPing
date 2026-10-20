package domain

import (
	"testing"
	"time"
)

func TestStateMachine(t *testing.T) {
	tests := []struct {
		name  string
		items []RequestItem
		want  string
	}{{"empty", []RequestItem{{TemplateItem: TemplateItem{Required: true}, Status: "pending"}}, "pending"}, {"partial review", []RequestItem{{TemplateItem: TemplateItem{Required: true}, Status: "uploaded"}, {TemplateItem: TemplateItem{Required: true}, Status: "pending"}}, "under_review"}, {"correction", []RequestItem{{TemplateItem: TemplateItem{Required: true}, Status: "correction_required"}}, "correction_required"}, {"optional missing", []RequestItem{{TemplateItem: TemplateItem{Required: true}, Status: "approved"}, {Status: "pending"}}, "completed"}, {"optional review", []RequestItem{{TemplateItem: TemplateItem{Required: true}, Status: "approved"}, {Status: "uploaded"}}, "under_review"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Request{Items: tc.items}
			Recalculate(&r)
			if r.Status != tc.want {
				t.Fatalf("%s != %s", r.Status, tc.want)
			}
		})
	}
}
func TestMonthDayAndNextExecution(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	if got := MonthDay(2028, time.February, 31, loc); got.Day() != 29 {
		t.Fatal(got)
	}
	if got := NextExecution(Schedule{DayOfMonth: 31}, time.Date(2026, 12, 31, 12, 0, 0, 0, loc)); got.Year() != 2027 || got.Month() != time.January || got.Day() != 31 {
		t.Fatal(got)
	}
}
