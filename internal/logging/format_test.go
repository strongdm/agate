package logging

import (
	"strings"
	"testing"
)

func TestFormatInterview_WithOptions(t *testing.T) {
	questions := []InterviewQuestion{
		{
			Title:    "Authentication Method",
			Question: "What auth method should be used?",
			Options:  []string{"JWT tokens", "Session cookies"},
		},
	}

	result := FormatInterview(questions)

	if !strings.Contains(result, "### Q1: Authentication Method") {
		t.Error("expected question header")
	}
	if !strings.Contains(result, "- [ ] JWT tokens") {
		t.Error("expected JWT option checkbox")
	}
	if !strings.Contains(result, "- [ ] Session cookies") {
		t.Error("expected Session cookies option checkbox")
	}
	if !strings.Contains(result, "- [ ] No preference") {
		t.Error("expected No preference option")
	}
	if !strings.Contains(result, "> Notes:") {
		t.Error("expected Notes blockquote for question with options")
	}
	if strings.Contains(result, "> Answer:") {
		t.Error("should not contain Answer blockquote for question with options")
	}
	if !strings.Contains(result, "---") {
		t.Error("expected separator")
	}
	if strings.Contains(result, "**Answer**") {
		t.Error("should not contain legacy Answer marker")
	}
}

func TestFormatInterview_NoOptions(t *testing.T) {
	questions := []InterviewQuestion{
		{
			Title:    "Deployment Target",
			Question: "Where will this be deployed?",
		},
	}

	result := FormatInterview(questions)

	if !strings.Contains(result, "> Answer:") {
		t.Error("expected Answer blockquote for no-option question")
	}
	if strings.Contains(result, "> Notes:") {
		t.Error("should not contain Notes blockquote for no-option question")
	}
	if strings.Contains(result, "- [ ] No preference") {
		t.Error("should not contain option checkboxes for no-option question")
	}
}

func TestParseInterviewAnswers_CheckboxOnly(t *testing.T) {
	content := `### Q1: Authentication Method

What auth method should be used?

- [x] JWT tokens
- [ ] Session cookies
- [ ] No preference

> Notes:

---
`
	answers := ParseInterviewAnswers(content)

	got, ok := answers["Authentication Method"]
	if !ok {
		t.Fatal("expected answer for Authentication Method")
	}
	if got != "JWT tokens" {
		t.Errorf("expected 'JWT tokens', got %q", got)
	}
}

func TestParseInterviewAnswers_MultipleCheckboxes(t *testing.T) {
	content := `### Q1: Features

Which features?

- [x] Auth
- [x] Logging
- [ ] Caching
- [ ] No preference

> Notes:

---
`
	answers := ParseInterviewAnswers(content)

	got := answers["Features"]
	if got != "Auth, Logging" {
		t.Errorf("expected 'Auth, Logging', got %q", got)
	}
}

func TestParseInterviewAnswers_NotesOnly(t *testing.T) {
	content := `### Q1: Authentication Method

What auth method?

- [ ] JWT tokens
- [ ] Session cookies
- [ ] No preference

> Notes: Use OAuth2 with PKCE flow

---
`
	answers := ParseInterviewAnswers(content)

	got := answers["Authentication Method"]
	if got != "Use OAuth2 with PKCE flow" {
		t.Errorf("expected 'Use OAuth2 with PKCE flow', got %q", got)
	}
}

func TestParseInterviewAnswers_CheckboxAndNotes(t *testing.T) {
	content := `### Q1: Authentication Method

What auth method?

- [x] JWT tokens
- [ ] Session cookies
- [ ] No preference

> Notes: With refresh token rotation

---
`
	answers := ParseInterviewAnswers(content)

	got := answers["Authentication Method"]
	if got != "JWT tokens\nWith refresh token rotation" {
		t.Errorf("expected 'JWT tokens\\nWith refresh token rotation', got %q", got)
	}
}

func TestParseInterviewAnswers_NoOptionQuestion(t *testing.T) {
	content := `### Q1: Deployment Target

Where will this be deployed?

> Answer: AWS ECS with Fargate

---
`
	answers := ParseInterviewAnswers(content)

	got := answers["Deployment Target"]
	if got != "AWS ECS with Fargate" {
		t.Errorf("expected 'AWS ECS with Fargate', got %q", got)
	}
}

func TestParseInterviewAnswers_NoPreferenceIgnored(t *testing.T) {
	content := `### Q1: Database

Which database?

- [ ] PostgreSQL
- [ ] MySQL
- [x] No preference

> Notes:

---
`
	answers := ParseInterviewAnswers(content)

	got, ok := answers["Database"]
	if ok && got != "" {
		t.Errorf("expected empty/missing answer when only No preference checked, got %q", got)
	}
}

func TestParseInterviewAnswers_LegacyFormat(t *testing.T) {
	content := `### Q1: Deployment Target

Where will this be deployed?

**Answer**: AWS Lambda

---
`
	answers := ParseInterviewAnswers(content)

	got := answers["Deployment Target"]
	if got != "AWS Lambda" {
		t.Errorf("expected 'AWS Lambda', got %q", got)
	}
}

func TestParseInterviewAnswers_MultipleQuestions(t *testing.T) {
	content := `### Q1: Auth

What auth?

- [x] JWT tokens
- [ ] No preference

> Notes:

---

### Q2: Deploy

Where to deploy?

> Answer: Kubernetes

---
`
	answers := ParseInterviewAnswers(content)

	if answers["Auth"] != "JWT tokens" {
		t.Errorf("expected 'JWT tokens' for Auth, got %q", answers["Auth"])
	}
	if answers["Deploy"] != "Kubernetes" {
		t.Errorf("expected 'Kubernetes' for Deploy, got %q", answers["Deploy"])
	}
}
