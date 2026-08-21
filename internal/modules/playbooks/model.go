// Package playbooks manages workspace-configurable evaluation scripts and criteria.
package playbooks

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Criterion defines one rubric rule within a Playbook.
type Criterion struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MaxScore    int    `json:"max_score"`
	IsCritical  bool   `json:"is_critical,omitempty"`
	Penalty     int    `json:"penalty,omitempty"`
}

// Playbook defines a complete structured scoring rubric owned by a workspace.
type Playbook struct {
	ID          uuid.UUID   `json:"id"`
	WorkspaceID uuid.UUID   `json:"workspace_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	IsDefault   bool        `json:"is_default"`
	Criteria    []Criterion `json:"criteria"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Validate ensures a playbook has a valid name and valid criteria.
func (p *Playbook) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("playbook name is required")
	}
	if len(p.Criteria) == 0 {
		return fmt.Errorf("playbook must contain at least one criterion")
	}
	keys := make(map[string]bool, len(p.Criteria))
	totalMax := 0
	for _, c := range p.Criteria {
		key := strings.TrimSpace(c.Key)
		if key == "" {
			return fmt.Errorf("criterion key cannot be blank")
		}
		if keys[key] {
			return fmt.Errorf("duplicate criterion key %q", key)
		}
		keys[key] = true
		if strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("criterion %q must have a title", key)
		}
		if c.MaxScore <= 0 {
			return fmt.Errorf("criterion %q max_score must be positive", key)
		}
		totalMax += c.MaxScore
	}
	if totalMax == 0 {
		return fmt.Errorf("total playbook score must be greater than zero")
	}
	return nil
}

// DefaultCriteria returns the standard 8-criterion 100-point sales rubric.
func DefaultCriteria() []Criterion {
	return []Criterion{
		{Key: "greeting", Title: "Корпоративное приветствие", Description: "Менеджер должен четко поздороваться, назвать свое имя и компанию.", MaxScore: 5},
		{Key: "rapport", Title: "Установление контакта", Description: "Вежливый и доброжелательный тон, обращение к клиенту по имени.", MaxScore: 10},
		{Key: "needs_discovery", Title: "Выявление потребностей", Description: "Задать минимум 2-3 открытых вопроса для понимания задачи и бюджета клиента.", MaxScore: 20},
		{Key: "presentation", Title: "Презентация решения", Description: "Презентовать продукт через выгоды для клиента, а не просто перечислять функции.", MaxScore: 15},
		{Key: "objection_handling", Title: "Отработка возражений", Description: "Выслушать сомнения клиента, согласиться с важностью вопроса и аргументированно отработать.", MaxScore: 20},
		{Key: "next_action", Title: "Фиксация следующего шага", Description: "Назначить конкретную дату, время и цель следующего контакта.", MaxScore: 15},
		{Key: "communication", Title: "Грамотность и этика речи", Description: "Отсутствие слов-паразитов, перебиваний и пассивной агрессии.", MaxScore: 10},
		{Key: "closing", Title: "Завершение разговора", Description: "Позитивное прощание и подтверждение договоренностей.", MaxScore: 5},
	}
}
