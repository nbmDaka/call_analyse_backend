package dashboard

import (
	"strings"
	"testing"

	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/modules/workspaces"
	"github.com/google/uuid"
)

func TestSummaryQueryIsWorkspaceScopedBeforeAggregation(t *testing.T) {
	actor := calls.Actor{UserID: uuid.New(), WorkspaceID: uuid.New(), MembershipID: uuid.New(), WorkspaceRole: workspaces.RoleAdmin}
	query, args, err := summaryQuery(actor)
	if err != nil {
		t.Fatalf("summaryQuery() error = %v", err)
	}
	if !strings.Contains(query, "WHERE c.workspace_id = $1") || len(args) != 1 || args[0] != actor.WorkspaceID {
		t.Fatalf("dashboard query is not tenant scoped: query=%s args=%#v", query, args)
	}
}

func TestSummaryQueryRejectsActorWithoutWorkspace(t *testing.T) {
	if _, _, err := summaryQuery(calls.Actor{UserID: uuid.New(), WorkspaceRole: workspaces.RoleAdmin}); err == nil {
		t.Fatal("summaryQuery() accepted actor without workspace")
	}
}
